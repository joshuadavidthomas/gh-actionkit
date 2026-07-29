package actions

import (
	"context"
	"fmt"
)

const searchCandidates = 100

type SearchResult struct {
	Action      string  `json:"action"`
	Description *string `json:"description"`
	Stars       int     `json:"stars"`
	URL         string  `json:"url"`
}

type SearchOptions struct {
	Query          string
	ResultLimit    int
	CandidateLimit int
}

type SearchSource interface {
	SearchRepositories(context.Context, SearchOptions) ([]SearchResult, error)
}

func Search(ctx context.Context, source SearchSource, query string, limit int) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}
	if limit < 1 || limit > searchCandidates {
		return nil, fmt.Errorf("limit must be between 1 and %d", searchCandidates)
	}

	results, err := source.SearchRepositories(ctx, SearchOptions{
		Query:          query,
		ResultLimit:    limit,
		CandidateLimit: searchCandidates,
	})
	if err != nil {
		return nil, fmt.Errorf("search repositories: %w", err)
	}
	return results, nil
}
