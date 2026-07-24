package githubapi

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
)

type fakeGraphQLClient struct {
	response      string
	responses     []string
	err           error
	query         string
	variables     map[string]interface{}
	variableCalls []map[string]interface{}
}

func (f *fakeGraphQLClient) DoWithContext(
	_ context.Context,
	query string,
	variables map[string]interface{},
	response interface{},
) error {
	f.query = query
	f.variables = variables
	f.variableCalls = append(f.variableCalls, variables)
	if f.err != nil {
		return f.err
	}
	body := f.response
	if len(f.responses) > 0 {
		call := len(f.variableCalls) - 1
		if call >= len(f.responses) {
			return errors.New("missing GraphQL response")
		}
		body = f.responses[call]
	}
	if body == "" {
		return errors.New("missing GraphQL response")
	}
	return json.Unmarshal([]byte(body), response)
}

func TestSearchRepositoriesFiltersCandidatesByDefaultBranchManifest(t *testing.T) {
	graphQL := &fakeGraphQLClient{response: `{
		"search": {
			"nodes": [
				{
					"nameWithOwner": "docker/build-push-action",
					"description": "Build images",
					"stargazerCount": 7100,
					"url": "https://github.com/docker/build-push-action",
					"actionYml": {"__typename": "Blob"},
					"actionYaml": null
				},
				{
					"nameWithOwner": "owner/yaml-action",
					"description": null,
					"stargazerCount": 20,
					"url": "https://github.com/owner/yaml-action",
					"actionYml": null,
					"actionYaml": {"__typename": "Blob"}
				},
				{
					"nameWithOwner": "owner/not-an-action",
					"description": "No manifest",
					"stargazerCount": 10,
					"url": "https://github.com/owner/not-an-action",
					"actionYml": null,
					"actionYaml": null
				}
			]
		}
	}`}
	client := &Client{graphQL: graphQL}

	results, err := client.SearchRepositories(context.Background(), actions.SearchOptions{
		Query:          "docker & build",
		ResultLimit:    2,
		CandidateLimit: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Action != "docker/build-push-action" ||
		results[1].Action != "owner/yaml-action" {
		t.Fatalf("unexpected results: %#v", results)
	}
	if results[0].Description == nil || *results[0].Description != "Build images" ||
		results[1].Description != nil {
		t.Fatalf("unexpected descriptions: %#v", results)
	}
	if graphQL.query != searchRepositoriesQuery {
		t.Fatalf("unexpected query: %q", graphQL.query)
	}
	if graphQL.variables["query"] != "docker & build action in:name,description sort:stars-desc" ||
		graphQL.variables["limit"] != graphQLSearchPageSize || graphQL.variables["cursor"] != (*string)(nil) {
		t.Fatalf("unexpected variables: %#v", graphQL.variables)
	}
}

func TestSearchRepositoriesPaginatesUntilItFindsEnoughActions(t *testing.T) {
	graphQL := &fakeGraphQLClient{responses: []string{
		`{"search":{"nodes":[{
			"nameWithOwner":"owner/not-an-action",
			"stargazerCount":20,
			"url":"https://github.com/owner/not-an-action",
			"actionYml":null,
			"actionYaml":null
		}],"pageInfo":{"hasNextPage":true,"endCursor":"cursor-1"}}}`,
		`{"search":{"nodes":[{
			"nameWithOwner":"owner/action",
			"stargazerCount":10,
			"url":"https://github.com/owner/action",
			"actionYml":{"__typename":"Blob"},
			"actionYaml":null
		}],"pageInfo":{"hasNextPage":false,"endCursor":null}}}`,
	}}
	client := &Client{graphQL: graphQL}

	results, err := client.SearchRepositories(context.Background(), actions.SearchOptions{
		Query:          "build",
		ResultLimit:    1,
		CandidateLimit: 40,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != "owner/action" {
		t.Fatalf("unexpected results: %#v", results)
	}
	if len(graphQL.variableCalls) != 2 {
		t.Fatalf("GraphQL calls=%d want=2", len(graphQL.variableCalls))
	}
	cursor, ok := graphQL.variableCalls[1]["cursor"].(*string)
	if !ok || cursor == nil || *cursor != "cursor-1" {
		t.Fatalf("second cursor=%#v", graphQL.variableCalls[1]["cursor"])
	}
}

func TestSearchRepositoriesBoundsRequestsWhenPagesAreShort(t *testing.T) {
	graphQL := &fakeGraphQLClient{responses: []string{
		`{"search":{"nodes":[],"pageInfo":{"hasNextPage":true,"endCursor":"cursor-1"}}}`,
		`{"search":{"nodes":[],"pageInfo":{"hasNextPage":true,"endCursor":"cursor-2"}}}`,
	}}
	client := &Client{graphQL: graphQL}

	results, err := client.SearchRepositories(context.Background(), actions.SearchOptions{
		Query:          "build",
		ResultLimit:    1,
		CandidateLimit: 40,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 || len(graphQL.variableCalls) != 2 {
		t.Fatalf("results=%#v GraphQL calls=%d", results, len(graphQL.variableCalls))
	}
}
