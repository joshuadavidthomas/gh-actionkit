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

type recordingProcess struct {
	command command
	err     error
}

func (p *recordingProcess) run(_ context.Context, command command) error {
	p.command = command
	return p.err
}

type processExitError int

func (e processExitError) Error() string { return "process failed" }
func (e processExitError) ExitCode() int { return int(e) }

func TestLintBuildsCommandAndPreservesExitCode(t *testing.T) {
	process := &recordingProcess{err: processExitError(13)}
	lookPath := func(string) (string, error) {
		return "/usr/bin/zizmor", nil
	}

	exitCode, err := lint(
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
		process.run,
		lookPath,
	)
	if err != nil {
		t.Fatal(err)
	}
	if exitCode != 13 {
		t.Fatalf("got exit code %d", exitCode)
	}
	if process.command.path != "/usr/bin/zizmor" || process.command.dir != "/repo" {
		t.Fatalf("path=%q dir=%q", process.command.path, process.command.dir)
	}
	wantArgs := []string{"--collect=workflows", "--no-progress", "--format=json-v1", "--pedantic", "."}
	if !reflect.DeepEqual(process.command.args, wantArgs) {
		t.Fatalf("got arguments %#v", process.command.args)
	}
	wantEnv := []string{"GH_HOST=github.example.com", "GH_TOKEN=secret-token"}
	if !reflect.DeepEqual(process.command.env, wantEnv) {
		t.Fatalf("got environment %#v", process.command.env)
	}
	if !reflect.DeepEqual(process.command.unsetEnv, zizmorEnvironment) {
		t.Fatalf("got environment removals %#v", process.command.unsetEnv)
	}
	for _, argument := range process.command.args {
		if argument == "secret-token" {
			t.Fatal("token must not appear in command arguments")
		}
	}
}

func TestLintPreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	process := &recordingProcess{err: errors.New("process killed")}
	lookPath := func(string) (string, error) {
		return "/usr/bin/zizmor", nil
	}

	_, err := lint(ctx, "/repo", ZizmorOptions{}, io.Discard, io.Discard, process.run, lookPath)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLintFallsBackToUV(t *testing.T) {
	process := &recordingProcess{}
	var lookups []string
	lookPath := func(name string) (string, error) {
		lookups = append(lookups, name)
		if name == "uv" {
			return "/usr/bin/uv", nil
		}
		return "", exec.ErrNotFound
	}

	exitCode, err := lint(context.Background(), "/repo", ZizmorOptions{}, io.Discard, io.Discard, process.run, lookPath)
	if err != nil {
		t.Fatal(err)
	}
	if exitCode != 0 {
		t.Fatalf("got exit code %d", exitCode)
	}
	if !reflect.DeepEqual(lookups, []string{"zizmor", "uv"}) {
		t.Fatalf("got executable lookups %#v", lookups)
	}
	if process.command.path != "/usr/bin/uv" {
		t.Fatalf("got command path %q", process.command.path)
	}
	wantArgs := []string{
		"tool", "run", "--no-config", "--no-progress", "--default-index", uvPythonIndex,
		"--from", strings.TrimSpace(zizmorRequirement), "zizmor",
		"--collect=workflows", "--no-progress", "--offline", ".",
	}
	if !reflect.DeepEqual(process.command.args, wantArgs) {
		t.Fatalf("got arguments %#v", process.command.args)
	}
	if len(process.command.env) != 0 || !reflect.DeepEqual(process.command.unsetEnv, zizmorEnvironment) {
		t.Fatalf("unexpected offline environment: %#v", process.command)
	}
}

func TestLintUVFallbackUsesPinnedPackageOnline(t *testing.T) {
	process := &recordingProcess{}
	lookPath := func(name string) (string, error) {
		if name == "uv" {
			return "/usr/bin/uv", nil
		}
		return "", exec.ErrNotFound
	}
	credentials := &GitHubCredentials{Host: "github.com", Token: "secret-token"}

	_, err := lint(
		context.Background(),
		"/repo",
		ZizmorOptions{GitHub: credentials},
		io.Discard,
		io.Discard,
		process.run,
		lookPath,
	)
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{
		"tool", "run", "--no-config", "--no-progress", "--default-index", uvPythonIndex,
		"--from", strings.TrimSpace(zizmorRequirement), "zizmor",
		"--collect=workflows", "--no-progress", ".",
	}
	if !reflect.DeepEqual(process.command.args, wantArgs) {
		t.Fatalf("got arguments %#v", process.command.args)
	}
	wantEnv := []string{"GH_HOST=github.com", "GH_TOKEN=secret-token"}
	if !reflect.DeepEqual(process.command.env, wantEnv) {
		t.Fatalf("got environment %#v", process.command.env)
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

func TestLintReportsMissingExecutableAndFallback(t *testing.T) {
	var lookups []string
	lookPath := func(name string) (string, error) {
		lookups = append(lookups, name)
		return "", exec.ErrNotFound
	}

	_, err := lint(context.Background(), "/repo", ZizmorOptions{}, io.Discard, io.Discard, (&recordingProcess{}).run, lookPath)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !reflect.DeepEqual(lookups, []string{"zizmor", "uv"}) {
		t.Fatalf("got executable lookups %#v", lookups)
	}
}

func TestLintDoesNotHideLookupErrors(t *testing.T) {
	lookupErr := errors.New("permission denied")
	var lookups []string
	lookPath := func(name string) (string, error) {
		lookups = append(lookups, name)
		return "", lookupErr
	}

	_, err := lint(context.Background(), "/repo", ZizmorOptions{}, io.Discard, io.Discard, (&recordingProcess{}).run, lookPath)
	if !errors.Is(err, lookupErr) {
		t.Fatalf("got error %v", err)
	}
	if !reflect.DeepEqual(lookups, []string{"zizmor"}) {
		t.Fatalf("got executable lookups %#v", lookups)
	}
}
