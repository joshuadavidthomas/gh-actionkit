package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
	"github.com/spf13/cobra"
)

type VersionLookup func(context.Context, string) (actions.VersionInfo, error)
type ActionSearch func(context.Context, string, int, bool) ([]actions.SearchResult, error)

type Dependencies struct {
	Version           string
	LookupVersion     VersionLookup
	SearchActions     ActionSearch
	LintWorkflows     WorkflowLint
	ValidateWorkflows WorkflowValidate
	CheckActions      ActionCheck
}

func NewRootCommand(stdout, stderr io.Writer, dependencies Dependencies) *cobra.Command {
	command := &cobra.Command{
		Use:           "actionkit",
		Short:         "Find, check, and validate GitHub Actions",
		Version:       dependencies.Version,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	command.SetOut(stdout)
	command.SetErr(stderr)
	command.AddCommand(
		newVersionCommand(dependencies.LookupVersion),
		newSearchCommand(dependencies.SearchActions),
		newLintCommand(dependencies.LintWorkflows),
		newValidateCommand(dependencies.ValidateWorkflows),
		newCheckCommand(dependencies.CheckActions),
	)
	return command
}

func newVersionCommand(lookup VersionLookup) *cobra.Command {
	var outputJSON bool
	command := &cobra.Command{
		Use:   "version OWNER/REPO",
		Short: "Show the latest stable version of a GitHub Action",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			info, err := lookup(command.Context(), args[0])
			if err != nil {
				return err
			}
			if outputJSON {
				encoder := json.NewEncoder(command.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(info)
			}
			return writeVersion(command.OutOrStdout(), info)
		},
	}
	command.Flags().BoolVar(&outputJSON, "json", false, "output JSON")
	return command
}

func newSearchCommand(search ActionSearch) *cobra.Command {
	var limit int
	var outputJSON bool
	var fast bool
	command := &cobra.Command{
		Use:   "search QUERY",
		Short: "Search for GitHub Actions",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			results, err := search(command.Context(), args[0], limit, fast)
			if err != nil {
				return err
			}
			if outputJSON {
				encoder := json.NewEncoder(command.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(results)
			}
			output := command.OutOrStdout()
			if len(results) == 0 {
				_, err := fmt.Fprintf(output, "No actions found for %q\n", args[0])
				return err
			}
			for _, result := range results {
				if _, err := fmt.Fprintf(output, "%s (★ %s)\n", result.Action, formatStars(result.Stars)); err != nil {
					return err
				}
				if result.Description != nil && *result.Description != "" {
					if _, err := fmt.Fprintf(output, "  %s\n", *result.Description); err != nil {
						return err
					}
				}
				if _, err := fmt.Fprintln(output); err != nil {
					return err
				}
			}
			return nil
		},
	}
	command.Flags().IntVarP(&limit, "limit", "n", 10, "number of results to return (1-100)")
	command.Flags().BoolVar(&outputJSON, "json", false, "output JSON")
	command.Flags().BoolVar(&fast, "fast", false, "skip action manifest verification")
	return command
}

func formatStars(stars int) string {
	if stars >= 1000 {
		return fmt.Sprintf("%.1fk", float64(stars)/1000)
	}
	return fmt.Sprint(stars)
}

func writeVersion(output io.Writer, info actions.VersionInfo) error {
	if _, err := fmt.Fprintln(output, info.Action); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(output, "  major"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "    tag: %s\n", info.Major.Tag); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "    sha: %s\n", formatSHA(info.Major.SHA)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(output, "  latest"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "    tag: %s\n", info.Latest.Tag); err != nil {
		return err
	}
	_, err := fmt.Fprintf(output, "    sha: %s\n", formatSHA(info.Latest.SHA))
	return err
}

func formatSHA(sha *string) string {
	if sha == nil {
		return "unknown"
	}
	return *sha
}
