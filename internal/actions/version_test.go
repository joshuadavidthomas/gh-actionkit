package actions

import (
	"context"
	"errors"
	"testing"
)

type fakeVersionSource struct {
	release      string
	releaseFound bool
	tags         []string
	refs         map[string]string
	err          error
}

func (f fakeVersionSource) LatestRelease(context.Context, Repository) (string, bool, error) {
	return f.release, f.releaseFound, f.err
}

func (f fakeVersionSource) Tags(context.Context, Repository) ([]string, error) {
	return f.tags, f.err
}

func (f fakeVersionSource) ResolveTag(_ context.Context, _ Repository, tag string) (string, bool, error) {
	if f.err != nil {
		return "", false, f.err
	}
	sha, found := f.refs[tag]
	return sha, found, nil
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
