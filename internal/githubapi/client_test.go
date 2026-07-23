package githubapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
)

type fakeRESTClient struct {
	responses map[string]string
	errors    map[string]error
	paths     []string
}

func (f *fakeRESTClient) DoWithContext(_ context.Context, method, path string, _ io.Reader, response interface{}) error {
	if method != http.MethodGet {
		return errors.New("unexpected HTTP method")
	}
	f.paths = append(f.paths, path)
	if err := f.errors[path]; err != nil {
		return err
	}
	body, found := f.responses[path]
	if !found {
		return errors.New("unexpected path: " + path)
	}
	return json.Unmarshal([]byte(body), response)
}

func TestResolveTagPeelsAnnotatedTags(t *testing.T) {
	rest := &fakeRESTClient{responses: map[string]string{
		"repos/actions/checkout/git/ref/tags/v4.2.2": `{"object":{"type":"tag","sha":"tag-object"}}`,
		"repos/actions/checkout/git/tags/tag-object": `{"object":{"type":"commit","sha":"commit-sha"}}`,
	}}
	client := &Client{rest: rest}

	sha, found, err := client.ResolveTag(
		context.Background(),
		actions.Repository{Owner: "actions", Name: "checkout"},
		"v4.2.2",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !found || sha != "commit-sha" {
		t.Fatalf("got sha=%q found=%v", sha, found)
	}
}

func TestResolveTagReturnsNotFound(t *testing.T) {
	path := "repos/actions/checkout/git/ref/tags/v4"
	rest := &fakeRESTClient{errors: map[string]error{
		path: &api.HTTPError{StatusCode: http.StatusNotFound},
	}}
	client := &Client{rest: rest}

	_, found, err := client.ResolveTag(
		context.Background(),
		actions.Repository{Owner: "actions", Name: "checkout"},
		"v4",
	)
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("expected missing tag")
	}
}

func TestRateLimitErrorSuggestsCheckingAuthentication(t *testing.T) {
	path := "repos/actions/checkout/releases/latest"
	headers := http.Header{}
	headers.Set("X-RateLimit-Remaining", "0")
	rest := &fakeRESTClient{errors: map[string]error{
		path: &api.HTTPError{StatusCode: http.StatusForbidden, Headers: headers},
	}}
	client := &Client{rest: rest}

	_, _, err := client.LatestRelease(
		context.Background(),
		actions.Repository{Owner: "actions", Name: "checkout"},
	)
	if err == nil || !strings.Contains(err.Error(), "gh auth status") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSecondaryRateLimitSuggestsWhenToRetry(t *testing.T) {
	path := "repos/actions/checkout/releases/latest"
	headers := http.Header{}
	headers.Set("Retry-After", "60")
	rest := &fakeRESTClient{errors: map[string]error{
		path: &api.HTTPError{StatusCode: http.StatusForbidden, Headers: headers},
	}}
	client := &Client{rest: rest}

	_, _, err := client.LatestRelease(
		context.Background(),
		actions.Repository{Owner: "actions", Name: "checkout"},
	)
	if err == nil || !strings.Contains(err.Error(), "retry after 60 seconds") {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(err.Error(), "gh auth status") {
		t.Fatalf("secondary limit gave authentication advice: %v", err)
	}
}

func TestTagsPaginates(t *testing.T) {
	firstPage := make([]map[string]string, tagsPerPage)
	for index := range firstPage {
		firstPage[index] = map[string]string{"name": "v1"}
	}
	firstJSON, err := json.Marshal(firstPage)
	if err != nil {
		t.Fatal(err)
	}
	rest := &fakeRESTClient{responses: map[string]string{
		"repos/owner/action/tags?per_page=100&page=1": string(firstJSON),
		"repos/owner/action/tags?per_page=100&page=2": `[{"name":"v2"}]`,
	}}
	client := &Client{rest: rest}

	tags, err := client.Tags(context.Background(), actions.Repository{Owner: "owner", Name: "action"})
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != tagsPerPage+1 || tags[len(tags)-1] != "v2" {
		t.Fatalf("unexpected tags: length=%d last=%q", len(tags), tags[len(tags)-1])
	}
}
