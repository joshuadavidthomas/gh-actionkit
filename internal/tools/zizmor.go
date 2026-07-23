package tools

import (
	"context"
	"fmt"
	"io"
	"os/exec"

	"github.com/cli/safeexec"
)

type Command struct {
	Path   string
	Args   []string
	Dir    string
	Stdout io.Writer
	Stderr io.Writer
}

type Runner interface {
	Run(context.Context, Command) error
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, command Command) error {
	process := exec.CommandContext(ctx, command.Path, command.Args...)
	process.Dir = command.Dir
	process.Stdout = command.Stdout
	process.Stderr = command.Stderr
	return process.Run()
}

type Zizmor struct {
	runner   Runner
	lookPath func(string) (string, error)
}

func NewZizmor() Zizmor {
	return Zizmor{runner: ExecRunner{}, lookPath: safeexec.LookPath}
}

func (z Zizmor) Lint(
	ctx context.Context,
	repository string,
	outputJSON bool,
	pedantic bool,
	stdout io.Writer,
	stderr io.Writer,
) (int, error) {
	path, err := z.lookPath("zizmor")
	if err != nil {
		return 0, fmt.Errorf("zizmor not found: install it from https://docs.zizmor.sh/installation: %w", err)
	}

	arguments := []string{"--collect=workflows", "--no-progress"}
	if outputJSON {
		arguments = append(arguments, "--format=json-v1")
	}
	if pedantic {
		arguments = append(arguments, "--pedantic")
	}
	arguments = append(arguments, ".")

	err = z.runner.Run(ctx, Command{
		Path:   path,
		Args:   arguments,
		Dir:    repository,
		Stdout: stdout,
		Stderr: stderr,
	})
	if err == nil {
		return 0, nil
	}
	if exitError, ok := err.(interface{ ExitCode() int }); ok && exitError.ExitCode() >= 0 {
		return exitError.ExitCode(), nil
	}
	return 0, fmt.Errorf("run zizmor: %w", err)
}
