package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joshuadavidthomas/gh-actionkit/internal/githubapi"
	"github.com/joshuadavidthomas/gh-actionkit/internal/tools"
)

func TestLintWorkflowsSelectsOnlineAndOfflineModes(t *testing.T) {
	tests := []struct {
		name    string
		offline bool
	}{
		{name: "online"},
		{name: "offline", offline: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resolved := false
			resolve := func(context.Context) (githubapi.Credentials, error) {
				resolved = true
				return githubapi.Credentials{Host: "github.example.com", Token: "secret-token"}, nil
			}
			lint := func(
				_ context.Context,
				_ string,
				options tools.ZizmorOptions,
				_, _ io.Writer,
			) (int, error) {
				if test.offline && options.GitHub != nil {
					t.Fatalf("offline credentials=%#v", options.GitHub)
				}
				if !test.offline && (options.GitHub == nil || options.GitHub.Host != "github.example.com" ||
					options.GitHub.Token != "secret-token") {
					t.Fatalf("online credentials=%#v", options.GitHub)
				}
				return 0, nil
			}

			_, err := lintWorkflowsWith(
				context.Background(),
				"/repo",
				lintOptions{Offline: test.offline},
				io.Discard,
				io.Discard,
				resolve,
				lint,
			)
			if err != nil {
				t.Fatal(err)
			}
			if resolved == test.offline {
				t.Fatalf("resolved=%v offline=%v", resolved, test.offline)
			}
		})
	}
}

func TestLintWorkflowsStopsOnAuthenticationFailure(t *testing.T) {
	authErr := errors.New("no token")
	lintCalled := false
	_, err := lintWorkflowsWith(
		context.Background(),
		"/repo",
		lintOptions{},
		io.Discard,
		io.Discard,
		func(context.Context) (githubapi.Credentials, error) {
			return githubapi.Credentials{}, authErr
		},
		func(context.Context, string, tools.ZizmorOptions, io.Writer, io.Writer) (int, error) {
			lintCalled = true
			return 0, nil
		},
	)
	if !errors.Is(err, authErr) || !strings.Contains(err.Error(), "use --offline") {
		t.Fatalf("unexpected error: %v", err)
	}
	if lintCalled {
		t.Fatal("zizmor should not run after authentication failure")
	}
}

func TestLintWorkflowsPreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := lintWorkflowsWith(
		ctx,
		"/repo",
		lintOptions{},
		io.Discard,
		io.Discard,
		func(ctx context.Context) (githubapi.Credentials, error) {
			return githubapi.Credentials{}, ctx.Err()
		},
		func(context.Context, string, tools.ZizmorOptions, io.Writer, io.Writer) (int, error) {
			t.Fatal("zizmor should not run after cancellation")
			return 0, nil
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLintForwardsOptionsAndStatus(t *testing.T) {
	repository := t.TempDir()
	lint := func(
		_ context.Context,
		gotRepository string,
		options lintOptions,
		_, _ io.Writer,
	) (int, error) {
		wantRepository, err := filepath.Abs(repository)
		if err != nil {
			t.Fatal(err)
		}
		if gotRepository != wantRepository || !options.OutputJSON || !options.Pedantic || options.Offline {
			t.Fatalf("repository=%q options=%#v", gotRepository, options)
		}
		return 13, nil
	}
	command := commandForTest(
		newLintCommandWithLint(lint),
		&bytes.Buffer{},
		&bytes.Buffer{},
		"-C",
		repository,
		"--json",
		"--pedantic",
	)

	err := command.Execute()
	var statusError StatusError
	if !errors.As(err, &statusError) || statusError.Code != 13 {
		t.Fatalf("expected status 13, got %v", err)
	}
}

func TestLintNoPedanticOverridesDefault(t *testing.T) {
	lint := func(_ context.Context, _ string, options lintOptions, _, _ io.Writer) (int, error) {
		if options.Pedantic {
			t.Fatal("pedantic should be disabled")
		}
		return 0, nil
	}
	command := commandForTest(
		newLintCommandWithLint(lint),
		&bytes.Buffer{},
		&bytes.Buffer{},
		"-C",
		t.TempDir(),
		"--no-pedantic",
	)

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestLintForwardsOffline(t *testing.T) {
	lint := func(_ context.Context, _ string, options lintOptions, _, _ io.Writer) (int, error) {
		if !options.Offline {
			t.Fatal("offline should be enabled")
		}
		return 0, nil
	}
	command := commandForTest(
		newLintCommandWithLint(lint),
		&bytes.Buffer{},
		&bytes.Buffer{},
		"-C",
		t.TempDir(),
		"--offline",
	)

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
}
