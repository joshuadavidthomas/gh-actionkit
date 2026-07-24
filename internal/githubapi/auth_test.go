package githubapi

import (
	"context"
	"errors"
	"testing"
)

func TestResolveCredentialsUsesConfiguredTokenForDefaultHost(t *testing.T) {
	secureStorageCalled := false
	credentials, err := resolveCredentials(context.Background(), credentialSource{
		defaultHost: func() (string, string) { return "github.example.com", "hosts" },
		knownHosts:  func() []string { return []string{"github.example.com"} },
		tokenFromEnvOrConfig: func(host string) (string, string) {
			if host != "github.example.com" {
				t.Fatalf("host=%q", host)
			}
			return "configured-token", "oauth_token"
		},
		tokenFromSecureStorage: func(context.Context, string) (string, error) {
			secureStorageCalled = true
			return "", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if secureStorageCalled {
		t.Fatal("secure storage should not be read")
	}
	if credentials.Host != "github.example.com" || credentials.Token != "configured-token" {
		t.Fatalf("unexpected credentials: %#v", credentials)
	}
}

func TestResolveCredentialsFallsBackToSecureStorage(t *testing.T) {
	credentials, err := resolveCredentials(context.Background(), credentialSource{
		defaultHost:          func() (string, string) { return "github.com", "hosts" },
		knownHosts:           func() []string { return []string{"github.com"} },
		tokenFromEnvOrConfig: func(string) (string, string) { return "", "default" },
		tokenFromSecureStorage: func(_ context.Context, host string) (string, error) {
			if host != "github.com" {
				t.Fatalf("host=%q", host)
			}
			return "secure-token", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if credentials.Token != "secure-token" {
		t.Fatalf("unexpected credentials: %#v", credentials)
	}
}

func TestResolveCredentialsRejectsAmbiguousConfiguredHost(t *testing.T) {
	_, err := resolveCredentials(context.Background(), credentialSource{
		defaultHost:          func() (string, string) { return "github.example.com", "hosts" },
		knownHosts:           func() []string { return []string{"github.com", "github.example.com"} },
		tokenFromEnvOrConfig: func(string) (string, string) { return "token", "env" },
		tokenFromSecureStorage: func(context.Context, string) (string, error) {
			return "", nil
		},
	})
	assertAuthenticationError(t, err, "multiple authenticated GitHub hosts; set GH_HOST")
}

func TestResolveCredentialsAcceptsExplicitHostAmongMultipleHosts(t *testing.T) {
	credentials, err := resolveCredentials(context.Background(), credentialSource{
		defaultHost:          func() (string, string) { return "github.example.com", "GH_HOST" },
		knownHosts:           func() []string { return []string{"github.com", "github.example.com"} },
		tokenFromEnvOrConfig: func(string) (string, string) { return "token", "env" },
		tokenFromSecureStorage: func(context.Context, string) (string, error) {
			return "", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if credentials.Host != "github.example.com" {
		t.Fatalf("host=%q", credentials.Host)
	}
}

func TestResolveCredentialsRequiresHostAndToken(t *testing.T) {
	tests := []struct {
		name        string
		host        string
		wantMessage string
	}{
		{name: "host", wantMessage: "GitHub host not found"},
		{name: "token", host: "github.com", wantMessage: "authentication token not found for github.com"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := resolveCredentials(context.Background(), credentialSource{
				defaultHost:          func() (string, string) { return test.host, "hosts" },
				knownHosts:           func() []string { return []string{test.host} },
				tokenFromEnvOrConfig: func(string) (string, string) { return "", "default" },
				tokenFromSecureStorage: func(context.Context, string) (string, error) {
					return "", nil
				},
			})
			assertAuthenticationError(t, err, test.wantMessage)
		})
	}
}

func TestResolveCredentialsPreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := resolveCredentials(ctx, credentialSource{
		defaultHost:          func() (string, string) { return "github.com", "hosts" },
		knownHosts:           func() []string { return []string{"github.com"} },
		tokenFromEnvOrConfig: func(string) (string, string) { return "", "default" },
		tokenFromSecureStorage: func(ctx context.Context, _ string) (string, error) {
			return "", ctx.Err()
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertAuthenticationError(t *testing.T, err error, wantCause string) {
	t.Helper()
	var githubError *Error
	if !errors.As(err, &githubError) || githubError.Kind != ErrorAuthentication {
		t.Fatalf("unexpected error: %v", err)
	}
	if githubError.cause == nil || githubError.cause.Error() != wantCause {
		t.Fatalf("unexpected cause: %v", githubError.cause)
	}
}
