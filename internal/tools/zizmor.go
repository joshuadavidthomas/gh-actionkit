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

type Command struct {
	Path     string
	Args     []string
	Dir      string
	Env      []string
	UnsetEnv []string
	Stdout   io.Writer
	Stderr   io.Writer
}

type Runner interface {
	Run(context.Context, Command) error
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, command Command) error {
	process := exec.CommandContext(ctx, command.Path, command.Args...)
	process.Dir = command.Dir
	if len(command.Env) > 0 || len(command.UnsetEnv) > 0 {
		process.Env = commandEnvironment(process.Environ(), command.UnsetEnv, command.Env)
	}
	process.Stdout = command.Stdout
	process.Stderr = command.Stderr
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

type Zizmor struct {
	runner   Runner
	lookPath func(string) (string, error)
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

func NewZizmor() Zizmor {
	return Zizmor{runner: ExecRunner{}, lookPath: safeexec.LookPath}
}

func (z Zizmor) Lint(
	ctx context.Context,
	repository string,
	options ZizmorOptions,
	stdout io.Writer,
	stderr io.Writer,
) (int, error) {
	path, err := z.lookPath("zizmor")
	useUV := false
	if err != nil {
		if !errors.Is(err, exec.ErrNotFound) {
			return 0, fmt.Errorf("find zizmor executable: %w", err)
		}
		path, err = z.lookPath("uv")
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

	err = z.runner.Run(ctx, Command{
		Path:     path,
		Args:     arguments,
		Dir:      repository,
		Env:      environment,
		UnsetEnv: zizmorEnvironment,
		Stdout:   stdout,
		Stderr:   stderr,
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
