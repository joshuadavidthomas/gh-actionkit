package actions

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

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

type VersionInfo struct {
	Action string  `json:"action"`
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

func (s VersionService) Lookup(ctx context.Context, action string) (VersionInfo, error) {
	repository, err := parseRepository(action)
	if err != nil {
		return VersionInfo{}, err
	}

	latestTag, found, err := s.source.LatestRelease(ctx, repository)
	if err != nil {
		return VersionInfo{}, fmt.Errorf("find latest release for %s: %w", action, err)
	}
	if !found {
		tags, tagsErr := s.source.Tags(ctx, repository)
		if tagsErr != nil {
			return VersionInfo{}, fmt.Errorf("list tags for %s: %w", action, tagsErr)
		}
		latestTag, found = latestStableTag(tags)
	}
	if !found {
		return VersionInfo{}, fmt.Errorf("%w for %s", ErrNoVersions, action)
	}

	majorTag := latestTag
	if match := majorTagPattern.FindStringSubmatch(latestTag); match != nil {
		majorTag = match[1]
	}

	latestSHA, err := s.resolveTag(ctx, repository, latestTag)
	if err != nil {
		return VersionInfo{}, fmt.Errorf("resolve tag %s for %s: %w", latestTag, action, err)
	}
	majorSHA, err := s.resolveTag(ctx, repository, majorTag)
	if err != nil {
		return VersionInfo{}, fmt.Errorf("resolve tag %s for %s: %w", majorTag, action, err)
	}

	return VersionInfo{
		Action: action,
		Major:  Version{Tag: majorTag, SHA: majorSHA},
		Latest: Version{Tag: latestTag, SHA: latestSHA},
	}, nil
}

func (s VersionService) resolveTag(ctx context.Context, repository Repository, tag string) (*string, error) {
	sha, found, err := s.source.ResolveTag(ctx, repository, tag)
	if err != nil || !found {
		return nil, err
	}
	return &sha, nil
}

func parseRepository(action string) (Repository, error) {
	parts := strings.Split(action, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Repository{}, fmt.Errorf("invalid action %q: expected owner/repo", action)
	}
	return Repository{Owner: parts[0], Name: parts[1]}, nil
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
