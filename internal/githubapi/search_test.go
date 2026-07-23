package githubapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
)

func TestSearchRepositoriesEncodesQuery(t *testing.T) {
	path := "search/repositories?order=desc&per_page=10&q=docker+%26+build+action+in%3Aname%2Cdescription&sort=stars"
	rest := &fakeRESTClient{responses: map[string]string{
		path: `{"items":[{"full_name":"docker/build-push-action","description":"Build images","stargazers_count":7100,"html_url":"https://github.com/docker/build-push-action"}]}`,
	}}
	client := &Client{rest: rest}

	results, err := client.SearchRepositories(context.Background(), "docker & build", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Action != "docker/build-push-action" {
		t.Fatalf("unexpected results: %#v", results)
	}
}

func TestHasFileDistinguishesMissingFileFromAPIFailure(t *testing.T) {
	repository := actions.Repository{Owner: "owner", Name: "action"}
	missingPath := "repos/owner/action/contents/action.yml"
	rest := &fakeRESTClient{errors: map[string]error{
		missingPath: &api.HTTPError{StatusCode: http.StatusNotFound},
	}}
	client := &Client{rest: rest}

	found, err := client.HasFile(context.Background(), repository, "action.yml")
	if err != nil || found {
		t.Fatalf("found=%v err=%v", found, err)
	}
}
