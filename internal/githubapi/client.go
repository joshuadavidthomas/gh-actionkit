package githubapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
)

const tagsPerPage = 100

type restClient interface {
	DoWithContext(context.Context, string, string, io.Reader, interface{}) error
}

type Client struct {
	rest restClient
}

func New() (*Client, error) {
	rest, err := api.DefaultRESTClient()
	if err != nil {
		return nil, err
	}
	return &Client{rest: rest}, nil
}

func (c *Client) LatestRelease(ctx context.Context, repository actions.Repository) (string, bool, error) {
	var response struct {
		TagName string `json:"tag_name"`
	}
	err := c.get(ctx, repositoryPath(repository)+"/releases/latest", &response)
	if isNotFound(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return response.TagName, response.TagName != "", nil
}

func (c *Client) Tags(ctx context.Context, repository actions.Repository) ([]string, error) {
	var tags []string
	for page := 1; ; page++ {
		var response []struct {
			Name string `json:"name"`
		}
		path := fmt.Sprintf("%s/tags?per_page=%d&page=%d", repositoryPath(repository), tagsPerPage, page)
		if err := c.get(ctx, path, &response); err != nil {
			return nil, err
		}
		for _, tag := range response {
			tags = append(tags, tag.Name)
		}
		if len(response) < tagsPerPage {
			return tags, nil
		}
	}
}

func (c *Client) SearchRepositories(ctx context.Context, query string, limit int) ([]actions.SearchResult, error) {
	parameters := url.Values{
		"q":        {query + " action in:name,description"},
		"sort":     {"stars"},
		"order":    {"desc"},
		"per_page": {fmt.Sprint(limit)},
	}
	var response struct {
		Items []struct {
			FullName        string  `json:"full_name"`
			Description     *string `json:"description"`
			StargazersCount int     `json:"stargazers_count"`
			HTMLURL         string  `json:"html_url"`
		} `json:"items"`
	}
	if err := c.get(ctx, "search/repositories?"+parameters.Encode(), &response); err != nil {
		return nil, err
	}

	results := make([]actions.SearchResult, 0, len(response.Items))
	for _, item := range response.Items {
		results = append(results, actions.SearchResult{
			Action:      item.FullName,
			Description: item.Description,
			Stars:       item.StargazersCount,
			URL:         item.HTMLURL,
		})
	}
	return results, nil
}

func (c *Client) HasFile(ctx context.Context, repository actions.Repository, name string) (bool, error) {
	var response struct {
		Type string `json:"type"`
	}
	path := repositoryPath(repository) + "/contents/" + url.PathEscape(name)
	if err := c.get(ctx, path, &response); err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return response.Type == "file", nil
}

func (c *Client) ResolveTag(ctx context.Context, repository actions.Repository, tag string) (string, bool, error) {
	var ref struct {
		Object gitObject `json:"object"`
	}
	path := repositoryPath(repository) + "/git/ref/tags/" + url.PathEscape(tag)
	if err := c.get(ctx, path, &ref); err != nil {
		if isNotFound(err) {
			return "", false, nil
		}
		return "", false, err
	}

	object := ref.Object
	for depth := 0; depth < 10; depth++ {
		switch object.Type {
		case "commit":
			return object.SHA, true, nil
		case "tag":
			var annotatedTag struct {
				Object gitObject `json:"object"`
			}
			if err := c.get(ctx, repositoryPath(repository)+"/git/tags/"+object.SHA, &annotatedTag); err != nil {
				return "", false, err
			}
			object = annotatedTag.Object
		default:
			return "", false, fmt.Errorf("tag %q points to unsupported Git object type %q", tag, object.Type)
		}
	}
	return "", false, fmt.Errorf("tag %q exceeds annotated tag resolution depth", tag)
}

type gitObject struct {
	Type string `json:"type"`
	SHA  string `json:"sha"`
}

func (c *Client) get(ctx context.Context, path string, response interface{}) error {
	err := c.rest.DoWithContext(ctx, http.MethodGet, path, nil, response)
	if advice, limited := rateLimitAdvice(err); limited {
		return fmt.Errorf("%s: %w", advice, err)
	}
	return err
}

func repositoryPath(repository actions.Repository) string {
	return "repos/" + url.PathEscape(repository.Owner) + "/" + url.PathEscape(repository.Name)
}

func isNotFound(err error) bool {
	var httpError *api.HTTPError
	return errors.As(err, &httpError) && httpError.StatusCode == http.StatusNotFound
}

func rateLimitAdvice(err error) (string, bool) {
	var httpError *api.HTTPError
	if !errors.As(err, &httpError) {
		return "", false
	}
	if retryAfter := httpError.Headers.Get("Retry-After"); retryAfter != "" {
		return "GitHub API rate limited; retry after " + retryAfter + " seconds", true
	}
	if httpError.StatusCode == http.StatusTooManyRequests {
		return "GitHub API rate limited; retry later", true
	}
	if httpError.StatusCode == http.StatusForbidden && httpError.Headers.Get("X-RateLimit-Remaining") == "0" {
		return "GitHub API rate limited; run `gh auth status` to check authentication", true
	}
	return "", false
}
