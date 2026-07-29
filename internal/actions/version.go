package actions

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/Masterminds/semver/v3"
)

var majorTagPattern = regexp.MustCompile(`^(v?\d+)`)
var ErrNoVersions = errors.New("no releases or tags found")

type Repository struct {
	Owner string
	Name  string
}

type Version struct {
	Tag string  `json:"tag"`
	SHA *string `json:"sha"`
}

type RepositoryVersions struct {
	Major  Version `json:"major"`
	Latest Version `json:"latest"`
}

type VersionSource interface {
	LatestRelease(context.Context, Repository) (tag string, found bool, err error)
	Tags(context.Context, Repository) ([]string, error)
	ResolveTag(context.Context, Repository, string) (sha string, found bool, err error)
}

type VersionService struct {
	source VersionSource
}

func NewVersionService(source VersionSource) VersionService {
	return VersionService{source: source}
}

func (s VersionService) Lookup(ctx context.Context, repository Repository) (RepositoryVersions, error) {
	latest, err := s.latest(ctx, repository)
	if err != nil {
		return RepositoryVersions{}, err
	}

	majorTag := latest.Tag
	if match := majorTagPattern.FindStringSubmatch(latest.Tag); match != nil {
		majorTag = match[1]
	}
	majorSHA := latest.SHA
	if majorTag != latest.Tag {
		majorSHA, err = s.resolveTag(ctx, repository, majorTag)
		if err != nil {
			return RepositoryVersions{}, fmt.Errorf(
				"resolve tag %s for %s/%s: %w",
				majorTag,
				repository.Owner,
				repository.Name,
				err,
			)
		}
	}

	return RepositoryVersions{
		Major:  Version{Tag: majorTag, SHA: majorSHA},
		Latest: latest,
	}, nil
}

func (s VersionService) Latest(ctx context.Context, repository Repository) (Version, error) {
	return s.latest(ctx, repository)
}

func (s VersionService) latest(ctx context.Context, repository Repository) (Version, error) {
	name := repository.Owner + "/" + repository.Name
	latestTag, found, err := s.source.LatestRelease(ctx, repository)
	if err != nil {
		return Version{}, fmt.Errorf("find latest release for %s: %w", name, err)
	}
	if !found {
		tags, tagsErr := s.source.Tags(ctx, repository)
		if tagsErr != nil {
			return Version{}, fmt.Errorf("list tags for %s: %w", name, tagsErr)
		}
		latestTag, found = latestStableTag(tags)
	}
	if !found {
		return Version{}, fmt.Errorf("%w for %s", ErrNoVersions, name)
	}

	latestSHA, err := s.resolveTag(ctx, repository, latestTag)
	if err != nil {
		return Version{}, fmt.Errorf("resolve tag %s for %s: %w", latestTag, name, err)
	}
	return Version{Tag: latestTag, SHA: latestSHA}, nil
}

func (s VersionService) resolveTag(ctx context.Context, repository Repository, tag string) (*string, error) {
	sha, found, err := s.source.ResolveTag(ctx, repository, tag)
	if err != nil || !found {
		return nil, err
	}
	return &sha, nil
}

func latestStableTag(tags []string) (string, bool) {
	var selected string
	var selectedVersion *semver.Version
	var firstNonSemanticTag string

	for _, tag := range tags {
		version, err := semver.NewVersion(tag)
		if err != nil {
			if firstNonSemanticTag == "" {
				firstNonSemanticTag = tag
			}
			continue
		}
		if version.Prerelease() != "" {
			continue
		}
		if selectedVersion == nil || version.GreaterThan(selectedVersion) ||
			(version.Equal(selectedVersion) && len(tag) > len(selected)) {
			selected = tag
			selectedVersion = version
		}
	}
	if selectedVersion != nil {
		return selected, true
	}
	if firstNonSemanticTag != "" {
		return firstNonSemanticTag, true
	}
	return "", false
}
