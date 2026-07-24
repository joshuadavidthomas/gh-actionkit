package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/joshuadavidthomas/gh-actionkit/internal/githubapi"
	"github.com/joshuadavidthomas/gh-actionkit/internal/tools"
	"github.com/spf13/cobra"
)

type lintOptions struct {
	OutputJSON bool
	Pedantic   bool
	Offline    bool
}

type workflowLint func(context.Context, string, lintOptions, io.Writer, io.Writer) (int, error)
type credentialResolver func(context.Context) (githubapi.Credentials, error)
type zizmorLint func(context.Context, string, tools.ZizmorOptions, io.Writer, io.Writer) (int, error)

func newLintCommand() *cobra.Command {
	return newLintCommandWithLint(lintWorkflows)
}

func lintWorkflows(
	ctx context.Context,
	repository string,
	options lintOptions,
	stdout io.Writer,
	stderr io.Writer,
) (int, error) {
	return lintWorkflowsWith(
		ctx,
		repository,
		options,
		stdout,
		stderr,
		githubapi.ResolveCredentials,
		tools.NewZizmor().Lint,
	)
}

func lintWorkflowsWith(
	ctx context.Context,
	repository string,
	options lintOptions,
	stdout io.Writer,
	stderr io.Writer,
	resolveCredentials credentialResolver,
	lint zizmorLint,
) (int, error) {
	zizmorOptions := tools.ZizmorOptions{
		OutputJSON: options.OutputJSON,
		Pedantic:   options.Pedantic,
	}
	if !options.Offline {
		credentials, err := resolveCredentials(ctx)
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		if err != nil {
			return 0, fmt.Errorf("authenticate for zizmor online audits: %w; use --offline to run without GitHub API access", err)
		}
		zizmorOptions.GitHub = &tools.GitHubCredentials{
			Host:  credentials.Host,
			Token: credentials.Token,
		}
	}
	return lint(ctx, repository, zizmorOptions, stdout, stderr)
}

func newLintCommandWithLint(lint workflowLint) *cobra.Command {
	var repository string
	var outputJSON bool
	var pedantic bool
	var noPedantic bool
	var offline bool

	command := &cobra.Command{
		Use:   "lint",
		Short: "Audit GitHub Actions workflows with zizmor",
		Long:  "Run zizmor security audits against a repository's workflow collection, including stale, vulnerable, and unpinned Action refs.",
		Example: "  gh actionkit lint\n" +
			"  gh actionkit lint --pedantic\n" +
			"  gh actionkit lint --offline",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			repositoryPath, err := resolveRepository(repository)
			if err != nil {
				return err
			}
			if noPedantic {
				pedantic = false
			}
			exitCode, err := lint(
				command.Context(),
				repositoryPath,
				lintOptions{
					OutputJSON: outputJSON,
					Pedantic:   pedantic,
					Offline:    offline,
				},
				command.OutOrStdout(),
				command.ErrOrStderr(),
			)
			if err != nil {
				return err
			}
			if exitCode != 0 {
				return StatusError{Code: exitCode}
			}
			return nil
		},
	}
	command.Flags().StringVarP(&repository, "repo", "C", ".", "repository path to inspect")
	command.Flags().BoolVar(&outputJSON, "json", false, "output zizmor JSON")
	command.Flags().BoolVar(&pedantic, "pedantic", false, "enable zizmor pedantic audits")
	command.Flags().BoolVar(&noPedantic, "no-pedantic", false, "disable zizmor pedantic audits")
	command.Flags().BoolVar(&offline, "offline", false, "disable zizmor online audits")
	command.MarkFlagsMutuallyExclusive("pedantic", "no-pedantic")
	return command
}
