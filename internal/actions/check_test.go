package actions

import (
	"context"
	"testing"
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
	if !byRef["v3"].UpdateAvailable || len(byRef["v3"].Locations) != 2 {
		t.Fatalf("unexpected v3 result: %#v", byRef["v3"])
	}
	if !byRef[latestSHA].UpToDate || !byRef[latestSHA].Pinned {
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
	if byRef["main"].UpToDate || byRef["main"].UpdateAvailable {
		t.Fatalf("unresolved branch should be unknown: %#v", byRef["main"])
	}
}

func TestApplyCheckPolicyReportsEachViolation(t *testing.T) {
	results := []CheckResult{
		{Action: "actions/checkout", Ref: "v4", UpToDate: true},
		{Action: "third-party/action", Ref: "main"},
		{
			Action: "actions/setup-go",
			Ref:    "0123456789012345678901234567890123456789",
			Pinned: true,
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
	if results[0].UpToDate || results[0].UpdateAvailable {
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
	if !results[0].Pinned || results[0].UpToDate || results[0].UpdateAvailable {
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
	if results[0].UpdateAvailable {
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
	if !results[0].UpToDate {
		t.Fatalf("uppercase SHA should be current: %#v", results[0])
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
	if len(results) != 1 || results[0].Latest.Tag != nil || results[0].UpdateAvailable {
		t.Fatalf("unexpected result: %#v", results)
	}
}
