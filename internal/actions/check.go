package actions

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"golang.org/x/sync/errgroup"
)

var commitSHAPattern = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

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

type CheckService struct {
	source VersionSource
}

func NewCheckService(source VersionSource) CheckService {
	return CheckService{source: source}
}

func (s CheckService) Check(ctx context.Context, uses []ActionUse) ([]CheckResult, error) {
	if len(uses) == 0 {
		return []CheckResult{}, nil
	}

	repositories := make(map[Repository]struct{})
	for _, use := range uses {
		repositories[use.Identifier.Repository()] = struct{}{}
	}
	versions, err := s.loadVersions(ctx, repositories)
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
			group, err = s.newResult(ctx, use, versions[use.Identifier.Repository()])
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

func (s CheckService) loadVersions(
	ctx context.Context,
	repositories map[Repository]struct{},
) (map[Repository]*RepositoryVersions, error) {
	repositoryList := make([]Repository, 0, len(repositories))
	for repository := range repositories {
		repositoryList = append(repositoryList, repository)
	}
	loaded := make([]*RepositoryVersions, len(repositoryList))

	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(10)
	service := NewVersionService(s.source)
	for index, repository := range repositoryList {
		group.Go(func() error {
			if err := groupCtx.Err(); err != nil {
				return err
			}
			version, err := service.Lookup(groupCtx, repository)
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

func (s CheckService) newResult(
	ctx context.Context,
	use ActionUse,
	version *RepositoryVersions,
) (*CheckResult, error) {
	result := &CheckResult{
		Action: use.Identifier.String(),
		Ref:    use.Ref,
		Pinned: IsCommitSHA(use.Ref),
		Status: CheckStatusUnknown,
	}
	if result.Pinned {
		result.Used.SHA = stringPointer(use.Ref)
	} else {
		result.Used.Tag = stringPointer(use.Ref)
		switch {
		case version != nil && use.Ref == version.Major.Tag:
			result.Used.SHA = version.Major.SHA
		case version != nil && use.Ref == version.Latest.Tag:
			result.Used.SHA = version.Latest.SHA
		default:
			sha, found, err := s.source.ResolveTag(ctx, use.Identifier.Repository(), use.Ref)
			if err != nil {
				return nil, fmt.Errorf("resolve used ref %s@%s: %w", use.Identifier, use.Ref, err)
			}
			if found {
				result.Used.SHA = stringPointer(sha)
			}
		}
	}

	if version == nil {
		return result, nil
	}
	result.Major = CheckVersion{Tag: stringPointer(version.Major.Tag), SHA: version.Major.SHA}
	result.Latest = CheckVersion{Tag: stringPointer(version.Latest.Tag), SHA: version.Latest.SHA}
	if shaMatches(result.Used.SHA, version.Major.SHA) || shaMatches(result.Used.SHA, version.Latest.SHA) {
		result.Status = CheckStatusUpToDate
	} else if hasNewerStableVersion(use.Ref, result.Used.SHA, version.Latest) {
		result.Status = CheckStatusUpdateAvailable
	}
	return result, nil
}

func IsCommitSHA(ref string) bool {
	return commitSHAPattern.MatchString(ref)
}

func stringPointer(value string) *string {
	return &value
}

func shaMatches(left, right *string) bool {
	return left != nil && right != nil && strings.EqualFold(*left, *right)
}

func hasNewerStableVersion(usedRef string, usedSHA *string, latest Version) bool {
	if usedSHA == nil || latest.SHA == nil || IsCommitSHA(usedRef) {
		return false
	}
	usedVersion, usedErr := semver.NewVersion(usedRef)
	latestVersion, latestErr := semver.NewVersion(latest.Tag)
	return usedErr == nil && latestErr == nil && usedVersion.LessThan(latestVersion)
}
