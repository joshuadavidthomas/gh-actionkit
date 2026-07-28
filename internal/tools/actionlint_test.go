package tools

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestActionlintReturnsNoFiles(t *testing.T) {
	result, err := (Actionlint{}).Validate(context.Background(), t.TempDir(), false, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Files != 0 || result.Findings != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestActionlintReturnsCanceledContext(t *testing.T) {
	repository := t.TempDir()
	workflows := filepath.Join(repository, ".github", "workflows")
	if err := os.MkdirAll(workflows, 0o755); err != nil {
		t.Fatal(err)
	}
	workflow := "name: CI\non: push\njobs: {}\n"
	if err := os.WriteFile(filepath.Join(workflows, "ci.yml"), []byte(workflow), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := (Actionlint{}).Validate(ctx, repository, false, io.Discard, io.Discard)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if result != (ValidationResult{}) {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestActionlintReturnsWhenCanceledDuringLint(t *testing.T) {
	repository := t.TempDir()
	workflows := filepath.Join(repository, ".github", "workflows")
	if err := os.MkdirAll(workflows, 0o755); err != nil {
		t.Fatal(err)
	}
	workflow := "name: Broken\non: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - uses:\n"
	if err := os.WriteFile(filepath.Join(workflows, "broken.yml"), []byte(workflow), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	writer := newBlockingWriter()
	t.Cleanup(writer.release)
	type validationOutcome struct {
		result ValidationResult
		err    error
	}
	done := make(chan validationOutcome, 1)
	go func() {
		result, err := (Actionlint{}).Validate(ctx, repository, true, writer, io.Discard)
		done <- validationOutcome{result: result, err: err}
	}()

	select {
	case <-writer.started:
	case outcome := <-done:
		t.Fatalf("Validate returned before writing lint output: result %#v, error %v", outcome.result, outcome.err)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for lint output")
	}
	cancel()

	select {
	case outcome := <-done:
		if !errors.Is(outcome.err, context.Canceled) {
			t.Fatalf("expected context cancellation, got %v", outcome.err)
		}
		if outcome.result != (ValidationResult{}) {
			t.Fatalf("unexpected result: %#v", outcome.result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Validate did not return after cancellation")
	}
}

func TestActionlintValidatesDirectWorkflowFilesAsJSONLines(t *testing.T) {
	repository := t.TempDir()
	workflows := filepath.Join(repository, ".github", "workflows")
	if err := os.MkdirAll(workflows, 0o755); err != nil {
		t.Fatal(err)
	}
	workflow := "name: Broken\non: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - uses:\n"
	if err := os.WriteFile(filepath.Join(workflows, "broken.yml"), []byte(workflow), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer

	result, err := (Actionlint{}).Validate(context.Background(), repository, true, &stdout, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Files != 1 || result.Findings == 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if !strings.Contains(stdout.String(), `"filepath"`) || !strings.HasSuffix(stdout.String(), "\n") {
		t.Fatalf("expected JSON Lines output, got %q", stdout.String())
	}
}

type blockingWriter struct {
	started     chan struct{}
	unblock     chan struct{}
	startedOnce sync.Once
	releaseOnce sync.Once
}

func newBlockingWriter() *blockingWriter {
	return &blockingWriter{
		started: make(chan struct{}),
		unblock: make(chan struct{}),
	}
}

func (w *blockingWriter) Write(data []byte) (int, error) {
	w.startedOnce.Do(func() { close(w.started) })
	<-w.unblock
	return len(data), nil
}

func (w *blockingWriter) release() {
	w.releaseOnce.Do(func() { close(w.unblock) })
}
