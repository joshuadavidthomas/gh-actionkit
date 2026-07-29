package actions

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"go.yaml.in/yaml/v3"
)

var ErrNoActionManifest = errors.New("action manifest not found")

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
	Repository Repository
	Details    RepositoryDetails
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
	InspectAction(context.Context, ActionIdentifier, string) (RepositoryInspection, error)
}

func Inspect(ctx context.Context, source InspectSource, identifier ActionIdentifier) (InspectResult, error) {
	manifestRef := "HEAD"
	var latest *Version
	var pinnedSHA *string
	resolved, err := LatestVersion(ctx, source, identifier.Repository())
	switch {
	case err == nil:
		latest = &resolved
		manifestRef = resolved.Tag
		pinnedSHA = resolved.PinnedSHA()
		if pinnedSHA != nil {
			manifestRef = *pinnedSHA
		}
	case errors.Is(err, ErrNoVersions):
	case err != nil:
		return InspectResult{}, err
	}

	inspection, err := source.InspectAction(ctx, identifier, manifestRef)
	if err != nil {
		return InspectResult{}, fmt.Errorf("inspect action %s: %w", identifier, err)
	}
	canonicalIdentifier := identifier.WithRepository(inspection.Repository)
	if inspection.Manifest == nil {
		yml, yaml := canonicalIdentifier.ManifestCandidates()
		return InspectResult{}, fmt.Errorf(
			"inspect %s at %s: %s or %s: %w",
			canonicalIdentifier,
			manifestRef,
			yml,
			yaml,
			ErrNoActionManifest,
		)
	}
	manifest, err := parseActionManifest(*inspection.Manifest, canonicalIdentifier.String())
	if err != nil {
		return InspectResult{}, err
	}
	manifest.Ref = manifestRef

	result := InspectResult{
		Action:     canonicalIdentifier.String(),
		Repository: inspection.Details,
		Manifest:   manifest,
		Latest:     latest,
	}
	if pinnedSHA != nil {
		pinnedUses := fmt.Sprintf("uses: %s@%s # %s", canonicalIdentifier, *pinnedSHA, latest.Tag)
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
	inputNames := slices.Sorted(maps.Keys(document.Inputs))
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
	outputNames := slices.Sorted(maps.Keys(document.Outputs))
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
