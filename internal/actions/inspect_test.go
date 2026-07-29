package actions

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeInspectSource struct {
	fakeVersionSource
	inspection       RepositoryInspection
	inspectErr       error
	inspectedRefs    *[]string
	inspectedActions *[]ActionIdentifier
}

func (f fakeInspectSource) InspectAction(
	_ context.Context,
	identifier ActionIdentifier,
	ref string,
) (RepositoryInspection, error) {
	if f.inspectedActions != nil {
		*f.inspectedActions = append(*f.inspectedActions, identifier)
	}
	if f.inspectedRefs != nil {
		*f.inspectedRefs = append(*f.inspectedRefs, ref)
	}
	return f.inspection, f.inspectErr
}

func inspectForTest(
	t testing.TB,
	source InspectSource,
	value string,
) (InspectResult, error) {
	t.Helper()
	return NewInspectService(source).Inspect(context.Background(), mustParseIdentifier(t, value))
}

func TestInspectReturnsRepositoryManifestAndPinnedVersion(t *testing.T) {
	pushedAt := time.Date(2025, time.March, 13, 12, 30, 0, 0, time.UTC)
	description := "Checkout a Git repository"
	license := &RepositoryLicense{Name: "MIT License", SPDXID: "MIT"}
	latestSHA := "0123456789abcdef0123456789abcdef01234567"
	var inspectedRefs []string
	source := fakeInspectSource{
		fakeVersionSource: fakeVersionSource{
			release:      "v4.2.2",
			releaseFound: true,
			refs:         map[string]string{"v4.2.2": latestSHA},
		},
		inspectedRefs: &inspectedRefs,
		inspection: RepositoryInspection{
			Repository: Repository{Owner: "actions", Name: "checkout"},
			Details: RepositoryDetails{
				Description: &description,
				URL:         "https://github.com/actions/checkout",
				Owner:       RepositoryOwner{Login: "actions", Type: "Organization"},
				PushedAt:    &pushedAt,
				License:     license,
			},
			Manifest: &ManifestFile{Path: "action.yml", Content: `
name: Checkout
description: Checkout a Git repository
inputs:
  token:
    description: GitHub token
    required: true
  fetch-depth:
    description: Number of commits
    default: 1
  clean:
    default: false
  empty:
    default: ""
outputs:
  z-ref:
    description: Checked out ref
    value: ${{ steps.checkout.outputs.ref }}
  a-sha:
    description: Checked out SHA
runs:
  using: node20
  main: dist/index.js
`},
		},
	}

	result, err := inspectForTest(t, source, "Actions/Checkout")
	if err != nil {
		t.Fatal(err)
	}
	if len(inspectedRefs) != 1 || inspectedRefs[0] != latestSHA {
		t.Fatalf("inspected refs = %v, want latest SHA", inspectedRefs)
	}
	if result.Action != "actions/checkout" || result.Repository.Owner.Login != "actions" || result.Repository.Owner.Type != "Organization" {
		t.Fatalf("unexpected repository identity: %#v", result)
	}
	if result.Repository.Description == nil || *result.Repository.Description != description ||
		result.Repository.URL != "https://github.com/actions/checkout" || result.Repository.PushedAt == nil ||
		!result.Repository.PushedAt.Equal(pushedAt) || result.Repository.License == nil ||
		result.Repository.License.SPDXID != "MIT" {
		t.Fatalf("unexpected repository metadata: %#v", result)
	}
	if result.Manifest.Path != "action.yml" || result.Manifest.Ref != latestSHA ||
		result.Manifest.Name != "Checkout" || result.Manifest.Runtime != "node20" {
		t.Fatalf("unexpected manifest: %#v", result.Manifest)
	}
	if len(result.Manifest.Inputs) != 4 || result.Manifest.Inputs[0].Name != "clean" ||
		result.Manifest.Inputs[1].Name != "empty" || result.Manifest.Inputs[2].Name != "fetch-depth" ||
		result.Manifest.Inputs[3].Name != "token" {
		t.Fatalf("inputs are not sorted: %#v", result.Manifest.Inputs)
	}
	if result.Manifest.Inputs[0].Default == nil || *result.Manifest.Inputs[0].Default != "false" ||
		result.Manifest.Inputs[1].Default == nil || *result.Manifest.Inputs[1].Default != "" ||
		result.Manifest.Inputs[2].Default == nil || *result.Manifest.Inputs[2].Default != "1" ||
		!result.Manifest.Inputs[3].Required || result.Manifest.Inputs[3].Default != nil {
		t.Fatalf("unexpected input values: %#v", result.Manifest.Inputs)
	}
	if len(result.Manifest.Outputs) != 2 || result.Manifest.Outputs[0].Name != "a-sha" ||
		result.Manifest.Outputs[1].Name != "z-ref" || result.Manifest.Outputs[1].Value == nil {
		t.Fatalf("outputs are not sorted: %#v", result.Manifest.Outputs)
	}
	if result.Latest == nil || result.Latest.Tag != "v4.2.2" || result.Latest.SHA == nil ||
		*result.Latest.SHA != latestSHA {
		t.Fatalf("unexpected latest version: %#v", result.Latest)
	}
	wantPinned := "uses: actions/checkout@" + latestSHA + " # v4.2.2"
	if result.PinnedUses == nil || *result.PinnedUses != wantPinned {
		t.Fatalf("pinned uses = %#v, want %q", result.PinnedUses, wantPinned)
	}
}

func TestInspectPreservesSubpathInInspectionAndPin(t *testing.T) {
	latestSHA := "0123456789abcdef0123456789abcdef01234567"
	var inspectedActions []ActionIdentifier
	source := fakeInspectSource{
		fakeVersionSource: fakeVersionSource{
			release:      "v3.0.0",
			releaseFound: true,
			refs:         map[string]string{"v3.0.0": latestSHA},
		},
		inspectedActions: &inspectedActions,
		inspection: RepositoryInspection{
			Repository: Repository{Owner: "github", Name: "codeql-action"},
			Manifest:   &ManifestFile{Path: "init/action.yml", Content: "name: Init\nruns:\n  using: node20\n"},
		},
	}

	result, err := inspectForTest(t, source, "GitHub/codeql-action/init")
	if err != nil {
		t.Fatal(err)
	}
	if len(inspectedActions) != 1 || inspectedActions[0].String() != "GitHub/codeql-action/init" {
		t.Fatalf("inspected actions = %#v", inspectedActions)
	}
	if result.Action != "github/codeql-action/init" || result.Manifest.Path != "init/action.yml" {
		t.Fatalf("unexpected result: %#v", result)
	}
	wantPinned := "uses: github/codeql-action/init@" + latestSHA + " # v3.0.0"
	if result.PinnedUses == nil || *result.PinnedUses != wantPinned {
		t.Fatalf("pinned uses = %#v, want %q", result.PinnedUses, wantPinned)
	}
}

func TestInspectWithoutVersionsStillReturnsManifest(t *testing.T) {
	source := fakeInspectSource{inspection: RepositoryInspection{
		Repository: Repository{Owner: "owner", Name: "action"},
		Manifest:   &ManifestFile{Path: "action.yaml", Content: "name: Test\nruns:\n  using: composite\n"},
	}}

	result, err := inspectForTest(t, source, "owner/action")
	if err != nil {
		t.Fatal(err)
	}
	if result.Latest != nil || result.PinnedUses != nil || result.Manifest.Runtime != "composite" ||
		result.Manifest.Ref != "HEAD" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Manifest.Inputs == nil || result.Manifest.Outputs == nil {
		t.Fatalf("empty manifest collections must be non-nil: %#v", result.Manifest)
	}
}

func TestInspectLeavesPinnedUsesUnknownForMalformedSHA(t *testing.T) {
	var inspectedRefs []string
	source := fakeInspectSource{
		fakeVersionSource: fakeVersionSource{
			release:      "v1.0.0",
			releaseFound: true,
			refs:         map[string]string{"v1.0.0": "short"},
		},
		inspectedRefs: &inspectedRefs,
		inspection: RepositoryInspection{
			Repository: Repository{Owner: "owner", Name: "action"},
			Manifest:   &ManifestFile{Path: "action.yml", Content: "name: Test\nruns:\n  using: docker\n"},
		},
	}

	result, err := inspectForTest(t, source, "owner/action")
	if err != nil {
		t.Fatal(err)
	}
	if len(inspectedRefs) != 1 || inspectedRefs[0] != "v1.0.0" || result.Manifest.Ref != "v1.0.0" {
		t.Fatalf("inspected refs = %v, manifest ref = %q", inspectedRefs, result.Manifest.Ref)
	}
	if result.Latest == nil || result.PinnedUses != nil || result.Manifest.Runtime != "docker" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestInspectAcceptsScalarAliasDefaults(t *testing.T) {
	source := fakeInspectSource{inspection: RepositoryInspection{
		Repository: Repository{Owner: "owner", Name: "action"},
		Manifest: &ManifestFile{Path: "action.yml", Content: `
name: Test
defaults:
  value: &default-value enabled
inputs:
  mode:
    default: *default-value
runs:
  using: composite
`},
	}}

	result, err := inspectForTest(t, source, "owner/action")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Manifest.Inputs) != 1 || result.Manifest.Inputs[0].Default == nil ||
		*result.Manifest.Inputs[0].Default != "enabled" {
		t.Fatalf("unexpected inputs: %#v", result.Manifest.Inputs)
	}
}

func TestInspectReportsMissingManifest(t *testing.T) {
	source := fakeInspectSource{inspection: RepositoryInspection{
		Repository: Repository{Owner: "canonical-owner", Name: "canonical-action"},
	}}

	_, err := inspectForTest(t, source, "owner/action/subpath")
	if !errors.Is(err, ErrNoActionManifest) {
		t.Fatalf("expected missing manifest error, got %v", err)
	}
	want := "inspect canonical-owner/canonical-action/subpath at HEAD: subpath/action.yml or subpath/action.yaml: action manifest not found"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err, want)
	}
}

func TestInspectReportsManifestErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "invalid YAML",
			content: "name: [",
			want:    "parse subpath/action.yml for owner/action/subpath",
		},
		{
			name:    "missing runtime",
			content: "name: Test\n",
			want:    "parse subpath/action.yml for owner/action/subpath: runs.using is required",
		},
		{
			name:    "structured default",
			content: "name: Test\ninputs:\n  options:\n    default: [one, two]\nruns:\n  using: composite\n",
			want:    "parse subpath/action.yml for owner/action/subpath: input options default: must be a scalar value",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			source := fakeInspectSource{inspection: RepositoryInspection{
				Repository: Repository{Owner: "owner", Name: "action"},
				Manifest:   &ManifestFile{Path: "subpath/action.yml", Content: test.content},
			}}

			_, err := inspectForTest(t, source, "Owner/Action/subpath")
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestInspectStopsWhenVersionLookupFails(t *testing.T) {
	sourceErr := errors.New("rate limited")
	var inspectedRefs []string
	source := fakeInspectSource{
		fakeVersionSource: fakeVersionSource{err: sourceErr},
		inspectedRefs:     &inspectedRefs,
	}

	_, err := inspectForTest(t, source, "owner/action")
	if !errors.Is(err, sourceErr) {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(inspectedRefs) != 0 {
		t.Fatalf("repository inspection should not run after version error: %v", inspectedRefs)
	}
}

func TestInspectReportsSourceErrors(t *testing.T) {
	sourceErr := errors.New("rate limited")
	source := fakeInspectSource{inspectErr: sourceErr}

	_, err := inspectForTest(t, source, "owner/action")
	if !errors.Is(err, sourceErr) || !strings.Contains(err.Error(), "inspect action owner/action") {
		t.Fatalf("unexpected error: %v", err)
	}
}
