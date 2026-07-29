package actions

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"golang.org/x/sync/errgroup"
)

type Location struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

type ActionUse struct {
	Identifier ActionIdentifier
	Ref        string
	Location   Location
}

type CheckVersion struct {
	Tag *string `json:"tag"`
	SHA *string `json:"sha"`
}

type CheckStatus string

const (
	CheckStatusUpToDate        CheckStatus = "up_to_date"
	CheckStatusUpdateAvailable CheckStatus = "update_available"
	CheckStatusUnknown         CheckStatus = "unknown"
)

type PolicyViolation string

const (
	PolicyViolationUnpinned        PolicyViolation = "unpinned"
	PolicyViolationUnknown         PolicyViolation = "unknown"
	PolicyViolationDisallowedOwner PolicyViolation = "disallowed_owner"
)

type CheckResult struct {
	Action           string            `json:"action"`
	Ref              string            `json:"ref"`
	Pinned           bool              `json:"pinned"`
	Used             CheckVersion      `json:"used"`
	Major            CheckVersion      `json:"major"`
	Latest           CheckVersion      `json:"latest"`
	Status           CheckStatus       `json:"status"`
	PolicyViolations []PolicyViolation `json:"policy_violations,omitempty"`
	Locations        []Location        `json:"locations"`
}

type CheckPolicy struct {
	RequireSHA    bool
	FailOnUnknown bool
	AllowedOwners []string
}

func Check(ctx context.Context, source VersionSource, uses []ActionUse) ([]CheckResult, error) {
	if len(uses) == 0 {
		return []CheckResult{}, nil
	}

	repositories := make(map[Repository]struct{})
	for _, use := range uses {
		repositories[use.Identifier.Repository()] = struct{}{}
	}
	versions, err := loadVersions(ctx, source, repositories)
	if err != nil {
		return nil, err
	}

	type groupKey struct {
		identifier ActionIdentifier
		ref        string
	}
	groups := make(map[groupKey]*CheckResult)
	for _, use := range uses {
		key := groupKey{identifier: use.Identifier, ref: use.Ref}
		group, found := groups[key]
		if !found {
			group, err = newCheckResult(ctx, source, use, versions[use.Identifier.Repository()])
			if err != nil {
				return nil, err
			}
			groups[key] = group
		}
		group.Locations = append(group.Locations, use.Location)
	}

	results := make([]CheckResult, 0, len(groups))
	for _, result := range groups {
		results = append(results, *result)
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Action == results[j].Action {
			return results[i].Ref < results[j].Ref
		}
		return results[i].Action < results[j].Action
	})
	return results, nil
}

func ApplyCheckPolicy(results []CheckResult, policy CheckPolicy) {
	allowedOwners := make(map[string]struct{}, len(policy.AllowedOwners))
	for _, owner := range policy.AllowedOwners {
		allowedOwners[strings.ToLower(owner)] = struct{}{}
	}

	for index := range results {
		result := &results[index]
		result.PolicyViolations = nil
		if policy.RequireSHA && !result.Pinned {
			result.PolicyViolations = append(result.PolicyViolations, PolicyViolationUnpinned)
		}
		if policy.FailOnUnknown && result.Status == CheckStatusUnknown {
			result.PolicyViolations = append(result.PolicyViolations, PolicyViolationUnknown)
		}
		if len(allowedOwners) > 0 {
			owner, _, _ := strings.Cut(result.Action, "/")
			if _, allowed := allowedOwners[strings.ToLower(owner)]; !allowed {
				result.PolicyViolations = append(result.PolicyViolations, PolicyViolationDisallowedOwner)
			}
		}
	}
}

func loadVersions(
	ctx context.Context,
	source VersionSource,
	repositories map[Repository]struct{},
) (map[Repository]*RepositoryVersions, error) {
	repositoryList := make([]Repository, 0, len(repositories))
	for repository := range repositories {
		repositoryList = append(repositoryList, repository)
	}
	loaded := make([]*RepositoryVersions, len(repositoryList))

	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(10)
	for index, repository := range repositoryList {
		group.Go(func() error {
			if err := groupCtx.Err(); err != nil {
				return err
			}
			version, err := LookupVersions(groupCtx, source, repository)
			switch {
			case err == nil:
				loaded[index] = &version
				return nil
			case errors.Is(err, ErrNoVersions):
				return nil
			default:
				return err
			}
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	versions := make(map[Repository]*RepositoryVersions, len(repositoryList))
	for index, repository := range repositoryList {
		versions[repository] = loaded[index]
	}
	return versions, nil
}

func newCheckResult(
	ctx context.Context,
	source VersionSource,
	use ActionUse,
	version *RepositoryVersions,
) (*CheckResult, error) {
	result := &CheckResult{
		Action: use.Identifier.String(),
		Ref:    use.Ref,
		Pinned: commitSHAPattern.MatchString(use.Ref),
	}
	if result.Pinned {
		result.Used.SHA = &use.Ref
	} else {
		result.Used.Tag = &use.Ref
		switch {
		case version != nil && use.Ref == version.Major.Tag:
			result.Used.SHA = version.Major.SHA
		case version != nil && use.Ref == version.Latest.Tag:
			result.Used.SHA = version.Latest.SHA
		default:
			sha, found, err := source.ResolveTag(ctx, use.Identifier.Repository(), use.Ref)
			if err != nil {
				return nil, fmt.Errorf("resolve used ref %s@%s: %w", use.Identifier, use.Ref, err)
			}
			if found {
				result.Used.SHA = &sha
			}
		}
	}

	if version != nil {
		majorTag := version.Major.Tag
		latestTag := version.Latest.Tag
		result.Major = CheckVersion{Tag: &majorTag, SHA: version.Major.SHA}
		result.Latest = CheckVersion{Tag: &latestTag, SHA: version.Latest.SHA}
	}
	result.Status = classifyCheckStatus(use.Ref, result.Pinned, result.Used.SHA, version)
	return result, nil
}

func classifyCheckStatus(
	usedRef string,
	pinned bool,
	usedSHA *string,
	versions *RepositoryVersions,
) CheckStatus {
	if versions == nil {
		return CheckStatusUnknown
	}
	if usedSHA != nil &&
		((versions.Major.SHA != nil && strings.EqualFold(*usedSHA, *versions.Major.SHA)) ||
			(versions.Latest.SHA != nil && strings.EqualFold(*usedSHA, *versions.Latest.SHA))) {
		return CheckStatusUpToDate
	}
	if usedSHA == nil || versions.Latest.SHA == nil || pinned {
		return CheckStatusUnknown
	}
	usedVersion, usedErr := semver.NewVersion(usedRef)
	latestVersion, latestErr := semver.NewVersion(versions.Latest.Tag)
	if usedErr == nil && latestErr == nil && usedVersion.LessThan(latestVersion) {
		return CheckStatusUpdateAvailable
	}
	return CheckStatusUnknown
}
