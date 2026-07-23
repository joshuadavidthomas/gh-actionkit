package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
	"github.com/joshuadavidthomas/gh-actionkit/internal/cli"
	"github.com/joshuadavidthomas/gh-actionkit/internal/githubapi"
	"github.com/joshuadavidthomas/gh-actionkit/internal/tools"
	"github.com/joshuadavidthomas/gh-actionkit/internal/workflow"
)

func main() {
	zizmor := tools.NewZizmor()
	actionlint := tools.Actionlint{}
	dependencies := cli.Dependencies{
		LookupVersion: func(ctx context.Context, action string) (actions.VersionInfo, error) {
			client, err := githubapi.New()
			if err != nil {
				return actions.VersionInfo{}, fmt.Errorf("connect to GitHub: %w", err)
			}
			return actions.NewVersionService(client).Lookup(ctx, action)
		},
		SearchActions: func(ctx context.Context, query string, limit int, fast bool) ([]actions.SearchResult, error) {
			client, err := githubapi.New()
			if err != nil {
				return nil, fmt.Errorf("connect to GitHub: %w", err)
			}
			return actions.NewSearchService(client).Search(ctx, query, limit, fast)
		},
		LintWorkflows: zizmor.Lint,
		ValidateWorkflows: func(repository string, outputJSON bool, stdout, stderr io.Writer) (int, int, error) {
			result, err := actionlint.Validate(repository, outputJSON, stdout, stderr)
			return result.Files, result.Findings, err
		},
		CheckActions: func(ctx context.Context, repository string) (actions.CheckReport, error) {
			scan, err := workflow.ScanRepository(repository)
			if err != nil {
				return actions.CheckReport{}, err
			}
			report := actions.CheckReport{WorkflowFiles: scan.Files, Uses: len(scan.Uses), Results: []actions.CheckResult{}}
			if len(scan.Uses) == 0 {
				return report, nil
			}
			client, err := githubapi.New()
			if err != nil {
				return actions.CheckReport{}, fmt.Errorf("connect to GitHub: %w", err)
			}
			report.Results, err = actions.NewCheckService(client).Check(ctx, scan.Uses)
			return report, err
		},
	}

	if err := cli.NewRootCommand(os.Stdout, os.Stderr, dependencies).Execute(); err != nil {
		exitCode := 2
		var statusError interface{ ExitCode() int }
		if errors.As(err, &statusError) {
			exitCode = statusError.ExitCode()
		}
		if err.Error() != "" {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(exitCode)
	}
}
