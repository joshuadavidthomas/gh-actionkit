package githubapi

import (
	"context"
	"fmt"
	"strings"

	gh "github.com/cli/go-gh/v2"
	"github.com/cli/go-gh/v2/pkg/auth"
)

type Credentials struct {
	Host  string
	Token string
}

type credentialSource struct {
	defaultHost            func() (string, string)
	knownHosts             func() []string
	tokenFromEnvOrConfig   func(string) (string, string)
	tokenFromSecureStorage func(context.Context, string) (string, error)
}

func ResolveCredentials(ctx context.Context) (Credentials, error) {
	return resolveCredentials(ctx, credentialSource{
		defaultHost:            auth.DefaultHost,
		knownHosts:             auth.KnownHosts,
		tokenFromEnvOrConfig:   auth.TokenFromEnvOrConfig,
		tokenFromSecureStorage: secureTokenForHost,
	})
}

func resolveCredentials(ctx context.Context, source credentialSource) (Credentials, error) {
	if err := ctx.Err(); err != nil {
		return Credentials{}, err
	}
	host, hostSource := source.defaultHost()
	if host == "" {
		return Credentials{}, authenticationError(fmt.Errorf("GitHub host not found"))
	}
	if hostSource != "GH_HOST" && len(source.knownHosts()) > 1 {
		return Credentials{}, authenticationError(fmt.Errorf("multiple authenticated GitHub hosts; set GH_HOST"))
	}

	token, _ := source.tokenFromEnvOrConfig(host)
	if token == "" {
		var err error
		token, err = source.tokenFromSecureStorage(ctx, host)
		if ctx.Err() != nil {
			return Credentials{}, ctx.Err()
		}
		if err != nil {
			return Credentials{}, authenticationError(err)
		}
	}
	if token == "" {
		return Credentials{}, authenticationError(fmt.Errorf("authentication token not found for %s", host))
	}
	return Credentials{Host: host, Token: token}, nil
}

func secureTokenForHost(ctx context.Context, host string) (string, error) {
	stdout, stderr, err := gh.ExecContext(ctx, "auth", "token", "--secure-storage", "--hostname", host)
	if err != nil {
		if detail := strings.TrimSpace(stderr.String()); detail != "" {
			return "", fmt.Errorf("read token from secure storage: %s: %w", detail, err)
		}
		return "", fmt.Errorf("read token from secure storage: %w", err)
	}
	return strings.TrimSpace(stdout.String()), nil
}
