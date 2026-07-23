package tools

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestActionlintReturnsNoFiles(t *testing.T) {
	result, err := (Actionlint{}).Validate(t.TempDir(), false, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Files != 0 || result.Findings != 0 {
		t.Fatalf("unexpected result: %#v", result)
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

	result, err := (Actionlint{}).Validate(repository, true, &stdout, &bytes.Buffer{})
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

func TestWorkflowFilesIgnoreNestedWorkflows(t *testing.T) {
	repository := t.TempDir()
	nested := filepath.Join(repository, ".github", "workflows", "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "ignored.yml"), []byte("invalid"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := workflowFiles(repository)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("unexpected files: %#v", files)
	}
}
