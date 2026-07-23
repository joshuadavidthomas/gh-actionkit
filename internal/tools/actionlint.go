package tools

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rhysd/actionlint"
)

const actionlintJSONLines = `{{range $err := .}}{{json $err}}{{end}}`

type ValidationResult struct {
	Files    int
	Findings int
}

type Actionlint struct{}

func (Actionlint) Validate(repository string, outputJSON bool, stdout, stderr io.Writer) (ValidationResult, error) {
	files, err := workflowFiles(repository)
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

func workflowFiles(repository string) ([]string, error) {
	directory := filepath.Join(repository, ".github", "workflows")
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") {
			files = append(files, filepath.Join(directory, name))
		}
	}
	sort.Strings(files)
	return files, nil
}
