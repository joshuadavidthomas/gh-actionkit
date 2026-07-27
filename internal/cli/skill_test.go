package cli

import (
	"bytes"
	"testing"
)

func TestSkillPrintsBundledContentExactly(t *testing.T) {
	content := []byte("---\nname: gh-actionkit\n---\n")
	var stdout bytes.Buffer
	command := commandForTest(newSkillCommand(content), &stdout, &bytes.Buffer{})

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stdout.Bytes(), content) {
		t.Fatalf("skill output = %q, want %q", stdout.Bytes(), content)
	}
}
