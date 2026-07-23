package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
	"github.com/joshuadavidthomas/gh-actionkit/internal/githubapi"
	"github.com/spf13/cobra"
)

type actionSearch func(context.Context, string, int, bool) ([]actions.SearchResult, error)

func newSearchCommand() *cobra.Command {
	return newSearchCommandWithSearch(searchActions)
}

func searchActions(ctx context.Context, query string, limit int, fast bool) ([]actions.SearchResult, error) {
	client, err := githubapi.New()
	if err != nil {
		return nil, fmt.Errorf("connect to GitHub: %w", err)
	}
	return actions.NewSearchService(client).Search(ctx, query, limit, fast)
}

func newSearchCommandWithSearch(search actionSearch) *cobra.Command {
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
