package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
	"github.com/joshuadavidthomas/gh-actionkit/internal/githubapi"
	"github.com/joshuadavidthomas/gh-actionkit/internal/workflow"
	"github.com/spf13/cobra"
)

type actionCheck func(context.Context, string) (actions.CheckReport, error)

func newCheckCommand() *cobra.Command {
	return newCheckCommandWithCheck(checkActions)
}

func checkActions(ctx context.Context, repository string) (actions.CheckReport, error) {
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
}

func newCheckCommandWithCheck(check actionCheck) *cobra.Command {
	var repository string
	var outputJSON bool

	command := &cobra.Command{
		Use:   "check",
		Short: "Check workflow action refs for newer versions",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			repositoryPath, err := resolveRepository(repository)
			if err != nil {
				return err
			}
			report, err := check(command.Context(), repositoryPath)
			if err != nil {
				return err
			}
			if outputJSON {
				encoder := json.NewEncoder(command.OutOrStdout())
				encoder.SetIndent("", "  ")
				if err := encoder.Encode(report.Results); err != nil {
					return err
				}
			} else {
				switch {
				case report.WorkflowFiles == 0:
					_, err = fmt.Fprintln(command.OutOrStdout(), "No workflow files found in .github/workflows")
				case report.Uses == 0:
					_, err = fmt.Fprintln(command.OutOrStdout(), "No remote action uses found in workflow files")
				default:
					err = writeCheckResults(command.OutOrStdout(), report.Results)
				}
				if err != nil {
					return err
				}
			}
			for _, result := range report.Results {
				if result.UpdateAvailable {
					return StatusError{Code: 1}
				}
			}
			return nil
		},
	}
	command.Flags().StringVarP(&repository, "repo", "C", ".", "repository path to inspect")
	command.Flags().BoolVar(&outputJSON, "json", false, "output JSON")
	return command
}

func writeCheckResults(output io.Writer, results []actions.CheckResult) error {
	writer := tabwriter.NewWriter(output, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(writer, "ACTION\tSTATUS\tWORKFLOWS\tUSED\tMAJOR\tLATEST"); err != nil {
		return err
	}
	for _, result := range results {
		status := "unknown"
		if result.UpToDate {
			status = "up to date"
		} else if result.UpdateAvailable {
			status = "update available"
		}
		locations := make([]string, 0, len(result.Locations))
		for _, location := range result.Locations {
			locations = append(locations, fmt.Sprintf("%s:%d", location.File, location.Line))
		}
		if _, err := fmt.Fprintf(
			writer,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			result.Action,
			status,
			strings.Join(locations, ", "),
			formatCheckVersion(result.Used),
			formatCheckVersion(result.Major),
			formatCheckVersion(result.Latest),
		); err != nil {
			return err
		}
	}
	return writer.Flush()
}

func formatCheckVersion(version actions.CheckVersion) string {
	switch {
	case version.Tag != nil && version.SHA != nil:
		return *version.Tag + "@" + *version.SHA
	case version.Tag != nil:
		return *version.Tag + "@unknown"
	case version.SHA != nil:
		return *version.SHA
	default:
		return "unknown"
	}
}
