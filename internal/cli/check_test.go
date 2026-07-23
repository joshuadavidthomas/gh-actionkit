package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

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
	command := NewRootCommand(&stdout, &bytes.Buffer{}, Dependencies{CheckActions: check})
	command.SetArgs([]string{"check", "-C", t.TempDir(), "--json"})

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
	command := NewRootCommand(&stdout, &bytes.Buffer{}, Dependencies{CheckActions: check})
	command.SetArgs([]string{"check", "-C", t.TempDir(), "--json"})

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
	command := NewRootCommand(failingWriter{}, &bytes.Buffer{}, Dependencies{CheckActions: check})
	command.SetArgs([]string{"check", "-C", t.TempDir()})

	if err := command.Execute(); err == nil {
		t.Fatal("expected an error")
	}
}

func TestCheckReportsNoWorkflowFiles(t *testing.T) {
	check := func(context.Context, string) (actions.CheckReport, error) {
		return actions.CheckReport{}, nil
	}
	var stdout bytes.Buffer
	command := NewRootCommand(&stdout, &bytes.Buffer{}, Dependencies{CheckActions: check})
	command.SetArgs([]string{"check", "-C", t.TempDir()})

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if stdout.String() != "No workflow files found in .github/workflows\n" {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}
