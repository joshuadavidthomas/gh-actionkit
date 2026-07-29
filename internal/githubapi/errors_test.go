package githubapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

func TestNormalizeErrorClassifiesAuthenticationFailure(t *testing.T) {
	cause := &api.HTTPError{StatusCode: http.StatusUnauthorized}

	err := normalizeError(cause, time.Time{})
	var githubError *githubError
	if !errors.As(err, &githubError) || githubError.kind != errorAuthentication {
		t.Fatalf("unexpected error: %#v", err)
	}
	if !errors.Is(err, cause) || !strings.Contains(err.Error(), "gh auth status") {
		t.Fatalf("authentication error lost its cause or recovery advice: %v", err)
	}
}

func TestNormalizeErrorClassifiesOnlyProvenForbiddenRateLimits(t *testing.T) {
	ordinary := normalizeError(&api.HTTPError{StatusCode: http.StatusForbidden}, time.Time{})
	var ordinaryError *githubError
	if !errors.As(ordinary, &ordinaryError) || ordinaryError.kind != errorResponse {
		t.Fatalf("ordinary forbidden response classified as rate limit: %v", ordinary)
	}

	headers := http.Header{"Retry-After": []string{"60"}}
	now := time.Date(2026, time.March, 1, 12, 0, 0, 0, time.UTC)
	limited := normalizeError(&api.HTTPError{
		StatusCode: http.StatusForbidden,
		Headers:    headers,
	}, now)
	var limitedError *githubError
	if !errors.As(limited, &limitedError) || limitedError.kind != errorRateLimit {
		t.Fatalf("unexpected rate-limit error: %v", limited)
	}
	wantRetry := now.Add(time.Minute)
	if limitedError.retryAt == nil || !limitedError.retryAt.Equal(wantRetry) {
		t.Fatalf("retry time=%v want=%v", limitedError.retryAt, wantRetry)
	}
}

func TestRetryAtParsesHTTPDate(t *testing.T) {
	want := time.Date(2026, time.March, 1, 12, 5, 0, 0, time.UTC)
	headers := http.Header{}
	headers.Set("Retry-After", want.Format(http.TimeFormat))

	got := retryAt(headers, time.Time{})
	if got == nil || !got.Equal(want) {
		t.Fatalf("retry time=%v want=%v", got, want)
	}
}

func TestNormalizeErrorParsesPrimaryRateLimitReset(t *testing.T) {
	reset := time.Date(2026, time.March, 1, 12, 5, 0, 0, time.UTC)
	headers := http.Header{}
	headers.Set("X-RateLimit-Remaining", "0")
	headers.Set("X-RateLimit-Reset", "1772366700")

	err := normalizeError(&api.HTTPError{
		StatusCode: http.StatusForbidden,
		Headers:    headers,
	}, time.Time{})
	var githubError *githubError
	if !errors.As(err, &githubError) || githubError.kind != errorRateLimit {
		t.Fatalf("unexpected error: %v", err)
	}
	if githubError.retryAt == nil || !githubError.retryAt.Equal(reset) {
		t.Fatalf("retry time=%v want=%v", githubError.retryAt, reset)
	}
}

func TestNormalizeErrorClassifiesTooManyRequests(t *testing.T) {
	err := normalizeError(&api.HTTPError{StatusCode: http.StatusTooManyRequests}, time.Time{})
	var githubError *githubError
	if !errors.As(err, &githubError) || githubError.kind != errorRateLimit {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeErrorClassifiesGraphQLRateLimit(t *testing.T) {
	cause := &api.GraphQLError{Errors: []api.GraphQLErrorItem{{
		Type:    "RATE_LIMITED",
		Message: "API rate limit exceeded",
	}}}

	err := normalizeError(cause, time.Time{})
	var githubError *githubError
	if !errors.As(err, &githubError) || githubError.kind != errorRateLimit {
		t.Fatalf("unexpected error: %v", err)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("GraphQL rate-limit error lost its cause: %v", err)
	}
}

func TestNormalizeErrorPreservesContextCancellation(t *testing.T) {
	if err := normalizeError(context.Canceled, time.Time{}); err != context.Canceled {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := normalizeError(context.DeadlineExceeded, time.Time{}); err != context.DeadlineExceeded {
		t.Fatalf("unexpected error: %v", err)
	}
}
