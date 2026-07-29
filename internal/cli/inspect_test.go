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
	inspect := func(_ context.Context, identifier actions.ActionIdentifier) (actions.InspectResult, error) {
		if identifier.String() != "github/codeql-action/init" ||
			identifier.Repository() != (actions.Repository{Owner: "github", Name: "codeql-action"}) ||
			identifier.Path() != "init" {
			t.Fatalf("identifier = %#v", identifier)
		}
		return actions.InspectResult{
			Action: "github/codeql-action/init",
			Repository: actions.RepositoryDetails{
				Owner:    actions.RepositoryOwner{Login: "github", Type: "Organization"},
				PushedAt: &pushedAt,
				License:  &actions.RepositoryLicense{Name: "MIT License", SPDXID: "MIT"},
			},
			Manifest: actions.ActionManifest{
				Path:    "init/action.yml",
				Runtime: "node20",
				Inputs:  []actions.ManifestInput{},
				Outputs: []actions.ManifestOutput{},
			},
		}, nil
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command := commandForTest(
		newInspectCommand(inspect),
		&stdout,
		&stderr,
		"github/codeql-action/init",
		"--json",
	)

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	var result actions.InspectResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON %q: %v", stdout.String(), err)
	}
	if result.Action != "github/codeql-action/init" || result.Manifest.Path != "init/action.yml" ||
		result.Manifest.Inputs == nil || result.Manifest.Outputs == nil || result.Latest != nil ||
		result.PinnedUses != nil || result.Repository.Owner.Login != "github" || result.Repository.License == nil {
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

func TestInspectRejectsMalformedIdentifierBeforeLookup(t *testing.T) {
	called := false
	inspect := func(context.Context, actions.ActionIdentifier) (actions.InspectResult, error) {
		called = true
		return actions.InspectResult{}, nil
	}
	command := commandForTest(
		newInspectCommand(inspect),
		&bytes.Buffer{},
		&bytes.Buffer{},
		"owner/repo/",
	)

	if err := command.Execute(); err == nil {
		t.Fatal("expected an error")
	}
	if called {
		t.Fatal("inspect ran for malformed identifier")
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
	inspect := func(context.Context, actions.ActionIdentifier) (actions.InspectResult, error) {
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
		newInspectCommand(inspect),
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

func TestInspectSanitizesHumanOutputAndPreservesJSON(t *testing.T) {
	description := "link \x1b]8;;https://evil.example\x07click\x1b]8;;\x07"
	defaultValue := "evil\x1b[2Jtext"
	inspect := func(context.Context, actions.ActionIdentifier) (actions.InspectResult, error) {
		return actions.InspectResult{
			Action: "owner/evil\x1b[2Jaction",
			Repository: actions.RepositoryDetails{
				Owner:       actions.RepositoryOwner{Login: "owner"},
				Description: &description,
			},
			Manifest: actions.ActionManifest{
				Name:        "Evil",
				Description: description,
				Inputs: []actions.ManifestInput{{
					Name:        "payload",
					Description: &description,
					Default:     &defaultValue,
				}},
				Outputs: []actions.ManifestOutput{},
			},
		}, nil
	}

	var humanOutput bytes.Buffer
	humanCommand := commandForTest(
		newInspectCommand(inspect),
		&humanOutput,
		&bytes.Buffer{},
		"owner/evil",
	)
	if err := humanCommand.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsRune(humanOutput.String(), '\x1b') {
		t.Fatalf("human output contains untrusted ESC: %q", humanOutput.String())
	}
	for _, text := range []string{"owner/evil�[2Jaction", "evil�[2Jtext", "link �]8;;https://evil.example�click�]8;;�"} {
		if !strings.Contains(humanOutput.String(), text) {
			t.Errorf("human output does not contain %q: %q", text, humanOutput.String())
		}
	}

	var jsonOutput bytes.Buffer
	jsonCommand := commandForTest(
		newInspectCommand(inspect),
		&jsonOutput,
		&bytes.Buffer{},
		"owner/evil",
		"--json",
	)
	if err := jsonCommand.Execute(); err != nil {
		t.Fatal(err)
	}
	var result actions.InspectResult
	if err := json.Unmarshal(jsonOutput.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON %q: %v", jsonOutput.String(), err)
	}
	if result.Action != "owner/evil\x1b[2Jaction" || result.Manifest.Description != description {
		t.Fatalf("JSON changed control-character payload: %#v", result)
	}
	if result.Manifest.Inputs[0].Default == nil || *result.Manifest.Inputs[0].Default != defaultValue {
		t.Fatalf("JSON changed default payload: %#v", result.Manifest.Inputs)
	}
}

func TestInspectReportsUnknownOptionalValues(t *testing.T) {
	inspect := func(context.Context, actions.ActionIdentifier) (actions.InspectResult, error) {
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
	command := commandForTest(newInspectCommand(inspect), &stdout, &bytes.Buffer{}, "owner/action")

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
	inspect := func(context.Context, actions.ActionIdentifier) (actions.InspectResult, error) {
		return actions.InspectResult{}, inspectErr
	}
	command := commandForTest(
		newInspectCommand(inspect),
		&bytes.Buffer{},
		&bytes.Buffer{},
		"owner/action",
	)

	if err := command.Execute(); !errors.Is(err, inspectErr) {
		t.Fatalf("expected inspect error, got %v", err)
	}
}

func TestInspectPropagatesOutputErrors(t *testing.T) {
	inspect := func(context.Context, actions.ActionIdentifier) (actions.InspectResult, error) {
		return actions.InspectResult{Action: "owner/action"}, nil
	}
	command := commandForTest(
		newInspectCommand(inspect),
		failingWriter{},
		&bytes.Buffer{},
		"owner/action",
	)

	if err := command.Execute(); err == nil {
		t.Fatal("expected output error")
	}
}
