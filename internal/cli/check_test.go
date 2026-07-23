package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
)

func TestCheckWritesJSONBeforeReturningFindingStatus(t *testing.T) {
	tag := "v3"
	check := func(context.Context, string) (actions.CheckReport, error) {
		return actions.CheckReport{
			WorkflowFiles: 1,
			Uses:          1,
			Results: []actions.CheckResult{{
				Action:          "actions/checkout",
				Used:            actions.CheckVersion{Tag: &tag},
				UpdateAvailable: true,
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
	if !strings.Contains(stdout.String(), `"update_available": true`) {
		t.Fatalf("unexpected JSON: %q", stdout.String())
	}
	if strings.Contains(stdout.String(), `"short"`) || !strings.Contains(stdout.String(), `"major"`) {
		t.Fatalf("unexpected version fields: %q", stdout.String())
	}
}

func TestCheckEmptyJSONIsAnArray(t *testing.T) {
	check := func(context.Context, string) (actions.CheckReport, error) {
		return actions.CheckReport{Results: []actions.CheckResult{}}, nil
	}
	var stdout bytes.Buffer
	command := commandForTest(newCheckCommandWithCheck(check), &stdout, &bytes.Buffer{}, "-C", t.TempDir(), "--json")

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "[]\n" {
		t.Fatalf("unexpected JSON: %q", stdout.String())
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
			Action:   "actions/checkout",
			Used:     actions.CheckVersion{SHA: &sha},
			Major:    actions.CheckVersion{Tag: testStringPointer("v4"), SHA: &sha},
			Latest:   actions.CheckVersion{Tag: testStringPointer("v4.2.2"), SHA: &sha},
			UpToDate: true,
			Locations: []actions.Location{
				{File: ".github/workflows/test.yml", Line: 12},
				{File: ".github/workflows/release.yml", Line: 34},
			},
		},
		{Action: "owner/update", UpdateAvailable: true},
		{Action: "owner/unknown"},
	}, 0, false)

	for _, text := range []string{
		"GitHub Actions workflow versions",
		"╭",
		"actions/checkout",
		"up to date",
		"update available",
		"unknown",
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
		Action:          "actions/checkout",
		UpdateAvailable: true,
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
