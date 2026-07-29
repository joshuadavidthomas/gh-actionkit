package tools

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/cli/safeexec"
)

type command struct {
	path     string
	args     []string
	dir      string
	env      []string
	unsetEnv []string
	stdout   io.Writer
	stderr   io.Writer
}

func runCommand(ctx context.Context, command command) error {
	process := exec.CommandContext(ctx, command.path, command.args...)
	process.Dir = command.dir
	if len(command.env) > 0 || len(command.unsetEnv) > 0 {
		process.Env = commandEnvironment(process.Environ(), command.unsetEnv, command.env)
	}
	process.Stdout = command.stdout
	process.Stderr = command.stderr
	return process.Run()
}

func commandEnvironment(environment, unset, overlay []string) []string {
	removed := make(map[string]struct{}, len(unset))
	for _, name := range unset {
		removed[strings.ToUpper(name)] = struct{}{}
	}
	result := make([]string, 0, len(environment)+len(overlay))
	for _, value := range environment {
		name, _, _ := strings.Cut(value, "=")
		if _, found := removed[strings.ToUpper(name)]; !found {
			result = append(result, value)
		}
	}
	return append(result, overlay...)
}

type GitHubCredentials struct {
	Host  string
	Token string
}

type ZizmorOptions struct {
	OutputJSON bool
	Pedantic   bool
	GitHub     *GitHubCredentials
}

const uvPythonIndex = "https://pypi.org/simple"

//go:embed requirements.txt
var zizmorRequirement string

var zizmorEnvironment = []string{
	"GH_TOKEN",
	"GITHUB_TOKEN",
	"GH_ENTERPRISE_TOKEN",
	"GITHUB_ENTERPRISE_TOKEN",
	"ZIZMOR_GITHUB_TOKEN",
	"ZIZMOR_OFFLINE",
	"ZIZMOR_NO_ONLINE_AUDITS",
}

func Lint(
	ctx context.Context,
	repository string,
	options ZizmorOptions,
	stdout io.Writer,
	stderr io.Writer,
) (int, error) {
	return lint(ctx, repository, options, stdout, stderr, runCommand, safeexec.LookPath)
}

func lint(
	ctx context.Context,
	repository string,
	options ZizmorOptions,
	stdout io.Writer,
	stderr io.Writer,
	run func(context.Context, command) error,
	lookPath func(string) (string, error),
) (int, error) {
	path, err := lookPath("zizmor")
	useUV := false
	if err != nil {
		if !errors.Is(err, exec.ErrNotFound) {
			return 0, fmt.Errorf("find zizmor executable: %w", err)
		}
		path, err = lookPath("uv")
		if err != nil {
			return 0, fmt.Errorf(
				"zizmor not found and uv fallback unavailable: install zizmor from https://docs.zizmor.sh/installation: %w",
				err,
			)
		}
		useUV = true
	}

	arguments := []string{"--collect=workflows", "--no-progress"}
	if options.OutputJSON {
		arguments = append(arguments, "--format=json-v1")
	}
	if options.Pedantic {
		arguments = append(arguments, "--pedantic")
	}
	environment := []string(nil)
	if options.GitHub == nil {
		arguments = append(arguments, "--offline")
	} else {
		if options.GitHub.Host == "" || options.GitHub.Token == "" {
			return 0, errors.New("configure zizmor GitHub access: host and token are required")
		}
		environment = []string{
			"GH_HOST=" + options.GitHub.Host,
			"GH_TOKEN=" + options.GitHub.Token,
		}
	}
	arguments = append(arguments, ".")
	if useUV {
		requirement := strings.TrimSpace(zizmorRequirement)
		if requirement == "" {
			return 0, errors.New("embedded zizmor requirement is empty")
		}
		arguments = append([]string{
			"tool", "run",
			"--no-config",
			"--no-progress",
			"--default-index", uvPythonIndex,
			"--from", requirement,
			"zizmor",
		}, arguments...)
	}

	err = run(ctx, command{
		path:     path,
		args:     arguments,
		dir:      repository,
		env:      environment,
		unsetEnv: zizmorEnvironment,
		stdout:   stdout,
		stderr:   stderr,
	})
	if err == nil {
		return 0, nil
	}
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	if exitError, ok := err.(interface{ ExitCode() int }); ok && exitError.ExitCode() >= 0 {
		return exitError.ExitCode(), nil
	}
	return 0, fmt.Errorf("run zizmor: %w", err)
}
