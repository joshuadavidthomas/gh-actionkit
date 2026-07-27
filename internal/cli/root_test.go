package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
	"github.com/spf13/cobra"
)

func commandForTest(command *cobra.Command, stdout, stderr io.Writer, args ...string) *cobra.Command {
	command.SetOut(stdout)
	command.SetErr(stderr)
	command.SetArgs(args)
	return command
}

func TestRootVersion(t *testing.T) {
	var stdout bytes.Buffer
	command := NewRootCommand("v1.2.3", nil, &stdout, &bytes.Buffer{})
	command.SetArgs([]string{"--version"})

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "actionkit version v1.2.3\n" {
		t.Fatalf("unexpected version output: %q", stdout.String())
	}
}

func TestRootRegistersCommands(t *testing.T) {
	command := NewRootCommand("dev", nil, &bytes.Buffer{}, &bytes.Buffer{})
	got := make([]string, 0, len(command.Commands()))
	for _, subcommand := range command.Commands() {
		got = append(got, subcommand.Name())
	}
	slices.Sort(got)
	want := []string{"check", "inspect", "lint", "search", "skill", "validate", "version"}
	if !slices.Equal(got, want) {
		t.Fatalf("commands = %v, want %v", got, want)
	}
}

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
	command := commandForTest(
		newVersionCommandWithLookup(lookup),
		&stdout,
		&stderr,
		"actions/checkout",
		"--json",
	)

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

func TestVersionSnippet(t *testing.T) {
	latestSHA := "0123456789abcdef0123456789abcdef01234567"
	lookup := func(context.Context, string) (actions.VersionInfo, error) {
		return actions.VersionInfo{
			Action: "actions/checkout",
			Latest: actions.Version{Tag: "v4.2.2", SHA: &latestSHA},
		}, nil
	}
	var stdout bytes.Buffer
	command := commandForTest(
		newVersionCommandWithLookup(lookup),
		&stdout,
		&bytes.Buffer{},
		"actions/checkout",
		"--snippet",
	)

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	want := "uses: actions/checkout@0123456789abcdef0123456789abcdef01234567 # v4.2.2\n"
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
			lookup := func(context.Context, string) (actions.VersionInfo, error) {
				return actions.VersionInfo{
					Action: "owner/action",
					Latest: actions.Version{Tag: "v1.2.3", SHA: test.sha},
				}, nil
			}
			var stdout bytes.Buffer
			command := commandForTest(
				newVersionCommandWithLookup(lookup),
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
	lookup := func(context.Context, string) (actions.VersionInfo, error) {
		return actions.VersionInfo{}, nil
	}
	command := commandForTest(
		newVersionCommandWithLookup(lookup),
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
	lookup := func(context.Context, string) (actions.VersionInfo, error) {
		return actions.VersionInfo{
			Action: "owner/action",
			Major:  actions.Version{Tag: "v1"},
			Latest: actions.Version{Tag: "v1.2.3"},
		}, nil
	}
	var stdout bytes.Buffer
	command := commandForTest(newVersionCommandWithLookup(lookup), &stdout, &bytes.Buffer{}, "owner/action")

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
	command := commandForTest(
		newVersionCommandWithLookup(lookup),
		&bytes.Buffer{},
		&bytes.Buffer{},
		"owner/action",
	)

	if err := command.Execute(); !errors.Is(err, lookupErr) {
		t.Fatalf("expected lookup error, got %v", err)
	}
}
