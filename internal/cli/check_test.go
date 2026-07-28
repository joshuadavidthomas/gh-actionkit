package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
	"github.com/joshuadavidthomas/gh-actionkit/internal/workflow"
)

func TestCheckWritesJSONBeforeReturningFindingStatus(t *testing.T) {
	tag := "v3"
	check := func(context.Context, string) (actions.CheckReport, error) {
		return actions.CheckReport{
			WorkflowFiles: 1,
			Uses:          1,
			Results: []actions.CheckResult{{
				Action: "actions/checkout",
				Used:   actions.CheckVersion{Tag: &tag},
				Status: actions.CheckStatusUpdateAvailable,
			}},
		}, nil
	}
	var stdout bytes.Buffer
	command := commandForTest(newCheckCommandWithCheck(check), &stdout, &bytes.Buffer{}, "-C", t.TempDir(), "--json")

	err := command.Execute()
	var statusError StatusError
	if !errors.As(err, &statusError) || statusError.Code != 1 {
		t.Fatalf("expected status 1, got %v", err)
	}
	if !strings.Contains(stdout.String(), `"status": "update_available"`) {
		t.Fatalf("unexpected JSON: %q", stdout.String())
	}
	if strings.Contains(stdout.String(), `"up_to_date":`) || strings.Contains(stdout.String(), `"update_available":`) {
		t.Fatalf("unexpected legacy JSON fields: %q", stdout.String())
	}
	if strings.Contains(stdout.String(), `"short"`) || !strings.Contains(stdout.String(), `"major"`) {
		t.Fatalf("unexpected version fields: %q", stdout.String())
	}
}

func TestCheckPoliciesWriteViolationsBeforeReturningFindingStatus(t *testing.T) {
	check := func(context.Context, string) (actions.CheckReport, error) {
		return actions.CheckReport{
			WorkflowFiles: 1,
			Uses:          1,
			Results: []actions.CheckResult{{
				Action: "third-party/action",
				Ref:    "main",
				Status: actions.CheckStatusUnknown,
			}},
		}, nil
	}
	var stdout bytes.Buffer
	command := commandForTest(
		newCheckCommandWithCheck(check),
		&stdout,
		&bytes.Buffer{},
		"-C",
		t.TempDir(),
		"--json",
		"--require-sha",
		"--fail-on-unknown",
		"--allow-owner",
		"actions",
	)

	err := command.Execute()
	var statusError StatusError
	if !errors.As(err, &statusError) || statusError.Code != 1 {
		t.Fatalf("expected status 1, got %v", err)
	}
	for _, violation := range []string{`"unpinned"`, `"unknown"`, `"disallowed_owner"`} {
		if !strings.Contains(stdout.String(), violation) {
			t.Fatalf("JSON does not contain %s: %q", violation, stdout.String())
		}
	}
}

func TestCheckAcceptsRepeatedAllowedOwners(t *testing.T) {
	check := func(context.Context, string) (actions.CheckReport, error) {
		return actions.CheckReport{
			WorkflowFiles: 1,
			Uses:          3,
			Results: []actions.CheckResult{
				{Action: "actions/checkout", Status: actions.CheckStatusUpToDate},
				{Action: "github/codeql-action", Status: actions.CheckStatusUpToDate},
				{Action: "third-party/action", Status: actions.CheckStatusUpToDate},
			},
		}, nil
	}
	var stdout bytes.Buffer
	command := commandForTest(
		newCheckCommandWithCheck(check),
		&stdout,
		&bytes.Buffer{},
		"-C",
		t.TempDir(),
		"--json",
		"--allow-owner",
		"actions",
		"--allow-owner",
		"github",
	)

	err := command.Execute()
	var statusError StatusError
	if !errors.As(err, &statusError) || statusError.Code != 1 {
		t.Fatalf("expected status 1, got %v", err)
	}
	if count := strings.Count(stdout.String(), `"disallowed_owner"`); count != 1 {
		t.Fatalf("disallowed owner violations = %d, want 1: %q", count, stdout.String())
	}
}

func TestCheckUnknownDoesNotFailWithoutPolicy(t *testing.T) {
	check := func(context.Context, string) (actions.CheckReport, error) {
		return actions.CheckReport{
			WorkflowFiles: 1,
			Uses:          1,
			Results: []actions.CheckResult{{
				Action: "owner/action",
				Ref:    "main",
				Status: actions.CheckStatusUnknown,
			}},
		}, nil
	}
	command := commandForTest(newCheckCommandWithCheck(check), &bytes.Buffer{}, &bytes.Buffer{}, "-C", t.TempDir())

	if err := command.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckEmptyJSONExplainsScanResultOnStderr(t *testing.T) {
	tests := []struct {
		name       string
		report     actions.CheckReport
		wantStderr string
	}{
		{
			name:       "no workflow files",
			report:     actions.CheckReport{Results: []actions.CheckResult{}},
			wantStderr: "No workflow files found in .github/workflows\n",
		},
		{
			name: "no remote action uses",
			report: actions.CheckReport{
				WorkflowFiles: 1,
				Results:       []actions.CheckResult{},
			},
			wantStderr: "No remote action uses found in workflow files\n",
		},
		{
			name: "clean result",
			report: actions.CheckReport{
				WorkflowFiles: 1,
				Uses:          1,
				Results:       []actions.CheckResult{},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			check := func(context.Context, string) (actions.CheckReport, error) {
				return test.report, nil
			}
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			command := commandForTest(newCheckCommandWithCheck(check), &stdout, &stderr, "-C", t.TempDir(), "--json")

			if err := command.Execute(); err != nil {
				t.Fatal(err)
			}
			if stdout.String() != "[]\n" {
				t.Fatalf("unexpected JSON: %q", stdout.String())
			}
			if stderr.String() != test.wantStderr {
				t.Fatalf("unexpected stderr: %q", stderr.String())
			}
		})
	}
}

type endToEndVersionSource struct {
	releases map[actions.Repository]string
	refs     map[actions.Repository]map[string]string
}

func (s endToEndVersionSource) LatestRelease(_ context.Context, repository actions.Repository) (string, bool, error) {
	tag, found := s.releases[repository]
	return tag, found, nil
}

func (endToEndVersionSource) Tags(context.Context, actions.Repository) ([]string, error) {
	return nil, nil
}

func (s endToEndVersionSource) ResolveTag(
	_ context.Context,
	repository actions.Repository,
	tag string,
) (string, bool, error) {
	sha, found := s.refs[repository][tag]
	return sha, found, nil
}

func TestCheckCommandEndToEnd(t *testing.T) {
	checkoutSHA := "1111111111111111111111111111111111111111"
	setupGoV4SHA := "2222222222222222222222222222222222222222"
	setupGoV5SHA := "3333333333333333333333333333333333333333"
	mysterySHA := "4444444444444444444444444444444444444444"
	otherToolSHA := "5555555555555555555555555555555555555555"

	source := endToEndVersionSource{
		releases: map[actions.Repository]string{
			{Owner: "actions", Name: "checkout"}: "v4.2.0",
			{Owner: "actions", Name: "setup-go"}: "v5.0.0",
			{Owner: "example", Name: "mystery"}:  "v1.0.0",
			{Owner: "other", Name: "tool"}:       "v1.0.0",
		},
		refs: map[actions.Repository]map[string]string{
			{Owner: "actions", Name: "checkout"}: {
				"v4":     checkoutSHA,
				"v4.2.0": checkoutSHA,
			},
			{Owner: "actions", Name: "setup-go"}: {
				"v4":     setupGoV4SHA,
				"v5":     setupGoV5SHA,
				"v5.0.0": setupGoV5SHA,
			},
			{Owner: "example", Name: "mystery"}: {
				"v1":     "6666666666666666666666666666666666666666",
				"v1.0.0": "6666666666666666666666666666666666666666",
			},
			{Owner: "other", Name: "tool"}: {
				"v1":     otherToolSHA,
				"v1.0.0": otherToolSHA,
			},
		},
	}

	directory := writeCheckWorkflow(t, checkoutSHA, mysterySHA)
	check := realCheckWithSource(source)

	t.Run("JSON captures classifications, policies, and locations", func(t *testing.T) {
		var stdout bytes.Buffer
		command := commandForTest(
			newCheckCommandWithCheck(check),
			&stdout,
			&bytes.Buffer{},
			"--repo",
			directory,
			"--json",
			"--require-sha",
			"--fail-on-unknown",
			"--allow-owner",
			"actions",
			"--allow-owner",
			"example",
		)
		command.SilenceUsage = true

		requireFindingStatus(t, command.Execute())

		var results []actions.CheckResult
		if err := json.Unmarshal(stdout.Bytes(), &results); err != nil {
			t.Fatalf("decode JSON %q: %v", stdout.String(), err)
		}
		if len(results) != 4 {
			t.Fatalf("results = %#v", results)
		}

		byAction := make(map[string]actions.CheckResult, len(results))
		for _, result := range results {
			byAction[result.Action] = result
		}
		assertEndToEndResult(t, byAction["actions/checkout"], checkoutSHA, true, actions.CheckStatusUpToDate, nil, 6, "", checkoutSHA, "v4", checkoutSHA, "v4.2.0", checkoutSHA)
		assertEndToEndResult(t, byAction["actions/setup-go"], "v4", false, actions.CheckStatusUpdateAvailable, []actions.PolicyViolation{actions.PolicyViolationUnpinned}, 10, "v4", setupGoV4SHA, "v5", setupGoV5SHA, "v5.0.0", setupGoV5SHA)
		assertEndToEndResult(t, byAction["example/mystery"], mysterySHA, true, actions.CheckStatusUnknown, []actions.PolicyViolation{actions.PolicyViolationUnknown}, 14, "", mysterySHA, "v1", "6666666666666666666666666666666666666666", "v1.0.0", "6666666666666666666666666666666666666666")
		assertEndToEndResult(t, byAction["other/tool"], "v1", false, actions.CheckStatusUpToDate, []actions.PolicyViolation{actions.PolicyViolationUnpinned, actions.PolicyViolationDisallowedOwner}, 18, "v1", otherToolSHA, "v1", otherToolSHA, "v1.0.0", otherToolSHA)
	})

	t.Run("human output shows each classification", func(t *testing.T) {
		var stdout bytes.Buffer
		command := commandForTest(
			newCheckCommandWithCheck(check),
			&stdout,
			&bytes.Buffer{},
			"--repo",
			directory,
		)

		requireFindingStatus(t, command.Execute())
		for _, status := range []string{"up to date", "update available", "unknown"} {
			if !strings.Contains(stdout.String(), status) {
				t.Errorf("output does not contain %q:\n%s", status, stdout.String())
			}
		}
	})

	t.Run("only current refs return success", func(t *testing.T) {
		directory := t.TempDir()
		writeWorkflow(t, directory, "name: Current\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@"+checkoutSHA+"\n")
		command := commandForTest(newCheckCommandWithCheck(check), &bytes.Buffer{}, &bytes.Buffer{}, "--repo", directory)

		if err := command.Execute(); err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
	})

	t.Run("policy violations return a finding status without updates", func(t *testing.T) {
		directory := t.TempDir()
		writeWorkflow(t, directory, "name: Policy\njobs:\n  test:\n    steps:\n      - uses: other/tool@v1\n")
		command := commandForTest(
			newCheckCommandWithCheck(check),
			&bytes.Buffer{},
			&bytes.Buffer{},
			"--repo",
			directory,
			"--require-sha",
			"--allow-owner",
			"other",
		)

		requireFindingStatus(t, command.Execute())
	})

	t.Run("empty scan keeps JSON on stdout and explains on stderr", func(t *testing.T) {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		command := commandForTest(newCheckCommandWithCheck(check), &stdout, &stderr, "--repo", t.TempDir(), "--json")

		if err := command.Execute(); err != nil {
			t.Fatal(err)
		}
		if stdout.String() != "[]\n" {
			t.Fatalf("stdout = %q", stdout.String())
		}
		if stderr.String() != "No workflow files found in .github/workflows\n" {
			t.Fatalf("stderr = %q", stderr.String())
		}
	})
}

func writeCheckWorkflow(t *testing.T, checkoutSHA, mysterySHA string) string {
	t.Helper()
	directory := t.TempDir()
	writeWorkflow(t, directory, "name: Check\njobs:\n  checkout:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@"+checkoutSHA+"\n  update:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/setup-go@v4\n  mystery:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: example/mystery@"+mysterySHA+"\n  other:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: other/tool@v1\n")
	return directory
}

func writeWorkflow(t *testing.T, directory, content string) {
	t.Helper()
	path := filepath.Join(directory, ".github", "workflows", "ci.yml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func realCheckWithSource(source actions.VersionSource) actionCheck {
	return func(ctx context.Context, repository string) (actions.CheckReport, error) {
		scan, err := workflow.ScanRepository(repository)
		if err != nil {
			return actions.CheckReport{}, err
		}
		report := actions.CheckReport{WorkflowFiles: scan.Files, Uses: len(scan.Uses), Results: []actions.CheckResult{}}
		if len(scan.Uses) == 0 {
			return report, nil
		}
		report.Results, err = actions.NewCheckService(source).Check(ctx, scan.Uses)
		return report, err
	}
}

func requireFindingStatus(t *testing.T, err error) {
	t.Helper()
	var statusError StatusError
	if !errors.As(err, &statusError) || statusError.Code != 1 {
		t.Fatalf("expected status 1, got %v", err)
	}
}

func assertEndToEndResult(
	t *testing.T,
	result actions.CheckResult,
	ref string,
	pinned bool,
	status actions.CheckStatus,
	violations []actions.PolicyViolation,
	line int,
	usedTag, usedSHA, majorTag, majorSHA, latestTag, latestSHA string,
) {
	t.Helper()
	if result.Ref != ref || result.Pinned != pinned || result.Status != status {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(result.PolicyViolations) != len(violations) {
		t.Fatalf("policy violations = %#v, want %#v", result.PolicyViolations, violations)
	}
	for index := range violations {
		if result.PolicyViolations[index] != violations[index] {
			t.Fatalf("policy violations = %#v, want %#v", result.PolicyViolations, violations)
		}
	}
	if len(result.Locations) != 1 || result.Locations[0] != (actions.Location{File: ".github/workflows/ci.yml", Line: line}) {
		t.Fatalf("locations = %#v", result.Locations)
	}
	assertCheckVersion(t, result.Used, usedTag, usedSHA)
	assertCheckVersion(t, result.Major, majorTag, majorSHA)
	assertCheckVersion(t, result.Latest, latestTag, latestSHA)
}

func assertCheckVersion(t *testing.T, version actions.CheckVersion, tag, sha string) {
	t.Helper()
	if tag == "" {
		if version.Tag != nil {
			t.Fatalf("tag = %q, want nil", *version.Tag)
		}
	} else if version.Tag == nil || *version.Tag != tag {
		t.Fatalf("tag = %v, want %q", version.Tag, tag)
	}
	if sha == "" {
		if version.SHA != nil {
			t.Fatalf("SHA = %q, want nil", *version.SHA)
		}
	} else if version.SHA == nil || *version.SHA != sha {
		t.Fatalf("SHA = %v, want %q", version.SHA, sha)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestCheckPropagatesTextOutputErrors(t *testing.T) {
	check := func(context.Context, string) (actions.CheckReport, error) {
		return actions.CheckReport{
			WorkflowFiles: 1,
			Uses:          1,
			Results:       []actions.CheckResult{{Action: "owner/action"}},
		}, nil
	}
	command := commandForTest(newCheckCommandWithCheck(check), failingWriter{}, &bytes.Buffer{}, "-C", t.TempDir())

	if err := command.Execute(); err == nil {
		t.Fatal("expected an error")
	}
}

func TestCheckReportsNoWorkflowFiles(t *testing.T) {
	check := func(context.Context, string) (actions.CheckReport, error) {
		return actions.CheckReport{}, nil
	}
	var stdout bytes.Buffer
	command := commandForTest(newCheckCommandWithCheck(check), &stdout, &bytes.Buffer{}, "-C", t.TempDir())

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "No workflow files found in .github/workflows\n" {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}

func TestRenderCheckResultsStylesHumanOutput(t *testing.T) {
	sha := "0123456789abcdef0123456789abcdef01234567"
	output := renderCheckResults([]actions.CheckResult{
		{
			Action: "actions/checkout",
			Used:   actions.CheckVersion{SHA: &sha},
			Major:  actions.CheckVersion{Tag: testStringPointer("v4"), SHA: &sha},
			Latest: actions.CheckVersion{Tag: testStringPointer("v4.2.2"), SHA: &sha},
			Status: actions.CheckStatusUpToDate,
			Locations: []actions.Location{
				{File: ".github/workflows/test.yml", Line: 12},
				{File: ".github/workflows/release.yml", Line: 34},
			},
		},
		{
			Action: "owner/update",
			Status: actions.CheckStatusUpdateAvailable,
			PolicyViolations: []actions.PolicyViolation{
				actions.PolicyViolationUnpinned,
				actions.PolicyViolationDisallowedOwner,
			},
		},
		{
			Action:           "owner/unknown",
			Status:           actions.CheckStatusUnknown,
			PolicyViolations: []actions.PolicyViolation{actions.PolicyViolationUnknown},
		},
	}, 0, false)

	for _, text := range []string{
		"GitHub Actions workflow versions",
		"╭",
		"actions/checkout",
		"up to date",
		"update available",
		"unpinned",
		"owner not allowed",
		"unknown",
		"unknown ref rejected",
		".github/workflows/test.yml:12",
		".github/workflows/release.yml:34",
		"v4.2.2",
		"0123456789ab",
	} {
		if !strings.Contains(output, text) {
			t.Errorf("output does not contain %q:\n%s", text, output)
		}
	}
	if strings.Contains(output, sha) {
		t.Errorf("human output contains full SHA:\n%s", output)
	}
	if strings.Contains(output, "\x1b[") {
		t.Errorf("uncolored output contains ANSI escapes: %q", output)
	}
}

func TestRenderCheckResultsUsesColorAndFitsWidth(t *testing.T) {
	output := renderCheckResults([]actions.CheckResult{{
		Action: "actions/checkout",
		Status: actions.CheckStatusUpdateAvailable,
	}}, 80, true)

	if !strings.Contains(output, "\x1b[") {
		t.Fatalf("colored output has no ANSI escapes: %q", output)
	}
	plainOutput := renderCheckResults([]actions.CheckResult{{Action: "actions/checkout"}}, 80, false)
	if !strings.Contains(plainOutput, "│ Action") {
		t.Fatalf("narrow output did not switch to cards: %q", plainOutput)
	}
	for _, line := range strings.Split(output, "\n") {
		if width := lipgloss.Width(line); width > 80 {
			t.Errorf("line width = %d, want at most 80: %q", width, line)
		}
	}
}

func TestFormatCheckSHA(t *testing.T) {
	for _, test := range []struct {
		sha  string
		want string
	}{
		{sha: "short", want: "short"},
		{sha: "0123456789abcdef0123456789abcdef01234567", want: "0123456789ab"},
	} {
		if got := formatCheckSHA(test.sha); got != test.want {
			t.Errorf("formatCheckSHA(%q) = %q, want %q", test.sha, got, test.want)
		}
	}
}

func testStringPointer(value string) *string {
	return &value
}
