package githubapi

import (
	"context"
	"testing"
	"time"

	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
)

func parseActionIdentifier(t testing.TB, value string) actions.ActionIdentifier {
	t.Helper()
	identifier, err := actions.ParseActionIdentifier(value)
	if err != nil {
		t.Fatal(err)
	}
	return identifier
}

func TestInspectActionMapsMetadataAndPrefersActionYML(t *testing.T) {
	graphQL := &fakeGraphQLClient{response: `{
		"repository": {
			"nameWithOwner": "actions/checkout",
			"description": "Checkout a Git repository",
			"url": "https://github.com/actions/checkout",
			"isArchived": true,
			"pushedAt": "2025-03-13T12:30:00Z",
			"owner": {"login": "actions", "__typename": "Organization"},
			"licenseInfo": {"name": "MIT License", "spdxId": "MIT"},
			"actionYml": {"__typename": "Blob", "text": "name: Checkout\nruns:\n  using: node20\n"},
			"actionYaml": {"__typename": "Blob", "text": "name: Wrong\nruns:\n  using: docker\n"}
		}
	}`}
	client := &Client{graphQL: graphQL}

	inspection, err := client.InspectAction(
		context.Background(),
		parseActionIdentifier(t, "Actions/Checkout/sub/action"),
		"0123456789abcdef0123456789abcdef01234567",
	)
	if err != nil {
		t.Fatal(err)
	}
	if graphQL.query != inspectRepositoryQuery || graphQL.variables["owner"] != "Actions" ||
		graphQL.variables["name"] != "Checkout" ||
		graphQL.variables["actionYml"] != "0123456789abcdef0123456789abcdef01234567:sub/action/action.yml" ||
		graphQL.variables["actionYaml"] != "0123456789abcdef0123456789abcdef01234567:sub/action/action.yaml" {
		t.Fatalf("query=%q variables=%#v", graphQL.query, graphQL.variables)
	}
	if inspection.Repository != (actions.Repository{Owner: "actions", Name: "checkout"}) ||
		inspection.Details.Description == nil ||
		*inspection.Details.Description != "Checkout a Git repository" ||
		inspection.Details.URL != "https://github.com/actions/checkout" ||
		!inspection.Details.Archived {
		t.Fatalf("unexpected repository: %#v", inspection)
	}
	if inspection.Details.Owner.Login != "actions" || inspection.Details.Owner.Type != "Organization" ||
		inspection.Details.License == nil || inspection.Details.License.SPDXID != "MIT" {
		t.Fatalf("unexpected owner or license: %#v", inspection)
	}
	wantTime := time.Date(2025, time.March, 13, 12, 30, 0, 0, time.UTC)
	if inspection.Details.PushedAt == nil || !inspection.Details.PushedAt.Equal(wantTime) {
		t.Fatalf("unexpected pushed time: %#v", inspection.Details.PushedAt)
	}
	if inspection.Manifest == nil || inspection.Manifest.Path != "sub/action/action.yml" ||
		inspection.Manifest.Content != "name: Checkout\nruns:\n  using: node20\n" {
		t.Fatalf("unexpected manifest: %#v", inspection.Manifest)
	}
}

func TestInspectActionFallsBackToActionYAML(t *testing.T) {
	graphQL := &fakeGraphQLClient{response: `{
		"repository": {
			"nameWithOwner": "owner/action",
			"description": null,
			"url": "https://github.com/owner/action",
			"isArchived": false,
			"pushedAt": null,
			"owner": {"login": "owner", "__typename": "User"},
			"licenseInfo": null,
			"actionYml": null,
			"actionYaml": {"__typename": "Blob", "text": "name: Test\nruns:\n  using: composite\n"}
		}
	}`}
	client := &Client{graphQL: graphQL}

	inspection, err := client.InspectAction(
		context.Background(),
		parseActionIdentifier(t, "owner/action"),
		"v1.2.3",
	)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Manifest == nil || inspection.Manifest.Path != "action.yaml" ||
		inspection.Details.PushedAt != nil || inspection.Details.License != nil ||
		inspection.Details.Description != nil {
		t.Fatalf("unexpected inspection: %#v", inspection)
	}
}

func TestInspectActionAllowsMissingManifest(t *testing.T) {
	graphQL := &fakeGraphQLClient{response: `{
		"repository": {
			"nameWithOwner": "owner/repository",
			"url": "https://github.com/owner/repository",
			"owner": {"login": "owner", "__typename": "User"},
			"actionYml": null,
			"actionYaml": null
		}
	}`}
	client := &Client{graphQL: graphQL}

	inspection, err := client.InspectAction(
		context.Background(),
		parseActionIdentifier(t, "owner/repository/path"),
		"HEAD",
	)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Manifest != nil {
		t.Fatalf("unexpected manifest: %#v", inspection.Manifest)
	}
}

func TestInspectActionRejectsUnavailableManifestContent(t *testing.T) {
	tests := []struct {
		name string
		blob string
		want string
	}{
		{
			name: "truncated",
			blob: `{"__typename":"Blob","text":"name: Test","isTruncated":true}`,
			want: "subpath/action.yml content is truncated",
		},
		{
			name: "unavailable",
			blob: `{"__typename":"Blob","text":null,"isTruncated":false}`,
			want: "subpath/action.yml content is unavailable",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			graphQL := &fakeGraphQLClient{response: `{
				"repository": {
					"nameWithOwner": "owner/action",
					"owner": {"login": "owner", "__typename": "User"},
					"actionYml": ` + test.blob + `,
					"actionYaml": null
				}
			}`}
			client := &Client{graphQL: graphQL}

			_, err := client.InspectAction(
				context.Background(),
				parseActionIdentifier(t, "owner/action/subpath"),
				"HEAD",
			)
			if err == nil || err.Error() != test.want {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestInspectActionRejectsUnavailableActionYAMLContent(t *testing.T) {
	graphQL := &fakeGraphQLClient{response: `{
		"repository": {
			"nameWithOwner": "owner/action",
			"owner": {"login": "owner", "__typename": "User"},
			"actionYml": null,
			"actionYaml": {"__typename":"Blob","text":"name: Test","isTruncated":true}
		}
	}`}
	client := &Client{graphQL: graphQL}

	_, err := client.InspectAction(
		context.Background(),
		parseActionIdentifier(t, "owner/action/subpath"),
		"HEAD",
	)
	if err == nil || err.Error() != "subpath/action.yaml content is truncated" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInspectActionRejectsMalformedCanonicalRepository(t *testing.T) {
	graphQL := &fakeGraphQLClient{response: `{
		"repository": {
			"nameWithOwner": "owner/repository/unexpected",
			"owner": {"login": "owner", "__typename": "User"},
			"actionYml": null,
			"actionYaml": null
		}
	}`}
	client := &Client{graphQL: graphQL}

	_, err := client.InspectAction(
		context.Background(),
		parseActionIdentifier(t, "owner/repository/subpath"),
		"HEAD",
	)
	want := `parse GitHub repository identity: invalid repository "owner/repository/unexpected": expected owner/repo`
	if err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
}

func TestInspectActionReportsMissingRepository(t *testing.T) {
	graphQL := &fakeGraphQLClient{response: `{"repository": null}`}
	client := &Client{graphQL: graphQL}

	_, err := client.InspectAction(
		context.Background(),
		parseActionIdentifier(t, "owner/missing/subpath"),
		"HEAD",
	)
	if err == nil || err.Error() != "repository owner/missing was not found" {
		t.Fatalf("unexpected error: %v", err)
	}
}
