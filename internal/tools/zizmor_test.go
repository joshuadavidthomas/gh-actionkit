package tools

import (
	"context"
	"errors"
	"io"
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

func TestZizmorReportsMissingExecutable(t *testing.T) {
	zizmor := Zizmor{
		runner: &recordingRunner{},
		lookPath: func(string) (string, error) {
			return "", errors.New("missing")
		},
	}

	_, err := zizmor.Lint(context.Background(), "/repo", false, false, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected an error")
	}
}
