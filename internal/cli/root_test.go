package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
)

func TestVersionJSON(t *testing.T) {
	majorSHA := "major-sha"
	latestSHA := "latest-sha"
	lookup := func(context.Context, string) (actions.VersionInfo, error) {
		return actions.VersionInfo{
			Action: "actions/checkout",
			Major:  actions.Version{Tag: "v4", SHA: &majorSHA},
			Latest: actions.Version{Tag: "v4.2.2", SHA: &latestSHA},
		}, nil
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := NewRootCommand(&stdout, &stderr, Dependencies{LookupVersion: lookup})
	command.SetArgs([]string{"version", "actions/checkout", "--json"})

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}

	var output actions.VersionInfo
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("invalid JSON %q: %v", stdout.String(), err)
	}
	if output.Latest.Tag != "v4.2.2" {
		t.Fatalf("unexpected output: %#v", output)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestVersionTextShowsUnknownSHA(t *testing.T) {
	lookup := func(context.Context, string) (actions.VersionInfo, error) {
		return actions.VersionInfo{
			Action: "owner/action",
			Major:  actions.Version{Tag: "v1"},
			Latest: actions.Version{Tag: "v1.2.3"},
		}, nil
	}
	var stdout bytes.Buffer
	command := NewRootCommand(&stdout, &bytes.Buffer{}, Dependencies{LookupVersion: lookup})
	command.SetArgs([]string{"version", "owner/action"})

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(stdout.String(), "sha: unknown"); count != 2 {
		t.Fatalf("expected two unknown SHAs, got %q", stdout.String())
	}
}

func TestVersionReturnsLookupError(t *testing.T) {
	lookupErr := errors.New("authentication failed")
	lookup := func(context.Context, string) (actions.VersionInfo, error) {
		return actions.VersionInfo{}, lookupErr
	}
	command := NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{}, Dependencies{LookupVersion: lookup})
	command.SetArgs([]string{"version", "owner/action"})

	if err := command.Execute(); !errors.Is(err, lookupErr) {
		t.Fatalf("expected lookup error, got %v", err)
	}
}
