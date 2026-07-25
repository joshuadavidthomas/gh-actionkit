package githubapi

import (
	"context"
	"testing"
	"time"

	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
)

func TestInspectRepositoryMapsMetadataAndPrefersActionYML(t *testing.T) {
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

	inspection, err := client.InspectRepository(
		context.Background(),
		actions.Repository{Owner: "Actions", Name: "Checkout"},
		"0123456789abcdef0123456789abcdef01234567",
	)
	if err != nil {
		t.Fatal(err)
	}
	if graphQL.query != inspectRepositoryQuery || graphQL.variables["owner"] != "Actions" ||
		graphQL.variables["name"] != "Checkout" ||
		graphQL.variables["actionYml"] != "0123456789abcdef0123456789abcdef01234567:action.yml" ||
		graphQL.variables["actionYaml"] != "0123456789abcdef0123456789abcdef01234567:action.yaml" {
		t.Fatalf("query=%q variables=%#v", graphQL.query, graphQL.variables)
	}
	if inspection.Action != "actions/checkout" || inspection.Repository.Description == nil ||
		*inspection.Repository.Description != "Checkout a Git repository" ||
		inspection.Repository.URL != "https://github.com/actions/checkout" ||
		!inspection.Repository.Archived {
		t.Fatalf("unexpected repository: %#v", inspection)
	}
	if inspection.Repository.Owner.Login != "actions" || inspection.Repository.Owner.Type != "Organization" ||
		inspection.Repository.License == nil || inspection.Repository.License.SPDXID != "MIT" {
		t.Fatalf("unexpected owner or license: %#v", inspection)
	}
	wantTime := time.Date(2025, time.March, 13, 12, 30, 0, 0, time.UTC)
	if inspection.Repository.PushedAt == nil || !inspection.Repository.PushedAt.Equal(wantTime) {
		t.Fatalf("unexpected pushed time: %#v", inspection.Repository.PushedAt)
	}
	if inspection.Manifest == nil || inspection.Manifest.Path != "action.yml" ||
		inspection.Manifest.Content != "name: Checkout\nruns:\n  using: node20\n" {
		t.Fatalf("unexpected manifest: %#v", inspection.Manifest)
	}
}

func TestInspectRepositoryFallsBackToActionYAML(t *testing.T) {
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

	inspection, err := client.InspectRepository(
		context.Background(),
		actions.Repository{Owner: "owner", Name: "action"},
		"v1.2.3",
	)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Manifest == nil || inspection.Manifest.Path != "action.yaml" ||
		inspection.Repository.PushedAt != nil || inspection.Repository.License != nil ||
		inspection.Repository.Description != nil {
		t.Fatalf("unexpected inspection: %#v", inspection)
	}
}

func TestInspectRepositoryAllowsMissingManifest(t *testing.T) {
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

	inspection, err := client.InspectRepository(
		context.Background(),
		actions.Repository{Owner: "owner", Name: "repository"},
		"HEAD",
	)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Manifest != nil {
		t.Fatalf("unexpected manifest: %#v", inspection.Manifest)
	}
}

func TestInspectRepositoryRejectsUnavailableManifestContent(t *testing.T) {
	tests := []struct {
		name string
		blob string
		want string
	}{
		{
			name: "truncated",
			blob: `{"__typename":"Blob","text":"name: Test","isTruncated":true}`,
			want: "action.yml content is truncated",
		},
		{
			name: "unavailable",
			blob: `{"__typename":"Blob","text":null,"isTruncated":false}`,
			want: "action.yml content is unavailable",
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

			_, err := client.InspectRepository(
				context.Background(),
				actions.Repository{Owner: "owner", Name: "action"},
				"HEAD",
			)
			if err == nil || err.Error() != test.want {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestInspectRepositoryRejectsUnavailableActionYAMLContent(t *testing.T) {
	graphQL := &fakeGraphQLClient{response: `{
		"repository": {
			"nameWithOwner": "owner/action",
			"owner": {"login": "owner", "__typename": "User"},
			"actionYml": null,
			"actionYaml": {"__typename":"Blob","text":"name: Test","isTruncated":true}
		}
	}`}
	client := &Client{graphQL: graphQL}

	_, err := client.InspectRepository(
		context.Background(),
		actions.Repository{Owner: "owner", Name: "action"},
		"HEAD",
	)
	if err == nil || err.Error() != "action.yaml content is truncated" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInspectRepositoryReportsMissingRepository(t *testing.T) {
	graphQL := &fakeGraphQLClient{response: `{"repository": null}`}
	client := &Client{graphQL: graphQL}

	_, err := client.InspectRepository(
		context.Background(),
		actions.Repository{Owner: "owner", Name: "missing"},
		"HEAD",
	)
	if err == nil || err.Error() != "repository owner/missing was not found" {
		t.Fatalf("unexpected error: %v", err)
	}
}
