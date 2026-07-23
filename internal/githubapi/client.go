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
	return c.rest.DoWithContext(ctx, http.MethodGet, path, nil, response)
}

func repositoryPath(repository actions.Repository) string {
	return "repos/" + url.PathEscape(repository.Owner) + "/" + url.PathEscape(repository.Name)
}

func isNotFound(err error) bool {
	var httpError *api.HTTPError
	return errors.As(err, &httpError) && httpError.StatusCode == http.StatusNotFound
}
