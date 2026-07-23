package actions

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

const searchCandidates = 100

var actionManifestNames = []string{"action.yml", "action.yaml"}

type SearchResult struct {
	Action      string `json:"action"`
	Description string `json:"description"`
	Stars       int    `json:"stars"`
	URL         string `json:"url"`
}

type SearchSource interface {
	SearchRepositories(context.Context, string, int) ([]SearchResult, error)
	HasFile(context.Context, Repository, string) (bool, error)
}

type SearchService struct {
	source  SearchSource
	workers int
}

func NewSearchService(source SearchSource) SearchService {
	return SearchService{source: source, workers: 10}
}

func (s SearchService) Search(ctx context.Context, query string, limit int, fast bool) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}
	if limit < 1 || limit > searchCandidates {
		return nil, fmt.Errorf("limit must be between 1 and %d", searchCandidates)
	}

	candidateLimit := searchCandidates
	if fast {
		candidateLimit = limit
	}
	candidates, err := s.source.SearchRepositories(ctx, query, candidateLimit)
	if err != nil {
		return nil, fmt.Errorf("search repositories: %w", err)
	}
	if fast || len(candidates) == 0 {
		return take(candidates, limit), nil
	}

	verified, err := s.verify(ctx, candidates)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(verified, func(i, j int) bool {
		return verified[i].Stars > verified[j].Stars
	})
	return take(verified, limit), nil
}

func (s SearchService) verify(ctx context.Context, candidates []SearchResult) ([]SearchResult, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan SearchResult)
	results := make(chan SearchResult, len(candidates))
	errorsFound := make(chan error, 1)
	var workers sync.WaitGroup

	workerCount := min(s.workers, len(candidates))
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for candidate := range jobs {
				verified, err := s.hasActionManifest(ctx, candidate.Action)
				if err != nil {
					select {
					case errorsFound <- err:
						cancel()
					default:
					}
					return
				}
				if verified {
					results <- candidate
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, candidate := range candidates {
			select {
			case jobs <- candidate:
			case <-ctx.Done():
				return
			}
		}
	}()

	workers.Wait()
	close(results)
	select {
	case err := <-errorsFound:
		return nil, err
	default:
	}

	verified := make([]SearchResult, 0, len(results))
	for result := range results {
		verified = append(verified, result)
	}
	return verified, nil
}

func (s SearchService) hasActionManifest(ctx context.Context, action string) (bool, error) {
	repository, err := parseRepository(action)
	if err != nil {
		return false, err
	}
	for _, name := range actionManifestNames {
		found, findErr := s.source.HasFile(ctx, repository, name)
		if findErr != nil {
			return false, fmt.Errorf("verify %s: %w", action, findErr)
		}
		if found {
			return true, nil
		}
	}
	return false, nil
}

func take(results []SearchResult, limit int) []SearchResult {
	if len(results) <= limit {
		return results
	}
	return results[:limit]
}
