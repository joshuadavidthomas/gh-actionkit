package tools

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateReturnsNoFiles(t *testing.T) {
	result, err := Validate(context.Background(), t.TempDir(), false, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Files != 0 || result.Findings != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestValidateReturnsCanceledContext(t *testing.T) {
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

	result, err := Validate(ctx, repository, false, io.Discard, io.Discard)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if result != (ValidationResult{}) {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestValidateDirectWorkflowFilesAsJSONLines(t *testing.T) {
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

	result, err := Validate(context.Background(), repository, true, &stdout, &bytes.Buffer{})
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
