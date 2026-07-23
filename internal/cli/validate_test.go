package cli

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestValidateReturnsFindingStatus(t *testing.T) {
	validate := func(_ string, outputJSON bool, _, _ io.Writer) (int, int, error) {
		if !outputJSON {
			t.Fatal("expected JSON output")
		}
		return 2, 3, nil
	}
	command := NewRootCommand(&bytes.Buffer{}, &bytes.Buffer{}, Dependencies{ValidateWorkflows: validate})
	command.SetArgs([]string{"validate", "-C", t.TempDir(), "--json"})

	err := command.Execute()
	var statusError StatusError
	if !errors.As(err, &statusError) || statusError.Code != 1 {
		t.Fatalf("expected status 1, got %v", err)
	}
}

func TestValidateReportsNoWorkflowsWithoutPollutingJSON(t *testing.T) {
	validate := func(_ string, _ bool, _, _ io.Writer) (int, int, error) {
		return 0, 0, nil
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := NewRootCommand(&stdout, &stderr, Dependencies{ValidateWorkflows: validate})
	command.SetArgs([]string{"validate", "-C", t.TempDir(), "--json"})

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
