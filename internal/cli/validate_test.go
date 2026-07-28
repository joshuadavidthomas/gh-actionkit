package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

func TestValidateReturnsFindingStatus(t *testing.T) {
	validate := func(_ context.Context, _ string, outputJSON bool, _, _ io.Writer) (int, int, error) {
		if !outputJSON {
			t.Fatal("expected JSON output")
		}
		return 2, 3, nil
	}
	command := commandForTest(
		newValidateCommandWithValidate(validate),
		&bytes.Buffer{},
		&bytes.Buffer{},
		"-C",
		t.TempDir(),
		"--json",
	)

	err := command.Execute()
	var statusError StatusError
	if !errors.As(err, &statusError) || statusError.Code != 1 {
		t.Fatalf("expected status 1, got %v", err)
	}
}

func TestValidatePassesCommandContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var received context.Context
	validate := func(ctx context.Context, _ string, _ bool, _, _ io.Writer) (int, int, error) {
		received = ctx
		return 0, 0, ctx.Err()
	}
	command := commandForTest(
		newValidateCommandWithValidate(validate),
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
	validate := func(_ context.Context, _ string, _ bool, _, _ io.Writer) (int, int, error) {
		return 0, 0, nil
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := commandForTest(
		newValidateCommandWithValidate(validate),
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
