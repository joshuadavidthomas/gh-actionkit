package cli

import (
	"bytes"
	"io"
	"slices"
	"testing"

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
