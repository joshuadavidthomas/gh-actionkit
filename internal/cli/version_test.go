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
	lookup := func(_ context.Context, repository actions.Repository) (actions.RepositoryVersions, error) {
		if repository != (actions.Repository{Owner: "github", Name: "codeql-action"}) {
			t.Fatalf("repository = %#v", repository)
		}
		return actions.RepositoryVersions{
			Major:  actions.Version{Tag: "v4", SHA: &majorSHA},
			Latest: actions.Version{Tag: "v4.2.2", SHA: &latestSHA},
		}, nil
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := commandForTest(
		newVersionCommand(lookup),
		&stdout,
		&stderr,
		"github/codeql-action/init",
		"--json",
	)

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}

	var output versionOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("invalid JSON %q: %v", stdout.String(), err)
	}
	if output.Action != "github/codeql-action/init" || output.Latest.Tag != "v4.2.2" {
		t.Fatalf("unexpected output: %#v", output)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestVersionSnippet(t *testing.T) {
	latestSHA := "0123456789abcdef0123456789abcdef01234567"
	lookup := func(context.Context, actions.Repository) (actions.RepositoryVersions, error) {
		return actions.RepositoryVersions{
			Latest: actions.Version{Tag: "v4.2.2", SHA: &latestSHA},
		}, nil
	}
	var stdout bytes.Buffer
	command := commandForTest(
		newVersionCommand(lookup),
		&stdout,
		&bytes.Buffer{},
		"github/codeql-action/init",
		"--snippet",
	)

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	want := "uses: github/codeql-action/init@0123456789abcdef0123456789abcdef01234567 # v4.2.2\n"
	if stdout.String() != want {
		t.Fatalf("snippet = %q, want %q", stdout.String(), want)
	}
}

func TestVersionSnippetRequiresFullCommitSHA(t *testing.T) {
	tests := []struct {
		name string
		sha  *string
	}{
		{name: "missing"},
		{name: "empty", sha: testStringPointer("")},
		{name: "short", sha: testStringPointer("0123456789ab")},
		{name: "non hexadecimal", sha: testStringPointer("zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lookup := func(context.Context, actions.Repository) (actions.RepositoryVersions, error) {
				return actions.RepositoryVersions{
					Latest: actions.Version{Tag: "v1.2.3", SHA: test.sha},
				}, nil
			}
			var stdout bytes.Buffer
			command := commandForTest(
				newVersionCommand(lookup),
				&stdout,
				&bytes.Buffer{},
				"owner/action",
				"--snippet",
			)
			command.SilenceUsage = true

			err := command.Execute()
			if err == nil || !strings.Contains(err.Error(), "does not resolve to a full commit SHA") {
				t.Fatalf("unexpected error: %v", err)
			}
			if stdout.Len() != 0 {
				t.Fatalf("unexpected output: %q", stdout.String())
			}
		})
	}
}

func TestVersionRejectsMultipleOutputFormats(t *testing.T) {
	lookup := func(context.Context, actions.Repository) (actions.RepositoryVersions, error) {
		return actions.RepositoryVersions{}, nil
	}
	command := commandForTest(
		newVersionCommand(lookup),
		&bytes.Buffer{},
		&bytes.Buffer{},
		"owner/action",
		"--json",
		"--snippet",
	)

	if err := command.Execute(); err == nil {
		t.Fatal("expected conflicting output format error")
	}
}

func TestVersionTextShowsUnknownSHA(t *testing.T) {
	lookup := func(context.Context, actions.Repository) (actions.RepositoryVersions, error) {
		return actions.RepositoryVersions{
			Major:  actions.Version{Tag: "v1"},
			Latest: actions.Version{Tag: "v1.2.3"},
		}, nil
	}
	var stdout bytes.Buffer
	command := commandForTest(newVersionCommand(lookup), &stdout, &bytes.Buffer{}, "owner/action")

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(stdout.String(), "sha: unknown"); count != 2 {
		t.Fatalf("expected two unknown SHAs, got %q", stdout.String())
	}
}

func TestVersionRejectsMalformedIdentifierBeforeLookup(t *testing.T) {
	called := false
	lookup := func(context.Context, actions.Repository) (actions.RepositoryVersions, error) {
		called = true
		return actions.RepositoryVersions{}, nil
	}
	command := commandForTest(
		newVersionCommand(lookup),
		&bytes.Buffer{},
		&bytes.Buffer{},
		"owner/repo//path",
	)

	if err := command.Execute(); err == nil {
		t.Fatal("expected an error")
	}
	if called {
		t.Fatal("lookup ran for malformed identifier")
	}
}

func TestVersionReturnsLookupError(t *testing.T) {
	lookupErr := errors.New("authentication failed")
	lookup := func(context.Context, actions.Repository) (actions.RepositoryVersions, error) {
		return actions.RepositoryVersions{}, lookupErr
	}
	command := commandForTest(
		newVersionCommand(lookup),
		&bytes.Buffer{},
		&bytes.Buffer{},
		"owner/action",
	)

	if err := command.Execute(); !errors.Is(err, lookupErr) {
		t.Fatalf("expected lookup error, got %v", err)
	}
}
