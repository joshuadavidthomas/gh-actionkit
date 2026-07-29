package workflow

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestFindFilesReturnsSortedDirectWorkflowYAMLFiles(t *testing.T) {
	repository := t.TempDir()
	workflowDirectory := filepath.Join(repository, ".github", "workflows")
	if err := os.MkdirAll(filepath.Join(workflowDirectory, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		"z.yaml":             "name: Z",
		"a.yml":              "name: A",
		"ignored.txt":        "ignored",
		"nested/ignored.yml": "name: Nested",
	} {
		if err := os.WriteFile(filepath.Join(workflowDirectory, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	files, err := FindFiles(repository)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		filepath.Join(workflowDirectory, "a.yml"),
		filepath.Join(workflowDirectory, "z.yaml"),
	}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("got files %#v, want %#v", files, want)
	}
}

func TestFindFilesSkipsSymlinkedWorkflowEntries(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks requires privileges on Windows")
	}

	repository := t.TempDir()
	workflowDirectory := filepath.Join(repository, ".github", "workflows")
	if err := os.MkdirAll(workflowDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workflowDirectory, "a.yml"), []byte(`jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
`), 0o644); err != nil {
		t.Fatal(err)
	}

	outside := t.TempDir()
	target := filepath.Join(outside, "secret.yml")
	if err := os.WriteFile(target, []byte("secret: data\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(workflowDirectory, "evil.yml")); err != nil {
		t.Fatal(err)
	}

	files, err := FindFiles(repository)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(workflowDirectory, "a.yml")}
	if !reflect.DeepEqual(files, want) {
		t.Fatalf("got files %#v, want %#v", files, want)
	}

	result, err := ScanRepository(repository)
	if err != nil {
		t.Fatal(err)
	}
	if result.Files != 1 || len(result.Uses) != 1 || result.Uses[0].Identifier.String() != "actions/checkout" {
		t.Fatalf("unexpected scan result: %#v", result)
	}
}

func TestFindFilesSkipsSymlinkedWorkflowDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks requires privileges on Windows")
	}

	repository := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repository, ".github"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "ci.yml"), []byte("name: CI\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(repository, ".github", "workflows")); err != nil {
		t.Fatal(err)
	}

	files, err := FindFiles(repository)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("got files %#v, want none", files)
	}
}

func TestFindFilesSkipsSymlinkedGitHubDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks requires privileges on Windows")
	}

	repository := t.TempDir()
	outside := t.TempDir()
	workflowDirectory := filepath.Join(outside, "workflows")
	if err := os.MkdirAll(workflowDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workflowDirectory, "ci.yml"), []byte("name: CI\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(repository, ".github")); err != nil {
		t.Fatal(err)
	}

	files, err := FindFiles(repository)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("got files %#v, want none", files)
	}
}

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
	if result.Uses[0].Identifier.String() != "owner/workflows/.github/workflows/test.yml" || result.Uses[0].Ref != "v2" {
		t.Fatalf("unexpected reusable workflow: %#v", result.Uses[0])
	}
	if result.Uses[1].Location.Line != 9 || result.Uses[1].Location.File != ".github/workflows/ci.yml" {
		t.Fatalf("unexpected checkout location: %#v", result.Uses[1].Location)
	}
	if result.Uses[2].Identifier.Repository().Name != "repo" {
		t.Fatalf("subpath repository parsed incorrectly: %#v", result.Uses[2])
	}
}

func TestScanRepositoryResolvesUsesAliases(t *testing.T) {
	repository := t.TempDir()
	workflowDirectory := filepath.Join(repository, ".github", "workflows")
	if err := os.MkdirAll(workflowDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `name: Aliases
on: push
jobs:
  source:
    runs-on: ubuntu-latest
    env:
      ACTION: &action actions/checkout@v4
      WORKFLOW: &workflow owner/workflows/.github/workflows/reuse.yml@v2
      BAD: &bad
        nested: owner/ignored@v1
    steps: &shared
      - uses: owner/shared@v1
  direct:
    runs-on: ubuntu-latest
    steps:
      - uses: *action
      - uses: *bad
  shared:
    runs-on: ubuntu-latest
    steps: *shared
  reusable:
    uses: *workflow
`
	if err := os.WriteFile(filepath.Join(workflowDirectory, "aliases.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := ScanRepository(repository)
	if err != nil {
		t.Fatal(err)
	}
	if result.Files != 1 || len(result.Uses) != 4 {
		t.Fatalf("unexpected result: %#v", result)
	}
	want := []struct {
		action string
		ref    string
		line   int
	}{
		{action: "owner/shared", ref: "v1", line: 12},
		{action: "actions/checkout", ref: "v4", line: 16},
		// An aliased sequence reuses the anchor's child nodes and their locations.
		{action: "owner/shared", ref: "v1", line: 12},
		{action: "owner/workflows/.github/workflows/reuse.yml", ref: "v2", line: 22},
	}
	for index, expected := range want {
		use := result.Uses[index]
		if use.Identifier.String() != expected.action || use.Ref != expected.ref || use.Location.File != ".github/workflows/aliases.yml" || use.Location.Line != expected.line {
			t.Errorf("use %d = %#v, want action %q, ref %q, file %q, line %d", index, use, expected.action, expected.ref, ".github/workflows/aliases.yml", expected.line)
		}
	}
}

func TestScanRepositoryResolvesAliasedJobsAndSteps(t *testing.T) {
	repository := t.TempDir()
	workflowDirectory := filepath.Join(repository, ".github", "workflows")
	if err := os.MkdirAll(workflowDirectory, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `jobs:
  source: &job
    uses: owner/workflows/.github/workflows/reuse.yml@v1
  copied: *job
  runner:
    runs-on: ubuntu-latest
    steps:
      - &step
        uses: owner/action@v2
      - *step
`
	if err := os.WriteFile(filepath.Join(workflowDirectory, "aliased-structures.yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := ScanRepository(repository)
	if err != nil {
		t.Fatal(err)
	}
	if result.Files != 1 || len(result.Uses) != 4 {
		t.Fatalf("unexpected result: %#v", result)
	}
	want := []struct {
		action string
		ref    string
		line   int
	}{
		{action: "owner/workflows/.github/workflows/reuse.yml", ref: "v1", line: 3},
		{action: "owner/workflows/.github/workflows/reuse.yml", ref: "v1", line: 3},
		{action: "owner/action", ref: "v2", line: 9},
		{action: "owner/action", ref: "v2", line: 9},
	}
	for index, expected := range want {
		use := result.Uses[index]
		if use.Identifier.String() != expected.action || use.Ref != expected.ref || use.Location.Line != expected.line {
			t.Errorf("use %d = %#v, want action %q, ref %q, line %d", index, use, expected.action, expected.ref, expected.line)
		}
	}
}

func TestParseUseRejectsMalformedActionIdentifiers(t *testing.T) {
	for _, spec := range []string{
		"owner/repo/@v1",
		"owner/repo//path@v1",
		"owner//path@v1",
		"/repo@v1",
	} {
		if _, ok := parseUse(spec, "workflow.yml", 1); ok {
			t.Errorf("parseUse(%q) accepted malformed identifier", spec)
		}
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
