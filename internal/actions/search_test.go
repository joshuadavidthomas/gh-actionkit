package actions

import (
	"context"
	"errors"
	"testing"
)

type fakeSearchSource struct {
	results []SearchResult
	err     error
	options SearchOptions
}

func (f *fakeSearchSource) SearchRepositories(
	_ context.Context,
	options SearchOptions,
) ([]SearchResult, error) {
	f.options = options
	return f.results, f.err
}

func TestSearchRequestsVerifiedCandidatesAndAppliesLimit(t *testing.T) {
	source := &fakeSearchSource{results: []SearchResult{
		{Action: "owner/popular", Stars: 200},
		{Action: "owner/small", Stars: 10},
	}}

	results, err := NewSearchService(source).Search(context.Background(), "build", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != "owner/popular" {
		t.Fatalf("unexpected results: %#v", results)
	}
	if source.options.Query != "build" || source.options.ResultLimit != 1 ||
		source.options.CandidateLimit != searchCandidates {
		t.Fatalf("unexpected search options: %#v", source.options)
	}
}

func TestSearchReportsSourceErrors(t *testing.T) {
	sourceErr := errors.New("rate limited")
	source := &fakeSearchSource{err: sourceErr}

	_, err := NewSearchService(source).Search(context.Background(), "build", 10)
	if !errors.Is(err, sourceErr) {
		t.Fatalf("expected source error, got %v", err)
	}
}

func TestSearchRejectsInvalidLimit(t *testing.T) {
	_, err := NewSearchService(&fakeSearchSource{}).Search(context.Background(), "build", 0)
	if err == nil {
		t.Fatal("expected an error")
	}
}
