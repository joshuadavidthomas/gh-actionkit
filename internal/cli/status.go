package cli

// StatusError asks main to return a command-specific process status without
// printing another error after a child tool has already written its output.
type StatusError struct {
	Code int
}

func (e StatusError) Error() string {
	return ""
}

func (e StatusError) ExitCode() int {
	return e.Code
}
