package tools

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"reflect"
	"testing"
)

type recordingRunner struct {
	command Command
	err     error
}

func (r *recordingRunner) Run(_ context.Context, command Command) error {
	r.command = command
	return r.err
}

type processExitError int

func (e processExitError) Error() string { return "process failed" }
func (e processExitError) ExitCode() int { return int(e) }

func TestZizmorBuildsCommandAndPreservesExitCode(t *testing.T) {
	runner := &recordingRunner{err: processExitError(13)}
	zizmor := Zizmor{
		runner: runner,
		lookPath: func(string) (string, error) {
			return "/usr/bin/zizmor", nil
		},
	}

	exitCode, err := zizmor.Lint(context.Background(), "/repo", true, true, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if exitCode != 13 {
		t.Fatalf("got exit code %d", exitCode)
	}
	if runner.command.Path != "/usr/bin/zizmor" || runner.command.Dir != "/repo" {
		t.Fatalf("unexpected command: %#v", runner.command)
	}
	wantArgs := []string{"--collect=workflows", "--no-progress", "--format=json-v1", "--pedantic", "."}
	if !reflect.DeepEqual(runner.command.Args, wantArgs) {
		t.Fatalf("got arguments %#v", runner.command.Args)
	}
}

func TestZizmorFallsBackToUVX(t *testing.T) {
	runner := &recordingRunner{}
	var lookups []string
	zizmor := Zizmor{
		runner: runner,
		lookPath: func(name string) (string, error) {
			lookups = append(lookups, name)
			if name == "uvx" {
				return "/usr/bin/uvx", nil
			}
			return "", exec.ErrNotFound
		},
	}

	exitCode, err := zizmor.Lint(context.Background(), "/repo", false, false, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if exitCode != 0 {
		t.Fatalf("got exit code %d", exitCode)
	}
	if !reflect.DeepEqual(lookups, []string{"zizmor", "uvx"}) {
		t.Fatalf("got executable lookups %#v", lookups)
	}
	if runner.command.Path != "/usr/bin/uvx" {
		t.Fatalf("got command path %q", runner.command.Path)
	}
	wantArgs := []string{"zizmor", "--collect=workflows", "--no-progress", "."}
	if !reflect.DeepEqual(runner.command.Args, wantArgs) {
		t.Fatalf("got arguments %#v", runner.command.Args)
	}
}

func TestZizmorReportsMissingExecutableAndFallback(t *testing.T) {
	var lookups []string
	zizmor := Zizmor{
		runner: &recordingRunner{},
		lookPath: func(name string) (string, error) {
			lookups = append(lookups, name)
			return "", exec.ErrNotFound
		},
	}

	_, err := zizmor.Lint(context.Background(), "/repo", false, false, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !reflect.DeepEqual(lookups, []string{"zizmor", "uvx"}) {
		t.Fatalf("got executable lookups %#v", lookups)
	}
}

func TestZizmorDoesNotHideLookupErrors(t *testing.T) {
	lookupErr := errors.New("permission denied")
	var lookups []string
	zizmor := Zizmor{
		runner: &recordingRunner{},
		lookPath: func(name string) (string, error) {
			lookups = append(lookups, name)
			return "", lookupErr
		},
	}

	_, err := zizmor.Lint(context.Background(), "/repo", false, false, io.Discard, io.Discard)
	if !errors.Is(err, lookupErr) {
		t.Fatalf("got error %v", err)
	}
	if !reflect.DeepEqual(lookups, []string{"zizmor"}) {
		t.Fatalf("got executable lookups %#v", lookups)
	}
}
