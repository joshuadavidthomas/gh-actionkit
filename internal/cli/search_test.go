package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
)

func TestSearchJSONUsesEmptyArray(t *testing.T) {
	search := func(context.Context, string, int) ([]actions.SearchResult, error) {
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

func TestSearchSanitizesHumanOutputAndPreservesJSON(t *testing.T) {
	description := "link \x1b]8;;https://evil.example\x07click\x1b]8;;\x07"
	results := []actions.SearchResult{{
		Action:      "owner/evil\x1b[2Jaction",
		Description: &description,
		Stars:       1,
	}}
	search := func(context.Context, string, int) ([]actions.SearchResult, error) {
		return results, nil
	}

	var humanOutput bytes.Buffer
	humanCommand := commandForTest(
		newSearchCommandWithSearch(search),
		&humanOutput,
		&bytes.Buffer{},
		"evil",
	)
	if err := humanCommand.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsRune(humanOutput.String(), '\x1b') {
		t.Fatalf("human output contains untrusted ESC: %q", humanOutput.String())
	}
	for _, text := range []string{"owner/evil�[2Jaction", "link �]8;;https://evil.example�click�]8;;�"} {
		if !strings.Contains(humanOutput.String(), text) {
			t.Errorf("human output does not contain %q: %q", text, humanOutput.String())
		}
	}

	var jsonOutput bytes.Buffer
	jsonCommand := commandForTest(
		newSearchCommandWithSearch(search),
		&jsonOutput,
		&bytes.Buffer{},
		"evil",
		"--json",
	)
	if err := jsonCommand.Execute(); err != nil {
		t.Fatal(err)
	}
	var decoded []actions.SearchResult
	if err := json.Unmarshal(jsonOutput.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON %q: %v", jsonOutput.String(), err)
	}
	if len(decoded) != 1 || decoded[0].Action != results[0].Action || decoded[0].Description == nil || *decoded[0].Description != description {
		t.Fatalf("JSON changed control-character payload: %#v", decoded)
	}
}

func TestSearchForwardsOptions(t *testing.T) {
	search := func(_ context.Context, query string, limit int) ([]actions.SearchResult, error) {
		if query != "docker build" || limit != 3 {
			t.Fatalf("query=%q limit=%d", query, limit)
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
	)

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "docker/build-push-action (⭐ 7.1k)\n\n" {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}
