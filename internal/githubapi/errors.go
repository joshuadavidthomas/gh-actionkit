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

type ErrorKind string

const (
	ErrorAuthentication ErrorKind = "authentication"
	ErrorRateLimit      ErrorKind = "rate_limit"
	ErrorResponse       ErrorKind = "response"
)

type Error struct {
	Kind       ErrorKind
	StatusCode int
	RetryAt    *time.Time
	cause      error
}

func (e *Error) Error() string {
	switch e.Kind {
	case ErrorAuthentication:
		return fmt.Sprintf("GitHub authentication failed: %v; run `gh auth status`", e.cause)
	case ErrorRateLimit:
		if e.RetryAt != nil {
			return fmt.Sprintf("GitHub API rate limited until %s", e.RetryAt.Format(time.RFC3339))
		}
		return "GitHub API rate limited; retry later"
	default:
		return fmt.Sprintf("GitHub API request failed: %v", e.cause)
	}
}

func (e *Error) Unwrap() error {
	return e.cause
}

func authenticationError(err error) error {
	return &Error{Kind: ErrorAuthentication, cause: err}
}

func normalizeError(err error, now time.Time) error {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	var graphQLError *api.GraphQLError
	if errors.As(err, &graphQLError) {
		kind := ErrorResponse
		if isGraphQLRateLimit(graphQLError) {
			kind = ErrorRateLimit
		}
		return &Error{Kind: kind, cause: err}
	}

	var httpError *api.HTTPError
	if !errors.As(err, &httpError) {
		return &Error{Kind: ErrorResponse, cause: err}
	}

	kind := ErrorResponse
	switch {
	case httpError.StatusCode == http.StatusUnauthorized:
		kind = ErrorAuthentication
	case isRateLimitError(httpError):
		kind = ErrorRateLimit
	}
	githubError := &Error{
		Kind:       kind,
		StatusCode: httpError.StatusCode,
		cause:      err,
	}
	if kind == ErrorRateLimit {
		githubError.RetryAt = retryAt(httpError.Headers, now)
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
