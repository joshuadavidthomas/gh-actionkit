package actions

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type fakeVersionSource struct {
	release      string
	releaseFound bool
	tags         []string
	refs         map[string]string
	err          error
	calls        *resolutionCallCounter
}

type resolutionCallCounter struct {
	mutex sync.Mutex
	calls map[string]int
}

func newResolutionCallCounter() *resolutionCallCounter {
	return &resolutionCallCounter{calls: make(map[string]int)}
}

func (c *resolutionCallCounter) record(tag string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.calls[tag]++
}

func (c *resolutionCallCounter) count(tag string) int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.calls[tag]
}

func (f fakeVersionSource) LatestRelease(context.Context, Repository) (string, bool, error) {
	return f.release, f.releaseFound, f.err
}

func (f fakeVersionSource) Tags(context.Context, Repository) ([]string, error) {
	return f.tags, f.err
}

func (f fakeVersionSource) ResolveTag(_ context.Context, _ Repository, tag string) (string, bool, error) {
	if f.calls != nil {
		f.calls.record(tag)
	}
	if f.err != nil {
		return "", false, f.err
	}
	sha, found := f.refs[tag]
	return sha, found, nil
}

func TestVersionServiceReusesLatestSHAForMajorTag(t *testing.T) {
	calls := newResolutionCallCounter()
	service := NewVersionService(fakeVersionSource{
		release:      "v4",
		releaseFound: true,
		refs:         map[string]string{"v4": "latest-sha"},
		calls:        calls,
	})

	info, err := service.Lookup(context.Background(), "actions/checkout")
	if err != nil {
		t.Fatal(err)
	}
	if info.Major.SHA == nil || info.Latest.SHA == nil || *info.Major.SHA != *info.Latest.SHA {
		t.Fatalf("expected major and latest to share a SHA: %#v", info)
	}
	if got := calls.count("v4"); got != 1 {
		t.Fatalf("ResolveTag(v4) calls = %d, want 1", got)
	}
}

func TestVersionServiceUsesLatestRelease(t *testing.T) {
	service := NewVersionService(fakeVersionSource{
		release:      "v4.2.2",
		releaseFound: true,
		refs: map[string]string{
			"v4":     "major-sha",
			"v4.2.2": "latest-sha",
		},
	})

	info, err := service.Lookup(context.Background(), "actions/checkout")
	if err != nil {
		t.Fatal(err)
	}
	if info.Major.Tag != "v4" || info.Major.SHA == nil || *info.Major.SHA != "major-sha" {
		t.Fatalf("unexpected major version: %#v", info.Major)
	}
	if info.Latest.Tag != "v4.2.2" || info.Latest.SHA == nil || *info.Latest.SHA != "latest-sha" {
		t.Fatalf("unexpected latest version: %#v", info.Latest)
	}
}

type recordingVersionSource struct {
	resolved []string
}

func (s *recordingVersionSource) LatestRelease(context.Context, Repository) (string, bool, error) {
	return "v4.2.2", true, nil
}

func (s *recordingVersionSource) Tags(context.Context, Repository) ([]string, error) {
	return nil, nil
}

func (s *recordingVersionSource) ResolveTag(_ context.Context, _ Repository, tag string) (string, bool, error) {
	s.resolved = append(s.resolved, tag)
	return "0123456789abcdef0123456789abcdef01234567", true, nil
}

func TestVersionServiceLatestDoesNotResolveMajorTag(t *testing.T) {
	source := &recordingVersionSource{}

	version, err := NewVersionService(source).Latest(context.Background(), "actions/checkout")
	if err != nil {
		t.Fatal(err)
	}
	if version.Tag != "v4.2.2" || len(source.resolved) != 1 || source.resolved[0] != "v4.2.2" {
		t.Fatalf("version=%#v resolved=%v", version, source.resolved)
	}
}

func TestVersionServiceFallsBackToHighestStableTag(t *testing.T) {
	service := NewVersionService(fakeVersionSource{
		tags: []string{"v3.0.0-beta.1", "v1.9.0", "v2.1.0", "v2"},
		refs: map[string]string{"v2.1.0": "latest-sha"},
	})

	info, err := service.Lookup(context.Background(), "owner/action")
	if err != nil {
		t.Fatal(err)
	}
	if info.Latest.Tag != "v2.1.0" {
		t.Fatalf("got latest tag %q", info.Latest.Tag)
	}
	if info.Major.Tag != "v2" || info.Major.SHA != nil {
		t.Fatalf("unexpected major version: %#v", info.Major)
	}
}

func TestVersionServiceRejectsPrereleaseOnlyTags(t *testing.T) {
	service := NewVersionService(fakeVersionSource{
		tags: []string{"v3.0.0-beta.1", "v2.0.0-rc.1"},
	})

	_, err := service.Lookup(context.Background(), "owner/action")
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestVersionServiceFallsBackToNonSemanticTag(t *testing.T) {
	service := NewVersionService(fakeVersionSource{
		tags: []string{"release-current"},
		refs: map[string]string{"release-current": "commit-sha"},
	})

	info, err := service.Lookup(context.Background(), "owner/action")
	if err != nil {
		t.Fatal(err)
	}
	if info.Latest.Tag != "release-current" {
		t.Fatalf("got latest tag %q", info.Latest.Tag)
	}
}

func TestVersionServiceRejectsInvalidAction(t *testing.T) {
	service := NewVersionService(fakeVersionSource{})
	_, err := service.Lookup(context.Background(), "actions/checkout/subpath")
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestVersionServiceReportsSourceErrors(t *testing.T) {
	sourceErr := errors.New("rate limited")
	service := NewVersionService(fakeVersionSource{err: sourceErr})
	_, err := service.Lookup(context.Background(), "actions/checkout")
	if !errors.Is(err, sourceErr) {
		t.Fatalf("expected source error, got %v", err)
	}
}
