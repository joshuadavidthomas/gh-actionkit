package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestSpinnerWritesAndClearsTransientStatus(t *testing.T) {
	var output bytes.Buffer
	indicator := newSpinner(&output, "Searching GitHub...", true, time.Hour)
	indicator.Stop()
	indicator.Stop()

	got := output.String()
	if !strings.Contains(got, "⠋ Searching GitHub...") {
		t.Fatalf("missing spinner status in %q", got)
	}
	if !strings.HasSuffix(got, "\r\x1b[2K") {
		t.Fatalf("spinner was not cleared: %q", got)
	}
	if count := strings.Count(got, "\r\x1b[2K"); count != 2 {
		t.Fatalf("Stop should clear once, got %d control sequences in %q", count, got)
	}
}

func TestDisabledSpinnerWritesNothing(t *testing.T) {
	var output bytes.Buffer
	indicator := newSpinner(&output, "Searching GitHub...", false, time.Hour)
	indicator.Stop()

	if output.Len() != 0 {
		t.Fatalf("unexpected output: %q", output.String())
	}
}
