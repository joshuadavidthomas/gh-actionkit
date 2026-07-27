package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/joshuadavidthomas/gh-actionkit/internal/cli"
)

func TestSkillCommandPrintsEmbeddedSkillSource(t *testing.T) {
	want, err := os.ReadFile("skills/gh-actionkit/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := cli.NewRootCommand("test", bundledSkill, &stdout, &stderr)
	command.SetArgs([]string{"skill"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stdout.Bytes(), want) {
		t.Fatal("skill command output does not match skills/gh-actionkit/SKILL.md")
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}
