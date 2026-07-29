package githubapi

import (
	"context"
	"fmt"
	"time"

	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
)

const inspectRepositoryQuery = `
query InspectRepository($owner: String!, $name: String!, $actionYml: String!, $actionYaml: String!) {
  repository(owner: $owner, name: $name) {
    nameWithOwner
    description
    url
    isArchived
    pushedAt
    owner {
      login
      __typename
    }
    licenseInfo {
      name
      spdxId
    }
    actionYml: object(expression: $actionYml) {
      __typename
      ... on Blob {
        text
        isTruncated
      }
    }
    actionYaml: object(expression: $actionYaml) {
      __typename
      ... on Blob {
        text
        isTruncated
      }
    }
  }
}`

type inspectRepositoryResponse struct {
	Repository *inspectRepository `json:"repository"`
}

type inspectRepository struct {
	NameWithOwner string     `json:"nameWithOwner"`
	Description   *string    `json:"description"`
	URL           string     `json:"url"`
	IsArchived    bool       `json:"isArchived"`
	PushedAt      *time.Time `json:"pushedAt"`
	Owner         struct {
		Login    string `json:"login"`
		TypeName string `json:"__typename"`
	} `json:"owner"`
	LicenseInfo *struct {
		Name   string `json:"name"`
		SPDXID string `json:"spdxId"`
	} `json:"licenseInfo"`
	ActionYML  *inspectBlob `json:"actionYml"`
	ActionYAML *inspectBlob `json:"actionYaml"`
}

type inspectBlob struct {
	TypeName    string  `json:"__typename"`
	Text        *string `json:"text"`
	IsTruncated bool    `json:"isTruncated"`
}

func (c *Client) InspectAction(
	ctx context.Context,
	identifier actions.ActionIdentifier,
	ref string,
) (actions.RepositoryInspection, error) {
	repository := identifier.Repository()
	actionYML, actionYAML := identifier.ManifestCandidates()
	variables := map[string]interface{}{
		"owner":      repository.Owner,
		"name":       repository.Name,
		"actionYml":  ref + ":" + actionYML,
		"actionYaml": ref + ":" + actionYAML,
	}
	var response inspectRepositoryResponse
	if err := c.graphQL.DoWithContext(ctx, inspectRepositoryQuery, variables, &response); err != nil {
		return actions.RepositoryInspection{}, normalizeError(err, time.Now())
	}
	if response.Repository == nil {
		return actions.RepositoryInspection{}, fmt.Errorf(
			"repository %s/%s was not found",
			repository.Owner,
			repository.Name,
		)
	}

	source := response.Repository
	canonicalRepository, err := actions.ParseRepository(source.NameWithOwner)
	if err != nil {
		return actions.RepositoryInspection{}, fmt.Errorf("parse GitHub repository identity: %w", err)
	}
	inspection := actions.RepositoryInspection{
		Repository: canonicalRepository,
		Details: actions.RepositoryDetails{
			Description: source.Description,
			URL:         source.URL,
			Owner: actions.RepositoryOwner{
				Login: source.Owner.Login,
				Type:  source.Owner.TypeName,
			},
			Archived: source.IsArchived,
			PushedAt: source.PushedAt,
		},
	}
	if source.LicenseInfo != nil {
		inspection.Details.License = &actions.RepositoryLicense{
			Name:   source.LicenseInfo.Name,
			SPDXID: source.LicenseInfo.SPDXID,
		}
	}
	manifest, found, err := inspectManifestFile(actionYML, source.ActionYML)
	if err != nil {
		return actions.RepositoryInspection{}, err
	}
	if !found {
		manifest, found, err = inspectManifestFile(actionYAML, source.ActionYAML)
		if err != nil {
			return actions.RepositoryInspection{}, err
		}
	}
	if found {
		inspection.Manifest = manifest
	}
	return inspection, nil
}

func inspectManifestFile(path string, blob *inspectBlob) (*actions.ManifestFile, bool, error) {
	if blob == nil || blob.TypeName != "Blob" {
		return nil, false, nil
	}
	if blob.IsTruncated {
		return nil, false, fmt.Errorf("%s content is truncated", path)
	}
	if blob.Text == nil {
		return nil, false, fmt.Errorf("%s content is unavailable", path)
	}
	return &actions.ManifestFile{Path: path, Content: *blob.Text}, true, nil
}
