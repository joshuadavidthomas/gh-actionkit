package cli

import (
	"errors"
	"fmt"
)

// exitStatusError asks main to preserve a command-specific process status
// without printing another error after the command has written its output.
type exitStatusError int

func (e exitStatusError) Error() string {
	return fmt.Sprintf("command exited with status %d", e)
}

// ExitStatus reports a command-specific status after the command has written its output.
func ExitStatus(err error) (int, bool) {
	var status exitStatusError
	if !errors.As(err, &status) {
		return 0, false
	}
	return int(status), true
}
