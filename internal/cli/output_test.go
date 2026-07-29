package cli

import (
	"strings"
	"testing"
)

func TestSanitizeTerminal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "plain Unicode",
			value: "plain café 中文 🚀",
			want:  "plain café 中文 🚀",
		},
		{
			name:  "multiline whitespace",
			value: "first\n\tsecond",
			want:  "first\n\tsecond",
		},
		{
			name:  "C0 ESC DEL and C1 controls",
			value: "a\x00\x1b\x7f\u0080\u009bb",
			want:  "a�����b",
		},
		{
			name:  "OSC hyperlink",
			value: "\x1b]8;;https://example.com\x07link\x1b]8;;\x07",
			want:  "�]8;;https://example.com�link�]8;;�",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := sanitizeTerminal(test.value); got != test.want {
				t.Fatalf("sanitizeTerminal(%q) = %q; want %q", test.value, got, test.want)
			}
			if strings.ContainsRune(sanitizeTerminal(test.value), '\x1b') {
				t.Fatalf("sanitizeTerminal(%q) retained ESC", test.value)
			}
		})
	}
}

func TestSanitizeTerminalLine(t *testing.T) {
	t.Parallel()

	value := "first\n\tsecond\r\x1b\u009b"
	want := `first\n\tsecond���`
	if got := sanitizeTerminalLine(value); got != want {
		t.Fatalf("sanitizeTerminalLine(%q) = %q; want %q", value, got, want)
	}
}
