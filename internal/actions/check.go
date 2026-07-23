package actions

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/Masterminds/semver/v3"
)

var commitSHAPattern = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

type Location struct {
	File string `json:"file"`
	Line int    `json:"line"`
}

type ActionUse struct {
	Action     string
	Repository Repository
	Ref        string
	Location   Location
}

type CheckVersion struct {
	Tag *string `json:"tag"`
	SHA *string `json:"sha"`
}

type CheckResult struct {
	Action          string       `json:"action"`
	Used            CheckVersion `json:"used"`
	Major           CheckVersion `json:"major"`
	Latest          CheckVersion `json:"latest"`
	UpToDate        bool         `json:"up_to_date"`
	UpdateAvailable bool         `json:"update_available"`
	Locations       []Location   `json:"locations"`
	ref             string
}

type CheckReport struct {
	WorkflowFiles int
	Uses          int
	Results       []CheckResult
}

type CheckService struct {
	source  VersionSource
	workers int
}

func NewCheckService(source VersionSource) CheckService {
	return CheckService{source: source, workers: 10}
}

func (s CheckService) Check(ctx context.Context, uses []ActionUse) ([]CheckResult, error) {
	if len(uses) == 0 {
		return []CheckResult{}, nil
	}

	repositories := make(map[Repository]struct{})
	for _, use := range uses {
		repositories[use.Repository] = struct{}{}
	}
	versions, err := s.loadVersions(ctx, repositories)
	if err != nil {
		return nil, err
	}

	groups := make(map[string]*CheckResult)
	for _, use := range uses {
		key := use.Action + "\x00" + use.Ref
		group, found := groups[key]
		if !found {
			group, err = s.newResult(ctx, use, versions[use.Repository])
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
			return results[i].ref < results[j].ref
		}
		return results[i].Action < results[j].Action
	})
	return results, nil
}

func (s CheckService) loadVersions(
	ctx context.Context,
	repositories map[Repository]struct{},
) (map[Repository]*VersionInfo, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan Repository)
	versions := make(map[Repository]*VersionInfo, len(repositories))
	var firstError error
	var mutex sync.Mutex
	var workers sync.WaitGroup

	workerCount := min(s.workers, len(repositories))
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			service := NewVersionService(s.source)
			for repository := range jobs {
				name := repository.Owner + "/" + repository.Name
				version, err := service.Lookup(ctx, name)
				mutex.Lock()
				switch {
				case err == nil:
					versionCopy := version
					versions[repository] = &versionCopy
				case errors.Is(err, ErrNoVersions):
					versions[repository] = nil
				case firstError == nil:
					firstError = err
					cancel()
				}
				mutex.Unlock()
			}
		}()
	}

sendJobs:
	for repository := range repositories {
		select {
		case jobs <- repository:
		case <-ctx.Done():
			break sendJobs
		}
	}
	close(jobs)
	workers.Wait()
	if firstError != nil {
		return nil, firstError
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return versions, nil
}

func (s CheckService) newResult(
	ctx context.Context,
	use ActionUse,
	version *VersionInfo,
) (*CheckResult, error) {
	result := &CheckResult{Action: use.Action, ref: use.Ref}
	if commitSHAPattern.MatchString(use.Ref) {
		result.Used.SHA = stringPointer(use.Ref)
	} else {
		result.Used.Tag = stringPointer(use.Ref)
		sha, found, err := s.source.ResolveTag(ctx, use.Repository, use.Ref)
		if err != nil {
			return nil, fmt.Errorf("resolve used ref %s@%s: %w", use.Action, use.Ref, err)
		}
		if found {
			result.Used.SHA = stringPointer(sha)
		}
	}

	if version == nil {
		return result, nil
	}
	result.Major = checkVersion(version.Major)
	result.Latest = checkVersion(version.Latest)
	result.UpToDate = shaMatches(result.Used.SHA, version.Major.SHA) ||
		shaMatches(result.Used.SHA, version.Latest.SHA)
	result.UpdateAvailable = !result.UpToDate && hasNewerStableVersion(use.Ref, result.Used.SHA, version.Latest)
	return result, nil
}

func checkVersion(version Version) CheckVersion {
	return CheckVersion{Tag: stringPointer(version.Tag), SHA: version.SHA}
}

func stringPointer(value string) *string {
	return &value
}

func shaMatches(left, right *string) bool {
	return left != nil && right != nil && strings.EqualFold(*left, *right)
}

func hasNewerStableVersion(usedRef string, usedSHA *string, latest Version) bool {
	if usedSHA == nil || latest.SHA == nil {
		return false
	}
	if commitSHAPattern.MatchString(usedRef) {
		return !shaMatches(usedSHA, latest.SHA)
	}
	usedVersion, usedErr := semver.NewVersion(usedRef)
	latestVersion, latestErr := semver.NewVersion(latest.Tag)
	return usedErr == nil && latestErr == nil && usedVersion.LessThan(latestVersion)
}
