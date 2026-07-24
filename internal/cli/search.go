package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
	"github.com/joshuadavidthomas/gh-actionkit/internal/githubapi"
	"github.com/spf13/cobra"
)

type actionSearch func(context.Context, string, int) ([]actions.SearchResult, error)

func newSearchCommand() *cobra.Command {
	return newSearchCommandWithSearch(searchActions)
}

func searchActions(ctx context.Context, query string, limit int) ([]actions.SearchResult, error) {
	client, err := githubapi.New()
	if err != nil {
		return nil, fmt.Errorf("connect to GitHub: %w", err)
	}
	return actions.NewSearchService(client).Search(ctx, query, limit)
}

func newSearchCommandWithSearch(search actionSearch) *cobra.Command {
	var limit int
	var outputJSON bool
	command := &cobra.Command{
		Use:   "search QUERY",
		Short: "Search for GitHub Actions",
		Long:  "Search GitHub repositories and verify that each result contains a root Action manifest.",
		Example: "  gh actionkit search checkout\n" +
			"  gh actionkit search \"docker build\" --limit 5",
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			indicator := startCommandSpinner(
				command.OutOrStdout(),
				command.ErrOrStderr(),
				outputJSON,
				"Searching GitHub and verifying actions...",
			)
			defer indicator.Stop()
			results, err := search(command.Context(), args[0], limit)
			indicator.Stop()
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
			return writeSearchResults(output, results)
		},
	}
	command.Flags().IntVarP(&limit, "limit", "n", 10, "number of results to return (1-100)")
	command.Flags().BoolVar(&outputJSON, "json", false, "output JSON")
	return command
}

func writeSearchResults(output io.Writer, results []actions.SearchResult) error {
	styles := newOutputStyles(newOutputRenderer(output))
	actionStyle := styles.action.Bold(true)

	for _, result := range results {
		details := styles.secondary.Render(fmt.Sprintf("(⭐ %s)", formatStars(result.Stars)))
		if _, err := fmt.Fprintf(output, "%s %s\n", actionStyle.Render(result.Action), details); err != nil {
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
}

func formatStars(stars int) string {
	if stars >= 1000 {
		return fmt.Sprintf("%.1fk", float64(stars)/1000)
	}
	return fmt.Sprint(stars)
}
