package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestWriteTerminalErrorSanitizesControlCharacters(t *testing.T) {
	t.Parallel()

	err := errors.New("parse input \x1b]8;;https://evil.example\x07name\x1b]8;;\x07\nfailed")
	var output bytes.Buffer
	writeTerminalError(&output, err)

	if strings.ContainsRune(output.String(), '\x1b') {
		t.Fatalf("terminal error contains untrusted ESC: %q", output.String())
	}
	want := "parse input �]8;;https://evil.example�name�]8;;�\\nfailed\n"
	if output.String() != want {
		t.Fatalf("terminal error = %q; want %q", output.String(), want)
	}
}
