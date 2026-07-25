package actions

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

var ErrNoActionManifest = errors.New("root action.yml or action.yaml not found")

type RepositoryOwner struct {
	Login string `json:"login"`
	Type  string `json:"type"`
}

type RepositoryLicense struct {
	Name   string `json:"name"`
	SPDXID string `json:"spdx_id"`
}

type ManifestFile struct {
	Path    string
	Content string
}

type RepositoryDetails struct {
	Description *string            `json:"description"`
	URL         string             `json:"url"`
	Owner       RepositoryOwner    `json:"owner"`
	Archived    bool               `json:"archived"`
	PushedAt    *time.Time         `json:"pushed_at"`
	License     *RepositoryLicense `json:"license"`
}

type RepositoryInspection struct {
	Action     string
	Repository RepositoryDetails
	Manifest   *ManifestFile
}

type ManifestInput struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Required    bool    `json:"required"`
	Default     *string `json:"default"`
}

type ManifestOutput struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Value       *string `json:"value"`
}

type ActionManifest struct {
	Path        string           `json:"path"`
	Ref         string           `json:"ref"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Runtime     string           `json:"runtime"`
	Inputs      []ManifestInput  `json:"inputs"`
	Outputs     []ManifestOutput `json:"outputs"`
}

type InspectResult struct {
	Action     string            `json:"action"`
	Repository RepositoryDetails `json:"repository"`
	Manifest   ActionManifest    `json:"manifest"`
	Latest     *Version          `json:"latest"`
	PinnedUses *string           `json:"pinned_uses"`
}

type InspectSource interface {
	VersionSource
	InspectRepository(context.Context, Repository, string) (RepositoryInspection, error)
}

type InspectService struct {
	source InspectSource
}

func NewInspectService(source InspectSource) InspectService {
	return InspectService{source: source}
}

func (s InspectService) Inspect(ctx context.Context, action string) (InspectResult, error) {
	repository, err := parseRepository(action)
	if err != nil {
		return InspectResult{}, err
	}

	manifestRef := "HEAD"
	var latest *Version
	resolved, err := NewVersionService(s.source).Latest(ctx, action)
	switch {
	case err == nil:
		latest = &resolved
		manifestRef = resolved.Tag
		if resolved.SHA != nil && commitSHAPattern.MatchString(*resolved.SHA) {
			manifestRef = *resolved.SHA
		}
	case errors.Is(err, ErrNoVersions):
	case err != nil:
		return InspectResult{}, err
	}

	inspection, err := s.source.InspectRepository(ctx, repository, manifestRef)
	if err != nil {
		return InspectResult{}, fmt.Errorf("inspect repository %s: %w", action, err)
	}
	if inspection.Manifest == nil {
		return InspectResult{}, fmt.Errorf("inspect %s at %s: %w", action, manifestRef, ErrNoActionManifest)
	}
	manifest, err := parseActionManifest(*inspection.Manifest, inspection.Action)
	if err != nil {
		return InspectResult{}, err
	}
	manifest.Ref = manifestRef

	result := InspectResult{
		Action:     inspection.Action,
		Repository: inspection.Repository,
		Manifest:   manifest,
		Latest:     latest,
	}
	if latest != nil && latest.SHA != nil && commitSHAPattern.MatchString(*latest.SHA) {
		pinnedUses := fmt.Sprintf("uses: %s@%s # %s", inspection.Action, *latest.SHA, latest.Tag)
		result.PinnedUses = &pinnedUses
	}
	return result, nil
}

type manifestDocument struct {
	Name        string                            `yaml:"name"`
	Description string                            `yaml:"description"`
	Inputs      map[string]manifestInputDocument  `yaml:"inputs"`
	Outputs     map[string]manifestOutputDocument `yaml:"outputs"`
	Runs        struct {
		Using string `yaml:"using"`
	} `yaml:"runs"`
}

type manifestInputDocument struct {
	Description *string   `yaml:"description"`
	Required    bool      `yaml:"required"`
	Default     yaml.Node `yaml:"default"`
}

type manifestOutputDocument struct {
	Description *string `yaml:"description"`
	Value       *string `yaml:"value"`
}

func parseActionManifest(file ManifestFile, action string) (ActionManifest, error) {
	var document manifestDocument
	if err := yaml.Unmarshal([]byte(file.Content), &document); err != nil {
		return ActionManifest{}, fmt.Errorf("parse %s for %s: %w", file.Path, action, err)
	}
	if document.Runs.Using == "" {
		return ActionManifest{}, fmt.Errorf("parse %s for %s: runs.using is required", file.Path, action)
	}

	manifest := ActionManifest{
		Path:        file.Path,
		Name:        document.Name,
		Description: document.Description,
		Runtime:     document.Runs.Using,
		Inputs:      make([]ManifestInput, 0, len(document.Inputs)),
		Outputs:     make([]ManifestOutput, 0, len(document.Outputs)),
	}
	inputNames := sortedKeys(document.Inputs)
	for _, name := range inputNames {
		input := document.Inputs[name]
		defaultValue, err := manifestScalar(input.Default)
		if err != nil {
			return ActionManifest{}, fmt.Errorf("parse %s for %s: input %s default: %w", file.Path, action, name, err)
		}
		manifest.Inputs = append(manifest.Inputs, ManifestInput{
			Name:        name,
			Description: input.Description,
			Required:    input.Required,
			Default:     defaultValue,
		})
	}
	outputNames := sortedKeys(document.Outputs)
	for _, name := range outputNames {
		output := document.Outputs[name]
		manifest.Outputs = append(manifest.Outputs, ManifestOutput{
			Name:        name,
			Description: output.Description,
			Value:       output.Value,
		})
	}
	return manifest, nil
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func manifestScalar(node yaml.Node) (*string, error) {
	for node.Kind == yaml.AliasNode && node.Alias != nil {
		node = *node.Alias
	}
	if node.Kind == 0 || node.Tag == "!!null" {
		return nil, nil
	}
	if node.Kind != yaml.ScalarNode {
		return nil, errors.New("must be a scalar value")
	}
	value := node.Value
	return &value, nil
}
