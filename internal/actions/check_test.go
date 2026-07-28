package actions

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestCheckGroupsUsesAndComparesResolvedCommits(t *testing.T) {
	latestSHA := "0123456789012345678901234567890123456789"
	source := fakeVersionSource{
		release:      "v4.2.2",
		releaseFound: true,
		refs: map[string]string{
			"v4":           latestSHA,
			"v4.2.2":       latestSHA,
			"v3":           "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"0123456789ab": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},
	}
	uses := []ActionUse{
		{
			Action:     "owner/action/subpath",
			Repository: Repository{Owner: "owner", Name: "action"},
			Ref:        "v3",
			Location:   Location{File: ".github/workflows/ci.yml", Line: 10},
		},
		{
			Action:     "owner/action/subpath",
			Repository: Repository{Owner: "owner", Name: "action"},
			Ref:        "v3",
			Location:   Location{File: ".github/workflows/release.yml", Line: 12},
		},
		{
			Action:     "owner/action/subpath",
			Repository: Repository{Owner: "owner", Name: "action"},
			Ref:        latestSHA,
			Location:   Location{File: ".github/workflows/ci.yml", Line: 20},
		},
		{
			Action:     "owner/action/subpath",
			Repository: Repository{Owner: "owner", Name: "action"},
			Ref:        "main",
			Location:   Location{File: ".github/workflows/ci.yml", Line: 30},
		},
		{
			Action:     "owner/action/subpath",
			Repository: Repository{Owner: "owner", Name: "action"},
			Ref:        "0123456789ab",
			Location:   Location{File: ".github/workflows/ci.yml", Line: 40},
		},
	}

	results, err := NewCheckService(source).Check(context.Background(), uses)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 4 {
		t.Fatalf("unexpected results: %#v", results)
	}
	byRef := make(map[string]CheckResult)
	for _, result := range results {
		byRef[result.Ref] = result
	}
	if byRef["v3"].Status != CheckStatusUpdateAvailable || len(byRef["v3"].Locations) != 2 {
		t.Fatalf("unexpected v3 result: %#v", byRef["v3"])
	}
	if byRef[latestSHA].Status != CheckStatusUpToDate || !byRef[latestSHA].Pinned {
		t.Fatalf("latest SHA should be current and pinned: %#v", byRef[latestSHA])
	}
	if byRef["v3"].Pinned || byRef["main"].Pinned || byRef["0123456789ab"].Pinned {
		t.Fatalf(
			"non-SHA refs should not be pinned: v3=%#v main=%#v short=%#v",
			byRef["v3"],
			byRef["main"],
			byRef["0123456789ab"],
		)
	}
	if byRef["main"].Status != CheckStatusUnknown {
		t.Fatalf("unresolved branch should be unknown: %#v", byRef["main"])
	}
}

func TestCheckReusesLoadedVersionSHAs(t *testing.T) {
	calls := newResolutionCallCounter()
	source := fakeVersionSource{
		release:      "v4.2.2",
		releaseFound: true,
		refs: map[string]string{
			"v4":     "major-sha",
			"v4.2.2": "latest-sha",
		},
		calls: calls,
	}
	uses := []ActionUse{
		{
			Action:     "owner/action",
			Repository: Repository{Owner: "owner", Name: "action"},
			Ref:        "v4",
		},
		{
			Action:     "owner/action",
			Repository: Repository{Owner: "owner", Name: "action"},
			Ref:        "v4.2.2",
		},
	}

	results, err := NewCheckService(source).Check(context.Background(), uses)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].Status != CheckStatusUpToDate || results[1].Status != CheckStatusUpToDate {
		t.Fatalf("unexpected results: %#v", results)
	}
	for _, tag := range []string{"v4", "v4.2.2"} {
		if got := calls.count(tag); got != 1 {
			t.Fatalf("ResolveTag(%s) calls = %d, want 1", tag, got)
		}
	}
}

func TestApplyCheckPolicyReportsEachViolation(t *testing.T) {
	results := []CheckResult{
		{Action: "actions/checkout", Ref: "v4", Status: CheckStatusUpToDate},
		{Action: "third-party/action", Ref: "main", Status: CheckStatusUnknown},
		{
			Action: "actions/setup-go",
			Ref:    "0123456789012345678901234567890123456789",
			Pinned: true,
			Status: CheckStatusUnknown,
		},
	}

	got := ApplyCheckPolicy(results, CheckPolicy{
		RequireSHA:    true,
		FailOnUnknown: true,
		AllowedOwners: []string{"Actions"},
	})

	if len(got[0].PolicyViolations) != 1 || got[0].PolicyViolations[0] != PolicyViolationUnpinned {
		t.Fatalf("unexpected current tag violations: %#v", got[0].PolicyViolations)
	}
	want := []PolicyViolation{
		PolicyViolationUnpinned,
		PolicyViolationUnknown,
		PolicyViolationDisallowedOwner,
	}
	if len(got[1].PolicyViolations) != len(want) {
		t.Fatalf("unexpected unknown violations: %#v", got[1].PolicyViolations)
	}
	for index := range want {
		if got[1].PolicyViolations[index] != want[index] {
			t.Fatalf("unexpected unknown violations: %#v", got[1].PolicyViolations)
		}
	}
	if len(got[2].PolicyViolations) != 1 || got[2].PolicyViolations[0] != PolicyViolationUnknown {
		t.Fatalf("unexpected pinned unknown violations: %#v", got[2].PolicyViolations)
	}
}

func TestApplyCheckPolicyClearsEarlierViolations(t *testing.T) {
	results := []CheckResult{{
		Action:           "owner/action",
		Status:           CheckStatusUnknown,
		PolicyViolations: []PolicyViolation{PolicyViolationUnpinned},
	}}

	got := ApplyCheckPolicy(results, CheckPolicy{})

	if len(got[0].PolicyViolations) != 0 {
		t.Fatalf("unexpected violations: %#v", got[0].PolicyViolations)
	}
}

func TestCheckDoesNotTrustMissingMajorTagName(t *testing.T) {
	source := fakeVersionSource{
		release:      "v4.2.2",
		releaseFound: true,
		refs: map[string]string{
			"v4.2.2": "0123456789012345678901234567890123456789",
		},
	}
	uses := []ActionUse{{
		Action:     "owner/action",
		Repository: Repository{Owner: "owner", Name: "action"},
		Ref:        "v4",
	}}

	results, err := NewCheckService(source).Check(context.Background(), uses)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != CheckStatusUnknown {
		t.Fatalf("missing major tag should be unknown: %#v", results[0])
	}
}

func TestCheckTreatsDifferentPinnedSHAAsUnknown(t *testing.T) {
	latestSHA := "4444444444444444444444444444444444444444"
	usedSHA := "5555555555555555555555555555555555555555"
	source := fakeVersionSource{
		release:      "v4.2.2",
		releaseFound: true,
		refs:         map[string]string{"v4": latestSHA, "v4.2.2": latestSHA},
	}
	uses := []ActionUse{{
		Action:     "owner/action",
		Repository: Repository{Owner: "owner", Name: "action"},
		Ref:        usedSHA,
	}}

	results, err := NewCheckService(source).Check(context.Background(), uses)
	if err != nil {
		t.Fatal(err)
	}
	if !results[0].Pinned || results[0].Status != CheckStatusUnknown {
		t.Fatalf("different pinned SHA should be unknown: %#v", results[0])
	}
}

func TestCheckDoesNotCallNewerPrereleaseAnUpdate(t *testing.T) {
	source := fakeVersionSource{
		release:      "v4.2.2",
		releaseFound: true,
		refs: map[string]string{
			"v4":            "4444444444444444444444444444444444444444",
			"v4.2.2":        "4444444444444444444444444444444444444444",
			"v5.0.0-beta.1": "5555555555555555555555555555555555555555",
		},
	}
	uses := []ActionUse{{
		Action:     "owner/action",
		Repository: Repository{Owner: "owner", Name: "action"},
		Ref:        "v5.0.0-beta.1",
	}}

	results, err := NewCheckService(source).Check(context.Background(), uses)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != CheckStatusUnknown {
		t.Fatalf("newer prerelease should not be an update: %#v", results[0])
	}
}

func TestCheckMatchesCommitSHAsCaseInsensitively(t *testing.T) {
	lowerSHA := "abcdefabcdefabcdefabcdefabcdefabcdefabcd"
	upperSHA := "ABCDEFABCDEFABCDEFABCDEFABCDEFABCDEFABCD"
	source := fakeVersionSource{
		release:      "v1.0.0",
		releaseFound: true,
		refs:         map[string]string{"v1": lowerSHA, "v1.0.0": lowerSHA},
	}
	uses := []ActionUse{{
		Action:     "owner/action",
		Repository: Repository{Owner: "owner", Name: "action"},
		Ref:        upperSHA,
	}}

	results, err := NewCheckService(source).Check(context.Background(), uses)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != CheckStatusUpToDate {
		t.Fatalf("uppercase SHA should be current: %#v", results[0])
	}
}

func TestIsCommitSHA(t *testing.T) {
	for _, test := range []struct {
		ref  string
		want bool
	}{
		{ref: "abcdefabcdefabcdefabcdefabcdefabcdefabcd", want: true},
		{ref: "ABCDEFABCDEFABCDEFABCDEFABCDEFABCDEFABCD", want: true},
		{ref: "abcdef", want: false},
		{ref: "gggggggggggggggggggggggggggggggggggggggg", want: false},
	} {
		if got := IsCommitSHA(test.ref); got != test.want {
			t.Errorf("IsCommitSHA(%q) = %t, want %t", test.ref, got, test.want)
		}
	}
}

func TestCheckPropagatesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	source := fakeVersionSource{
		release:      "v1.0.0",
		releaseFound: true,
		refs:         map[string]string{"v1": "sha", "v1.0.0": "sha"},
	}
	uses := []ActionUse{{
		Action:     "owner/action",
		Repository: Repository{Owner: "owner", Name: "action"},
		Ref:        "0123456789012345678901234567890123456789",
	}}

	_, err := NewCheckService(source).Check(ctx, uses)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
}

func TestCheckTreatsRepositoriesWithoutVersionsAsUnknown(t *testing.T) {
	uses := []ActionUse{{
		Action:     "owner/action",
		Repository: Repository{Owner: "owner", Name: "action"},
		Ref:        "main",
	}}

	results, err := NewCheckService(fakeVersionSource{}).Check(context.Background(), uses)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Latest.Tag != nil || results[0].Status != CheckStatusUnknown {
		t.Fatalf("unexpected result: %#v", results)
	}
}

type workerPoolVersionSource struct {
	mutex          sync.Mutex
	calls          map[workerPoolCall]int
	gates          map[Repository]chan struct{}
	startedSignals map[Repository]chan struct{}
	errorWaits     map[Repository]<-chan struct{}
	errors         map[Repository]error
	noVersions     map[Repository]bool
	started        chan Repository
	errorReturned  chan Repository
}

type workerPoolCall struct {
	repository Repository
	method     string
}

func newWorkerPoolVersionSource() *workerPoolVersionSource {
	return &workerPoolVersionSource{
		calls:          make(map[workerPoolCall]int),
		gates:          make(map[Repository]chan struct{}),
		startedSignals: make(map[Repository]chan struct{}),
		errorWaits:     make(map[Repository]<-chan struct{}),
		errors:         make(map[Repository]error),
		noVersions:     make(map[Repository]bool),
		started:        make(chan Repository, 10),
		errorReturned:  make(chan Repository, 10),
	}
}

func (s *workerPoolVersionSource) LatestRelease(
	ctx context.Context,
	repository Repository,
) (string, bool, error) {
	s.record(repository, "LatestRelease")
	if gate, blocked := s.gates[repository]; blocked {
		select {
		case s.started <- repository:
		case <-ctx.Done():
			return "", false, ctx.Err()
		}
		if signal, found := s.startedSignals[repository]; found {
			close(signal)
		}
		select {
		case <-gate:
		case <-ctx.Done():
			return "", false, ctx.Err()
		}
	}
	if err := s.errors[repository]; err != nil {
		if wait, found := s.errorWaits[repository]; found {
			select {
			case <-wait:
			case <-ctx.Done():
				return "", false, ctx.Err()
			}
		}
		select {
		case s.errorReturned <- repository:
		default:
		}
		return "", false, err
	}
	if s.noVersions[repository] {
		return "", false, nil
	}
	return "v1.0.0", true, nil
}

func (s *workerPoolVersionSource) Tags(ctx context.Context, repository Repository) ([]string, error) {
	s.record(repository, "Tags")
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}

func (s *workerPoolVersionSource) ResolveTag(
	ctx context.Context,
	repository Repository,
	tag string,
) (string, bool, error) {
	s.record(repository, "ResolveTag")
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	return repository.Owner + "/" + repository.Name + "@" + tag, true, nil
}

func (s *workerPoolVersionSource) record(repository Repository, method string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.calls[workerPoolCall{repository: repository, method: method}]++
}

func (s *workerPoolVersionSource) callCount(repository Repository, method string) int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.calls[workerPoolCall{repository: repository, method: method}]
}

type checkOutcome struct {
	results []CheckResult
	err     error
}

func startCheck(ctx context.Context, service CheckService, uses []ActionUse) <-chan checkOutcome {
	done := make(chan checkOutcome, 1)
	go func() {
		results, err := service.Check(ctx, uses)
		done <- checkOutcome{results: results, err: err}
	}()
	return done
}

func waitForLookupStarts(
	t *testing.T,
	ctx context.Context,
	source *workerPoolVersionSource,
	count int,
) {
	t.Helper()
	started := make(map[Repository]bool, count)
	for range count {
		select {
		case repository := <-source.started:
			if started[repository] {
				t.Fatalf("lookup for %v started more than once", repository)
			}
			started[repository] = true
		case <-ctx.Done():
			t.Fatalf("only %d of %d blocked lookups started: %v", len(started), count, ctx.Err())
		}
	}
}

func waitForCheck(t *testing.T, ctx context.Context, done <-chan checkOutcome) checkOutcome {
	t.Helper()
	select {
	case outcome := <-done:
		return outcome
	case <-ctx.Done():
		t.Fatalf("Check did not return: %v", ctx.Err())
		return checkOutcome{}
	}
}

func TestCheckServiceLoadVersionsRunsRepositoriesConcurrently(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	repositories := []Repository{
		{Owner: "owner", Name: "alpha"},
		{Owner: "owner", Name: "bravo"},
		{Owner: "owner", Name: "charlie"},
	}
	source := newWorkerPoolVersionSource()
	for _, repository := range repositories {
		source.gates[repository] = make(chan struct{})
	}
	uses := []ActionUse{
		{Action: "owner/alpha", Repository: repositories[0], Ref: "v1", Location: Location{Line: 1}},
		{Action: "owner/alpha", Repository: repositories[0], Ref: "v1", Location: Location{Line: 2}},
		{Action: "owner/bravo", Repository: repositories[1], Ref: "v1"},
		{Action: "owner/charlie", Repository: repositories[2], Ref: "v1"},
	}
	service := NewCheckService(source)
	service.workers = 3
	done := startCheck(ctx, service, uses)

	waitForLookupStarts(t, ctx, source, len(repositories))
	for _, repository := range repositories {
		close(source.gates[repository])
	}
	outcome := waitForCheck(t, ctx, done)
	if outcome.err != nil {
		t.Fatal(outcome.err)
	}
	if len(outcome.results) != len(repositories) {
		t.Fatalf("got %d results, want %d: %#v", len(outcome.results), len(repositories), outcome.results)
	}
	for _, repository := range repositories {
		if got := source.callCount(repository, "LatestRelease"); got != 1 {
			t.Fatalf("LatestRelease(%v) calls = %d, want 1", repository, got)
		}
	}
	for _, result := range outcome.results {
		if result.Status != CheckStatusUpToDate {
			t.Fatalf("unexpected result: %#v", result)
		}
	}
}

func TestCheckServiceLoadVersionsCancelsAfterFirstError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	blocked := Repository{Owner: "owner", Name: "blocked"}
	failed := Repository{Owner: "owner", Name: "failed"}
	wantErr := errors.New("lookup failed")
	source := newWorkerPoolVersionSource()
	source.gates[blocked] = make(chan struct{})
	blockedStarted := make(chan struct{})
	source.startedSignals[blocked] = blockedStarted
	source.errorWaits[failed] = blockedStarted
	source.errors[failed] = wantErr
	uses := []ActionUse{
		{Action: "owner/blocked", Repository: blocked, Ref: "v1"},
		{Action: "owner/failed", Repository: failed, Ref: "v1"},
		{Action: "owner/other-one", Repository: Repository{Owner: "owner", Name: "other-one"}, Ref: "v1"},
		{Action: "owner/other-two", Repository: Repository{Owner: "owner", Name: "other-two"}, Ref: "v1"},
	}
	service := NewCheckService(source)
	service.workers = 2
	done := startCheck(ctx, service, uses)

	select {
	case repository := <-source.errorReturned:
		if repository != failed {
			t.Fatalf("error came from %v, want %v", repository, failed)
		}
	case <-ctx.Done():
		t.Fatalf("injected error was not returned: %v", ctx.Err())
	}
	close(source.gates[blocked])
	outcome := waitForCheck(t, ctx, done)
	if !errors.Is(outcome.err, wantErr) {
		t.Fatalf("Check error = %v, want %v", outcome.err, wantErr)
	}
}

func TestCheckServiceLoadVersionsKeepsOtherResultsWhenRepositoryHasNoVersions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	missing := Repository{Owner: "owner", Name: "missing"}
	source := newWorkerPoolVersionSource()
	source.noVersions[missing] = true
	uses := []ActionUse{
		{Action: "owner/alpha", Repository: Repository{Owner: "owner", Name: "alpha"}, Ref: "v1"},
		{Action: "owner/missing", Repository: missing, Ref: "main"},
		{Action: "owner/bravo", Repository: Repository{Owner: "owner", Name: "bravo"}, Ref: "v1"},
	}
	service := NewCheckService(source)
	service.workers = 3

	outcome := waitForCheck(t, ctx, startCheck(ctx, service, uses))
	if outcome.err != nil {
		t.Fatal(outcome.err)
	}
	if len(outcome.results) != len(uses) {
		t.Fatalf("got %d results, want %d: %#v", len(outcome.results), len(uses), outcome.results)
	}
	for _, result := range outcome.results {
		if result.Action == "owner/missing" {
			if result.Status != CheckStatusUnknown || result.Latest.Tag != nil {
				t.Fatalf("unexpected no-versions result: %#v", result)
			}
			continue
		}
		if result.Status != CheckStatusUpToDate {
			t.Fatalf("unexpected result: %#v", result)
		}
	}
}

func TestCheckServiceLoadVersionsReturnsCallerCancellation(t *testing.T) {
	deadline, cancelDeadline := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelDeadline()
	ctx, cancel := context.WithCancel(deadline)
	defer cancel()

	repositories := []Repository{
		{Owner: "owner", Name: "alpha"},
		{Owner: "owner", Name: "bravo"},
		{Owner: "owner", Name: "charlie"},
	}
	source := newWorkerPoolVersionSource()
	uses := make([]ActionUse, 0, len(repositories))
	for _, repository := range repositories {
		source.gates[repository] = make(chan struct{})
		uses = append(uses, ActionUse{
			Action:     repository.Owner + "/" + repository.Name,
			Repository: repository,
			Ref:        "v1",
		})
	}
	service := NewCheckService(source)
	service.workers = 3
	done := startCheck(ctx, service, uses)

	waitForLookupStarts(t, deadline, source, len(repositories))
	cancel()
	outcome := waitForCheck(t, deadline, done)
	if !errors.Is(outcome.err, context.Canceled) {
		t.Fatalf("Check error = %v, want context cancellation", outcome.err)
	}
}
