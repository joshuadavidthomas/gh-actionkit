package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/joshuadavidthomas/gh-actionkit/internal/tools"
)

func TestValidateReturnsFindingStatus(t *testing.T) {
	validate := func(_ context.Context, _ string, outputJSON bool, _, _ io.Writer) (tools.ValidationResult, error) {
		if !outputJSON {
			t.Fatal("expected JSON output")
		}
		return tools.ValidationResult{Files: 2, Findings: 3}, nil
	}
	command := commandForTest(
		newValidateCommand(validate),
		&bytes.Buffer{},
		&bytes.Buffer{},
		"-C",
		t.TempDir(),
		"--json",
	)

	err := command.Execute()
	if status, ok := ExitStatus(err); !ok || status != 1 {
		t.Fatalf("expected status 1, got %v", err)
	}
}

func TestValidatePassesCommandContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var received context.Context
	validate := func(ctx context.Context, _ string, _ bool, _, _ io.Writer) (tools.ValidationResult, error) {
		received = ctx
		return tools.ValidationResult{}, ctx.Err()
	}
	command := commandForTest(
		newValidateCommand(validate),
		&bytes.Buffer{},
		&bytes.Buffer{},
		"-C",
		t.TempDir(),
	)

	err := command.ExecuteContext(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if received == nil || !errors.Is(received.Err(), context.Canceled) {
		t.Fatalf("validate received uncanceled context: %v", received)
	}
}

func TestValidateReportsNoWorkflowsWithoutPollutingJSON(t *testing.T) {
	validate := func(_ context.Context, _ string, _ bool, _, _ io.Writer) (tools.ValidationResult, error) {
		return tools.ValidationResult{}, nil
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := commandForTest(
		newValidateCommand(validate),
		&stdout,
		&stderr,
		"-C",
		t.TempDir(),
		"--json",
	)

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	if stderr.String() != "No workflow files found in .github/workflows\n" {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}
