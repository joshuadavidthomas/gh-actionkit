package tools

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

type recordingRunner struct {
	command Command
	err     error
}

func (r *recordingRunner) Run(_ context.Context, command Command) error {
	r.command = command
	return r.err
}

type processExitError int

func (e processExitError) Error() string { return "process failed" }
func (e processExitError) ExitCode() int { return int(e) }

func TestZizmorBuildsCommandAndPreservesExitCode(t *testing.T) {
	runner := &recordingRunner{err: processExitError(13)}
	zizmor := Zizmor{
		runner: runner,
		lookPath: func(string) (string, error) {
			return "/usr/bin/zizmor", nil
		},
	}

	exitCode, err := zizmor.Lint(
		context.Background(),
		"/repo",
		ZizmorOptions{
			OutputJSON: true,
			Pedantic:   true,
			GitHub: &GitHubCredentials{
				Host:  "github.example.com",
				Token: "secret-token",
			},
		},
		io.Discard,
		io.Discard,
	)
	if err != nil {
		t.Fatal(err)
	}
	if exitCode != 13 {
		t.Fatalf("got exit code %d", exitCode)
	}
	if runner.command.Path != "/usr/bin/zizmor" || runner.command.Dir != "/repo" {
		t.Fatalf("path=%q dir=%q", runner.command.Path, runner.command.Dir)
	}
	wantArgs := []string{"--collect=workflows", "--no-progress", "--format=json-v1", "--pedantic", "."}
	if !reflect.DeepEqual(runner.command.Args, wantArgs) {
		t.Fatalf("got arguments %#v", runner.command.Args)
	}
	wantEnv := []string{"GH_HOST=github.example.com", "GH_TOKEN=secret-token"}
	if !reflect.DeepEqual(runner.command.Env, wantEnv) {
		t.Fatalf("got environment %#v", runner.command.Env)
	}
	if !reflect.DeepEqual(runner.command.UnsetEnv, zizmorEnvironment) {
		t.Fatalf("got environment removals %#v", runner.command.UnsetEnv)
	}
	for _, argument := range runner.command.Args {
		if argument == "secret-token" {
			t.Fatal("token must not appear in command arguments")
		}
	}
}

func TestZizmorPreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runner := &recordingRunner{err: errors.New("process killed")}
	zizmor := Zizmor{
		runner: runner,
		lookPath: func(string) (string, error) {
			return "/usr/bin/zizmor", nil
		},
	}

	_, err := zizmor.Lint(ctx, "/repo", ZizmorOptions{}, io.Discard, io.Discard)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestZizmorFallsBackToUV(t *testing.T) {
	runner := &recordingRunner{}
	var lookups []string
	zizmor := Zizmor{
		runner: runner,
		lookPath: func(name string) (string, error) {
			lookups = append(lookups, name)
			if name == "uv" {
				return "/usr/bin/uv", nil
			}
			return "", exec.ErrNotFound
		},
	}

	exitCode, err := zizmor.Lint(context.Background(), "/repo", ZizmorOptions{}, io.Discard, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if exitCode != 0 {
		t.Fatalf("got exit code %d", exitCode)
	}
	if !reflect.DeepEqual(lookups, []string{"zizmor", "uv"}) {
		t.Fatalf("got executable lookups %#v", lookups)
	}
	if runner.command.Path != "/usr/bin/uv" {
		t.Fatalf("got command path %q", runner.command.Path)
	}
	wantArgs := []string{
		"tool", "run", "--no-config", "--no-progress", "--default-index", uvPythonIndex,
		"--from", strings.TrimSpace(zizmorRequirement), "zizmor",
		"--collect=workflows", "--no-progress", "--offline", ".",
	}
	if !reflect.DeepEqual(runner.command.Args, wantArgs) {
		t.Fatalf("got arguments %#v", runner.command.Args)
	}
	if len(runner.command.Env) != 0 || !reflect.DeepEqual(runner.command.UnsetEnv, zizmorEnvironment) {
		t.Fatalf("unexpected offline environment: %#v", runner.command)
	}
}

func TestZizmorUVFallbackUsesPinnedPackageOnline(t *testing.T) {
	runner := &recordingRunner{}
	zizmor := Zizmor{
		runner: runner,
		lookPath: func(name string) (string, error) {
			if name == "uv" {
				return "/usr/bin/uv", nil
			}
			return "", exec.ErrNotFound
		},
	}
	credentials := &GitHubCredentials{Host: "github.com", Token: "secret-token"}

	_, err := zizmor.Lint(
		context.Background(),
		"/repo",
		ZizmorOptions{GitHub: credentials},
		io.Discard,
		io.Discard,
	)
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{
		"tool", "run", "--no-config", "--no-progress", "--default-index", uvPythonIndex,
		"--from", strings.TrimSpace(zizmorRequirement), "zizmor",
		"--collect=workflows", "--no-progress", ".",
	}
	if !reflect.DeepEqual(runner.command.Args, wantArgs) {
		t.Fatalf("got arguments %#v", runner.command.Args)
	}
	wantEnv := []string{"GH_HOST=github.com", "GH_TOKEN=secret-token"}
	if !reflect.DeepEqual(runner.command.Env, wantEnv) {
		t.Fatalf("got environment %#v", runner.command.Env)
	}
}

func TestCommandEnvironmentRemovesSecretsAndModeOverrides(t *testing.T) {
	environment := commandEnvironment(
		[]string{
			"PATH=/usr/bin",
			"GH_TOKEN=old-token",
			"GITHUB_TOKEN=other-token",
			"ZIZMOR_OFFLINE=true",
		},
		zizmorEnvironment,
		[]string{"GH_TOKEN=new-token"},
	)
	want := []string{"PATH=/usr/bin", "GH_TOKEN=new-token"}
	if !reflect.DeepEqual(environment, want) {
		t.Fatalf("got environment %#v", environment)
	}
}

func TestZizmorReportsMissingExecutableAndFallback(t *testing.T) {
	var lookups []string
	zizmor := Zizmor{
		runner: &recordingRunner{},
		lookPath: func(name string) (string, error) {
			lookups = append(lookups, name)
			return "", exec.ErrNotFound
		},
	}

	_, err := zizmor.Lint(context.Background(), "/repo", ZizmorOptions{}, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !reflect.DeepEqual(lookups, []string{"zizmor", "uv"}) {
		t.Fatalf("got executable lookups %#v", lookups)
	}
}

func TestZizmorDoesNotHideLookupErrors(t *testing.T) {
	lookupErr := errors.New("permission denied")
	var lookups []string
	zizmor := Zizmor{
		runner: &recordingRunner{},
		lookPath: func(name string) (string, error) {
			lookups = append(lookups, name)
			return "", lookupErr
		},
	}

	_, err := zizmor.Lint(context.Background(), "/repo", ZizmorOptions{}, io.Discard, io.Discard)
	if !errors.Is(err, lookupErr) {
		t.Fatalf("got error %v", err)
	}
	if !reflect.DeepEqual(lookups, []string{"zizmor"}) {
		t.Fatalf("got executable lookups %#v", lookups)
	}
}
