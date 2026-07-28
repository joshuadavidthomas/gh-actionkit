package tools

import (
	"context"
	"io"

	"github.com/joshuadavidthomas/gh-actionkit/internal/workflow"
	"github.com/rhysd/actionlint"
)

const actionlintJSONLines = `{{range $err := .}}{{json $err}}{{end}}`

type ValidationResult struct {
	Files    int
	Findings int
}

type Actionlint struct{}

func (Actionlint) Validate(ctx context.Context, repository string, outputJSON bool, stdout, stderr io.Writer) (ValidationResult, error) {
	if err := ctx.Err(); err != nil {
		return ValidationResult{}, err
	}

	files, err := workflow.FindFiles(repository)
	if err != nil || len(files) == 0 {
		return ValidationResult{}, err
	}

	project, err := actionlint.NewProject(repository)
	if err != nil {
		return ValidationResult{}, err
	}
	format := ""
	if outputJSON {
		format = actionlintJSONLines
	}
	linter, err := actionlint.NewLinter(stdout, &actionlint.LinterOptions{
		WorkingDir: repository,
		LogWriter:  stderr,
		Format:     format,
		Shellcheck: "shellcheck",
		Pyflakes:   "pyflakes",
	})
	if err != nil {
		return ValidationResult{}, err
	}
	type lintResult struct {
		findings []*actionlint.Error
		err      error
	}
	result := make(chan lintResult, 1)
	go func() {
		findings, err := linter.LintFiles(files, project)
		result <- lintResult{findings: findings, err: err}
	}()

	select {
	case <-ctx.Done():
		// actionlint has no cancellation API, so the in-flight lint must finish in the background.
		return ValidationResult{}, ctx.Err()
	case lint := <-result:
		if lint.err != nil {
			return ValidationResult{}, lint.err
		}
		return ValidationResult{Files: len(files), Findings: len(lint.findings)}, nil
	}
}
