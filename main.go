package main

import (
	"context"
	"fmt"
	"os"

	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
	"github.com/joshuadavidthomas/gh-actionkit/internal/cli"
	"github.com/joshuadavidthomas/gh-actionkit/internal/githubapi"
)

func main() {
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
	}

	if err := cli.NewRootCommand(os.Stdout, os.Stderr, dependencies).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
