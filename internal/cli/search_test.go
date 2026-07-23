package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
)

func TestSearchJSONUsesEmptyArray(t *testing.T) {
	search := func(context.Context, string, int, bool) ([]actions.SearchResult, error) {
		return []actions.SearchResult{}, nil
	}
	var stdout bytes.Buffer
	command := commandForTest(newSearchCommandWithSearch(search), &stdout, &bytes.Buffer{}, "missing", "--json")

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "[]\n" {
		t.Fatalf("unexpected JSON: %q", stdout.String())
	}
}

func TestSearchForwardsOptions(t *testing.T) {
	search := func(_ context.Context, query string, limit int, fast bool) ([]actions.SearchResult, error) {
		if query != "docker build" || limit != 3 || !fast {
			t.Fatalf("query=%q limit=%d fast=%v", query, limit, fast)
		}
		return []actions.SearchResult{{Action: "docker/build-push-action", Stars: 7100}}, nil
	}
	var stdout bytes.Buffer
	command := commandForTest(
		newSearchCommandWithSearch(search),
		&stdout,
		&bytes.Buffer{},
		"docker build",
		"-n",
		"3",
		"--fast",
	)

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "docker/build-push-action (⭐ 7.1k)\n\n" {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}
