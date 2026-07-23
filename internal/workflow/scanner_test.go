package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanRepositoryFindsStructuredRemoteUses(t *testing.T) {
	repository := t.TempDir()
	workflowDirectory := filepath.Join(repository, ".github", "workflows")
	if err := os.MkdirAll(workflowDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `name: CI
on: push
jobs:
  reusable:
    uses: owner/workflows/.github/workflows/test.yml@v2
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: "owner/repo/subaction@0123456789012345678901234567890123456789" # pinned
      - uses: ./local-action
      - uses: docker://alpine:3
      - run: |
          uses: fake/action@v1
      - run: echo ignored
        env:
          uses: fake/env@v1
        with:
          uses: fake/input@v1
`
	if err := os.WriteFile(filepath.Join(workflowDirectory, "ci.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := ScanRepository(repository)
	if err != nil {
		t.Fatal(err)
	}
	if result.Files != 1 || len(result.Uses) != 3 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Uses[0].Action != "owner/workflows/.github/workflows/test.yml" || result.Uses[0].Ref != "v2" {
		t.Fatalf("unexpected reusable workflow: %#v", result.Uses[0])
	}
	if result.Uses[1].Location.Line != 9 || result.Uses[1].Location.File != ".github/workflows/ci.yml" {
		t.Fatalf("unexpected checkout location: %#v", result.Uses[1].Location)
	}
	if result.Uses[2].Repository.Name != "repo" {
		t.Fatalf("subpath repository parsed incorrectly: %#v", result.Uses[2])
	}
}

func TestScanRepositoryReportsMalformedYAML(t *testing.T) {
	repository := t.TempDir()
	workflowDirectory := filepath.Join(repository, ".github", "workflows")
	if err := os.MkdirAll(workflowDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workflowDirectory, "bad.yml"), []byte("jobs: ["), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := ScanRepository(repository); err == nil {
		t.Fatal("expected an error")
	}
}
