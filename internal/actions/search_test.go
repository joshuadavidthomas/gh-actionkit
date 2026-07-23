package actions

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

type fakeSearchSource struct {
	results   []SearchResult
	manifests map[string]bool
	err       error
	fileCalls atomic.Int32
}

func (f *fakeSearchSource) SearchRepositories(context.Context, string, int) ([]SearchResult, error) {
	return f.results, f.err
}

func (f *fakeSearchSource) HasFile(_ context.Context, repository Repository, name string) (bool, error) {
	f.fileCalls.Add(1)
	if f.err != nil {
		return false, f.err
	}
	return f.manifests[repository.Owner+"/"+repository.Name+"/"+name], nil
}

func TestSearchVerifiesAndSortsActions(t *testing.T) {
	source := &fakeSearchSource{
		results: []SearchResult{
			{Action: "owner/popular", Stars: 200},
			{Action: "owner/not-an-action", Stars: 300},
			{Action: "owner/small", Stars: 10},
		},
		manifests: map[string]bool{
			"owner/popular/action.yaml": true,
			"owner/small/action.yml":    true,
		},
	}

	results, err := NewSearchService(source).Search(context.Background(), "build", 10, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Action != "owner/popular" || results[1].Action != "owner/small" {
		t.Fatalf("unexpected results: %#v", results)
	}
}

func TestFastSearchSkipsManifestChecks(t *testing.T) {
	source := &fakeSearchSource{results: []SearchResult{{Action: "owner/repository"}}}

	results, err := NewSearchService(source).Search(context.Background(), "build", 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || source.fileCalls.Load() != 0 {
		t.Fatalf("results=%#v file calls=%d", results, source.fileCalls.Load())
	}
}

func TestSearchReportsVerificationErrors(t *testing.T) {
	sourceErr := errors.New("rate limited")
	source := &fakeSearchSource{
		results: []SearchResult{{Action: "owner/action"}},
		err:     sourceErr,
	}

	_, err := NewSearchService(source).Search(context.Background(), "build", 10, false)
	if !errors.Is(err, sourceErr) {
		t.Fatalf("expected source error, got %v", err)
	}
}

func TestSearchRejectsInvalidLimit(t *testing.T) {
	_, err := NewSearchService(&fakeSearchSource{}).Search(context.Background(), "build", 0, false)
	if err == nil {
		t.Fatal("expected an error")
	}
}
