package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
)

func TestInspectForwardsActionAndWritesJSON(t *testing.T) {
	pushedAt := time.Date(2025, time.March, 13, 12, 30, 0, 0, time.UTC)
	inspect := func(_ context.Context, action string) (actions.InspectResult, error) {
		if action != "actions/checkout" {
			t.Fatalf("action = %q", action)
		}
		return actions.InspectResult{
			Action: "actions/checkout",
			Repository: actions.RepositoryDetails{
				Owner:    actions.RepositoryOwner{Login: "actions", Type: "Organization"},
				PushedAt: &pushedAt,
				License:  &actions.RepositoryLicense{Name: "MIT License", SPDXID: "MIT"},
			},
			Manifest: actions.ActionManifest{
				Path:    "action.yml",
				Runtime: "node20",
				Inputs:  []actions.ManifestInput{},
				Outputs: []actions.ManifestOutput{},
			},
		}, nil
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := commandForTest(
		newInspectCommandWithInspect(inspect),
		&stdout,
		&stderr,
		"actions/checkout",
		"--json",
	)

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	var result actions.InspectResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON %q: %v", stdout.String(), err)
	}
	if result.Action != "actions/checkout" || result.Manifest.Inputs == nil ||
		result.Manifest.Outputs == nil || result.Latest != nil || result.PinnedUses != nil ||
		result.Repository.Owner.Login != "actions" || result.Repository.License == nil {
		t.Fatalf("unexpected result: %#v", result)
	}
	for _, fragment := range []string{
		`"repository": {`,
		`"owner": {`,
		`"pushed_at": "2025-03-13T12:30:00Z"`,
		`"license": {`,
		`"inputs": []`,
		`"outputs": []`,
		`"latest": null`,
		`"pinned_uses": null`,
	} {
		if !strings.Contains(stdout.String(), fragment) {
			t.Errorf("JSON does not contain %q: %s", fragment, stdout.String())
		}
	}
	if strings.Contains(stdout.String(), `"repository_description"`) {
		t.Fatalf("JSON contains obsolete flat repository field: %s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestInspectWritesHumanOutput(t *testing.T) {
	sha := "0123456789abcdef0123456789abcdef01234567"
	pinnedUses := "uses: actions/checkout@" + sha + " # v4.2.2"
	pushedAt := time.Date(2025, time.March, 13, 12, 30, 0, 0, time.UTC)
	repositoryDescription := "Checkout a Git repository"
	inputDescription := "GitHub token"
	outputDescription := "Checked out SHA"
	defaultValue := "${{ github.token }}"
	multilineDefault := "one\ntwo"
	inspect := func(context.Context, string) (actions.InspectResult, error) {
		return actions.InspectResult{
			Action: "actions/checkout",
			Repository: actions.RepositoryDetails{
				Description: &repositoryDescription,
				URL:         "https://github.com/actions/checkout",
				Owner:       actions.RepositoryOwner{Login: "actions", Type: "Organization"},
				Archived:    true,
				PushedAt:    &pushedAt,
				License:     &actions.RepositoryLicense{Name: "MIT License", SPDXID: "MIT"},
			},
			Manifest: actions.ActionManifest{
				Path:        "action.yml",
				Ref:         sha,
				Name:        "Checkout",
				Description: "Checkout a Git repository",
				Runtime:     "node20",
				Inputs: []actions.ManifestInput{
					{
						Name:        "token",
						Description: &inputDescription,
						Required:    true,
						Default:     &defaultValue,
					},
					{Name: "patterns", Default: &multilineDefault},
				},
				Outputs: []actions.ManifestOutput{{Name: "commit", Description: &outputDescription}},
			},
			Latest:     &actions.Version{Tag: "v4.2.2", SHA: &sha},
			PinnedUses: &pinnedUses,
		}, nil
	}
	var stdout bytes.Buffer
	command := commandForTest(
		newInspectCommandWithInspect(inspect),
		&stdout,
		&bytes.Buffer{},
		"actions/checkout",
	)

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{
		"actions/checkout",
		"actions (Organization)",
		"archived: true",
		"2025-03-13T12:30:00Z",
		"license: MIT",
		"action.yml",
		"ref: " + sha,
		"Checkout",
		"runtime: node20",
		"token (required, default: ${{ github.token }})",
		`patterns (optional, default: one\ntwo)`,
		"GitHub token",
		"commit",
		"Checked out SHA",
		"tag: v4.2.2",
		"sha: " + sha,
		pinnedUses,
	} {
		if !strings.Contains(stdout.String(), text) {
			t.Errorf("output does not contain %q:\n%s", text, stdout.String())
		}
	}
}

func TestInspectReportsUnknownOptionalValues(t *testing.T) {
	inspect := func(context.Context, string) (actions.InspectResult, error) {
		return actions.InspectResult{
			Action:     "owner/action",
			Repository: actions.RepositoryDetails{Owner: actions.RepositoryOwner{Login: "owner"}},
			Manifest: actions.ActionManifest{
				Path:    "action.yaml",
				Name:    "Test",
				Runtime: "composite",
				Inputs:  []actions.ManifestInput{},
				Outputs: []actions.ManifestOutput{},
			},
		}, nil
	}
	var stdout bytes.Buffer
	command := commandForTest(newInspectCommandWithInspect(inspect), &stdout, &bytes.Buffer{}, "owner/action")

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"last push: unknown", "license: unknown", "inputs\n    none", "outputs\n    none", "version: unknown"} {
		if !strings.Contains(stdout.String(), text) {
			t.Errorf("output does not contain %q:\n%s", text, stdout.String())
		}
	}
}

func TestInspectPropagatesErrors(t *testing.T) {
	inspectErr := errors.New("authentication failed")
	inspect := func(context.Context, string) (actions.InspectResult, error) {
		return actions.InspectResult{}, inspectErr
	}
	command := commandForTest(
		newInspectCommandWithInspect(inspect),
		&bytes.Buffer{},
		&bytes.Buffer{},
		"owner/action",
	)

	if err := command.Execute(); !errors.Is(err, inspectErr) {
		t.Fatalf("expected inspect error, got %v", err)
	}
}

func TestInspectPropagatesOutputErrors(t *testing.T) {
	inspect := func(context.Context, string) (actions.InspectResult, error) {
		return actions.InspectResult{Action: "owner/action"}, nil
	}
	command := commandForTest(
		newInspectCommandWithInspect(inspect),
		failingWriter{},
		&bytes.Buffer{},
		"owner/action",
	)

	if err := command.Execute(); err == nil {
		t.Fatal("expected output error")
	}
}
