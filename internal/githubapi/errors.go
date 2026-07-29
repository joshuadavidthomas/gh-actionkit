package githubapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

type errorKind string

const (
	errorAuthentication errorKind = "authentication"
	errorRateLimit      errorKind = "rate_limit"
	errorResponse       errorKind = "response"
)

type githubError struct {
	kind    errorKind
	retryAt *time.Time
	cause   error
}

func (e *githubError) Error() string {
	switch e.kind {
	case errorAuthentication:
		return fmt.Sprintf("GitHub authentication failed: %v; run `gh auth status`", e.cause)
	case errorRateLimit:
		if e.retryAt != nil {
			return fmt.Sprintf("GitHub API rate limited until %s", e.retryAt.Format(time.RFC3339))
		}
		return "GitHub API rate limited; retry later"
	default:
		return fmt.Sprintf("GitHub API request failed: %v", e.cause)
	}
}

func (e *githubError) Unwrap() error {
	return e.cause
}

func authenticationError(err error) error {
	return &githubError{kind: errorAuthentication, cause: err}
}

func normalizeError(err error, now time.Time) error {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	var graphQLError *api.GraphQLError
	if errors.As(err, &graphQLError) {
		kind := errorResponse
		if isGraphQLRateLimit(graphQLError) {
			kind = errorRateLimit
		}
		return &githubError{kind: kind, cause: err}
	}

	var httpError *api.HTTPError
	if !errors.As(err, &httpError) {
		return &githubError{kind: errorResponse, cause: err}
	}

	kind := errorResponse
	switch {
	case httpError.StatusCode == http.StatusUnauthorized:
		kind = errorAuthentication
	case isRateLimitError(httpError):
		kind = errorRateLimit
	}
	githubError := &githubError{kind: kind, cause: err}
	if kind == errorRateLimit {
		githubError.retryAt = retryAt(httpError.Headers, now)
	}
	return githubError
}

func isGraphQLRateLimit(err *api.GraphQLError) bool {
	for _, item := range err.Errors {
		if item.Type == "RATE_LIMITED" || item.Extensions["type"] == "RATE_LIMITED" ||
			item.Extensions["code"] == "RATE_LIMITED" {
			return true
		}
	}
	return false
}

func isRateLimitError(err *api.HTTPError) bool {
	if err.StatusCode == http.StatusTooManyRequests {
		return true
	}
	if err.StatusCode != http.StatusForbidden {
		return false
	}
	message := strings.ToLower(err.Message)
	return err.Headers.Get("Retry-After") != "" ||
		err.Headers.Get("X-RateLimit-Remaining") == "0" ||
		strings.Contains(message, "secondary rate limit") ||
		strings.Contains(message, "abuse detection")
}

func retryAt(headers http.Header, now time.Time) *time.Time {
	if retryAfter := headers.Get("Retry-After"); retryAfter != "" {
		if seconds, err := strconv.ParseInt(retryAfter, 10, 64); err == nil && seconds >= 0 {
			retry := now.Add(time.Duration(seconds) * time.Second)
			return &retry
		}
		if retry, err := http.ParseTime(retryAfter); err == nil {
			return &retry
		}
	}
	if reset := headers.Get("X-RateLimit-Reset"); reset != "" {
		if seconds, err := strconv.ParseInt(reset, 10, 64); err == nil {
			retry := time.Unix(seconds, 0)
			return &retry
		}
	}
	return nil
}
