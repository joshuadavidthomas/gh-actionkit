package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestSpinnerWritesAndClearsTransientStatus(t *testing.T) {
	var output bytes.Buffer
	indicator := newSpinner(&output, "Searching GitHub...", true)
	indicator.Stop()

	got := output.String()
	if !strings.Contains(got, "⠋ Searching GitHub...") {
		t.Fatalf("missing spinner status in %q", got)
	}
	if !strings.HasSuffix(got, "\r\x1b[2K") {
		t.Fatalf("spinner was not cleared: %q", got)
	}
}

func TestDisabledSpinnerWritesNothing(t *testing.T) {
	var output bytes.Buffer
	indicator := newSpinner(&output, "Searching GitHub...", false)
	indicator.Stop()

	if output.Len() != 0 {
		t.Fatalf("unexpected output: %q", output.String())
	}
}
