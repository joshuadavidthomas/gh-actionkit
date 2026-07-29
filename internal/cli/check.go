package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
	"github.com/joshuadavidthomas/gh-actionkit/internal/githubapi"
	"github.com/joshuadavidthomas/gh-actionkit/internal/workflow"
	"github.com/spf13/cobra"
)

type checkReport struct {
	WorkflowFiles int
	Uses          int
	Results       []actions.CheckResult
}

type actionCheck func(context.Context, string) (checkReport, error)

func checkActions(ctx context.Context, repository string) (checkReport, error) {
	scan, err := workflow.ScanRepository(repository)
	if err != nil {
		return checkReport{}, err
	}
	report := checkReport{WorkflowFiles: scan.Files, Uses: len(scan.Uses), Results: []actions.CheckResult{}}
	if len(scan.Uses) == 0 {
		return report, nil
	}
	client, err := githubapi.New()
	if err != nil {
		return checkReport{}, fmt.Errorf("connect to GitHub: %w", err)
	}
	report.Results, err = actions.NewCheckService(client).Check(ctx, scan.Uses)
	return report, err
}

func newCheckCommand(check actionCheck) *cobra.Command {
	var repository string
	var outputJSON bool
	var requireSHA bool
	var failOnUnknown bool
	var allowedOwners []string

	command := &cobra.Command{
		Use:   "check",
		Short: "Check workflow action refs for newer versions",
		Long:  "Scan workflow files, resolve each remote Action ref, and compare it with the latest stable release or tag.",
		Example: "  gh actionkit check\n" +
			"  gh actionkit check --require-sha --fail-on-unknown\n" +
			"  gh actionkit check --allow-owner actions --allow-owner github\n" +
			"  gh actionkit check --repo ../another-repository --json",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			repositoryPath, err := resolveRepository(repository)
			if err != nil {
				return err
			}
			indicator := startCommandSpinner(
				command.OutOrStdout(),
				command.ErrOrStderr(),
				outputJSON,
				"Checking action versions...",
			)
			report, err := check(command.Context(), repositoryPath)
			indicator.Stop()
			if err != nil {
				return err
			}
			actions.ApplyCheckPolicy(report.Results, actions.CheckPolicy{
				RequireSHA:    requireSHA,
				FailOnUnknown: failOnUnknown,
				AllowedOwners: allowedOwners,
			})
			emptyMessage := checkEmptyMessage(report)
			if outputJSON {
				encoder := json.NewEncoder(command.OutOrStdout())
				encoder.SetIndent("", "  ")
				if err := encoder.Encode(report.Results); err != nil {
					return err
				}
				if emptyMessage != "" {
					if _, err := fmt.Fprintln(command.ErrOrStderr(), emptyMessage); err != nil {
						return err
					}
				}
			} else if emptyMessage != "" {
				if _, err := fmt.Fprintln(command.OutOrStdout(), emptyMessage); err != nil {
					return err
				}
			} else if err := writeCheckResults(command.OutOrStdout(), report.Results); err != nil {
				return err
			}
			for _, result := range report.Results {
				if result.Status == actions.CheckStatusUpdateAvailable || len(result.PolicyViolations) > 0 {
					return exitStatusError(1)
				}
			}
			return nil
		},
	}
	command.Flags().StringVarP(&repository, "repo", "C", ".", "repository path to inspect")
	command.Flags().BoolVar(&outputJSON, "json", false, "output JSON")
	command.Flags().BoolVar(&requireSHA, "require-sha", false, "fail when a remote Action is not pinned to a full commit SHA")
	command.Flags().BoolVar(&failOnUnknown, "fail-on-unknown", false, "fail when an Action ref cannot be classified")
	command.Flags().StringSliceVar(&allowedOwners, "allow-owner", nil, "allow remote Actions from this owner (repeatable)")
	return command
}

func checkEmptyMessage(report checkReport) string {
	switch {
	case report.WorkflowFiles == 0:
		return "No workflow files found in .github/workflows"
	case report.Uses == 0:
		return "No remote action uses found in workflow files"
	default:
		return ""
	}
}

func writeCheckResults(output io.Writer, results []actions.CheckResult) error {
	_, err := fmt.Fprintln(output, renderCheckResults(results, outputWidth(output), outputUsesColor(output)))
	return err
}
