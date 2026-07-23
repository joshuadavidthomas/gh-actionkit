package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"testing"
)

func TestLintForwardsOptionsAndStatus(t *testing.T) {
	repository := t.TempDir()
	lint := func(
		_ context.Context,
		gotRepository string,
		outputJSON bool,
		pedantic bool,
		_, _ io.Writer,
	) (int, error) {
		wantRepository, err := filepath.Abs(repository)
		if err != nil {
			t.Fatal(err)
		}
		if gotRepository != wantRepository || !outputJSON || !pedantic {
			t.Fatalf("repository=%q json=%v pedantic=%v", gotRepository, outputJSON, pedantic)
		}
		return 13, nil
	}
	command := commandForTest(
		newLintCommandWithLint(lint),
		&bytes.Buffer{},
		&bytes.Buffer{},
		"-C",
		repository,
		"--json",
		"--pedantic",
	)

	err := command.Execute()
	var statusError StatusError
	if !errors.As(err, &statusError) || statusError.Code != 13 {
		t.Fatalf("expected status 13, got %v", err)
	}
}

func TestLintNoPedanticOverridesDefault(t *testing.T) {
	lint := func(_ context.Context, _ string, _ bool, pedantic bool, _, _ io.Writer) (int, error) {
		if pedantic {
			t.Fatal("pedantic should be disabled")
		}
		return 0, nil
	}
	command := commandForTest(
		newLintCommandWithLint(lint),
		&bytes.Buffer{},
		&bytes.Buffer{},
		"-C",
		t.TempDir(),
		"--no-pedantic",
	)

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
}
