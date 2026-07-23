package cli

import (
	"context"
	"io"

	"github.com/joshuadavidthomas/gh-actionkit/internal/tools"
	"github.com/spf13/cobra"
)

type workflowLint func(context.Context, string, bool, bool, io.Writer, io.Writer) (int, error)

func newLintCommand() *cobra.Command {
	return newLintCommandWithLint(tools.NewZizmor().Lint)
}

func newLintCommandWithLint(lint workflowLint) *cobra.Command {
	var repository string
	var outputJSON bool
	var pedantic bool
	var noPedantic bool

	command := &cobra.Command{
		Use:   "lint",
		Short: "Audit GitHub Actions workflows with zizmor",
		Long:  "Run zizmor security audits against a repository's workflow collection, including stale, vulnerable, and unpinned Action refs.",
		Example: "  gh actionkit lint\n" +
			"  gh actionkit lint --pedantic",
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
				outputJSON,
				pedantic,
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
	command.MarkFlagsMutuallyExclusive("pedantic", "no-pedantic")
	return command
}
