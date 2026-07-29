package cli

import (
	"fmt"
	"testing"
)

type unrelatedExitError struct{}

func (unrelatedExitError) Error() string { return "child process failed" }
func (unrelatedExitError) ExitCode() int { return 9 }

func TestExitStatusOnlyRecognizesCommandStatuses(t *testing.T) {
	status, ok := ExitStatus(fmt.Errorf("wrapped: %w", exitStatusError(7)))
	if !ok || status != 7 {
		t.Fatalf("status = %d, found = %t", status, ok)
	}
	if _, ok := ExitStatus(fmt.Errorf("wrapped: %w", unrelatedExitError{})); ok {
		t.Fatal("unrelated process error classified as a command status")
	}
}
