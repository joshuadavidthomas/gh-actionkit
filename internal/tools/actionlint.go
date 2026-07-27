package tools

import (
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

func (Actionlint) Validate(repository string, outputJSON bool, stdout, stderr io.Writer) (ValidationResult, error) {
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
	findings, err := linter.LintFiles(files, project)
	if err != nil {
		return ValidationResult{}, err
	}
	return ValidationResult{Files: len(files), Findings: len(findings)}, nil
}
