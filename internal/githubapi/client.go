package githubapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
)

const (
	tagsPerPage = 100
	// Manifest lookups make a 100-repository GraphQL query too costly for GitHub.
	graphQLSearchPageSize = 20
	requestTimeout        = 30 * time.Second

	searchRepositoriesQuery = `
query SearchRepositories($query: String!, $limit: Int!, $cursor: String) {
  search(query: $query, type: REPOSITORY, first: $limit, after: $cursor) {
    nodes {
      ... on Repository {
        nameWithOwner
        description
        stargazerCount
        url
        actionYml: object(expression: "HEAD:action.yml") {
          __typename
        }
        actionYaml: object(expression: "HEAD:action.yaml") {
          __typename
        }
      }
    }
    pageInfo {
      hasNextPage
      endCursor
    }
  }
}`
)

type restClient interface {
	DoWithContext(context.Context, string, string, io.Reader, interface{}) error
}

type graphQLClient interface {
	DoWithContext(context.Context, string, map[string]interface{}, interface{}) error
}

type Client struct {
	rest    restClient
	graphQL graphQLClient
}

var (
	_ actions.SearchSource  = (*Client)(nil)
	_ actions.VersionSource = (*Client)(nil)
)

func New() (*Client, error) {
	options := api.ClientOptions{Timeout: requestTimeout}
	rest, err := api.NewRESTClient(options)
	if err != nil {
		return nil, authenticationError(err)
	}
	graphQL, err := api.NewGraphQLClient(options)
	if err != nil {
		return nil, authenticationError(err)
	}
	return &Client{rest: rest, graphQL: graphQL}, nil
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

func (c *Client) SearchRepositories(
	ctx context.Context,
	options actions.SearchOptions,
) ([]actions.SearchResult, error) {
	results := make([]actions.SearchResult, 0, options.ResultLimit)
	var cursor *string
	candidatesRequested := 0
	for candidatesRequested < options.CandidateLimit && len(results) < options.ResultLimit {
		pageSize := min(graphQLSearchPageSize, options.CandidateLimit-candidatesRequested)
		candidatesRequested += pageSize
		variables := map[string]interface{}{
			"query":  options.Query + " action in:name,description sort:stars-desc",
			"limit":  pageSize,
			"cursor": cursor,
		}
		var response searchRepositoriesResponse
		if err := c.graphQL.DoWithContext(ctx, searchRepositoriesQuery, variables, &response); err != nil {
			return nil, normalizeError(err, time.Now())
		}

		for _, repository := range response.Search.Nodes {
			if !repository.hasActionManifest() {
				continue
			}
			results = append(results, actions.SearchResult{
				Action:      repository.NameWithOwner,
				Description: repository.Description,
				Stars:       repository.StargazerCount,
				URL:         repository.URL,
			})
			if len(results) == options.ResultLimit {
				break
			}
		}
		if !response.Search.PageInfo.HasNextPage || response.Search.PageInfo.EndCursor == nil {
			break
		}
		cursor = response.Search.PageInfo.EndCursor
	}
	return results, nil
}

type searchRepositoriesResponse struct {
	Search struct {
		Nodes    []searchRepository `json:"nodes"`
		PageInfo struct {
			HasNextPage bool    `json:"hasNextPage"`
			EndCursor   *string `json:"endCursor"`
		} `json:"pageInfo"`
	} `json:"search"`
}

type searchRepository struct {
	NameWithOwner  string         `json:"nameWithOwner"`
	Description    *string        `json:"description"`
	StargazerCount int            `json:"stargazerCount"`
	URL            string         `json:"url"`
	ActionYML      *graphQLObject `json:"actionYml"`
	ActionYAML     *graphQLObject `json:"actionYaml"`
}

type graphQLObject struct {
	TypeName string `json:"__typename"`
}

func (repository searchRepository) hasActionManifest() bool {
	return isBlob(repository.ActionYML) || isBlob(repository.ActionYAML)
}

func isBlob(object *graphQLObject) bool {
	return object != nil && object.TypeName == "Blob"
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
	return normalizeError(err, time.Now())
}

func repositoryPath(repository actions.Repository) string {
	return "repos/" + url.PathEscape(repository.Owner) + "/" + url.PathEscape(repository.Name)
}

func isNotFound(err error) bool {
	var httpError *api.HTTPError
	return errors.As(err, &httpError) && httpError.StatusCode == http.StatusNotFound
}
