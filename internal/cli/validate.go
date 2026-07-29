package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/joshuadavidthomas/gh-actionkit/internal/tools"
	"github.com/spf13/cobra"
)

type workflowValidate func(context.Context, string, bool, io.Writer, io.Writer) (tools.ValidationResult, error)

func newValidateCommand(validate workflowValidate) *cobra.Command {
	var repository string
	var outputJSON bool

	command := &cobra.Command{
		Use:   "validate",
		Short: "Validate GitHub Actions workflow syntax with actionlint",
		Long:  "Validate workflow syntax, expressions, context availability, and Action inputs with the embedded actionlint library.",
		Example: "  gh actionkit validate\n" +
			"  gh actionkit validate --json",
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			repositoryPath, err := resolveRepository(repository)
			if err != nil {
				return err
			}
			result, err := validate(
				command.Context(),
				repositoryPath,
				outputJSON,
				command.OutOrStdout(),
				command.ErrOrStderr(),
			)
			if err != nil {
				return fmt.Errorf("validate workflows: %w", err)
			}
			if result.Files == 0 {
				output := command.OutOrStdout()
				if outputJSON {
					output = command.ErrOrStderr()
				}
				_, err := fmt.Fprintln(output, "No workflow files found in .github/workflows")
				return err
			}
			if result.Findings > 0 {
				return exitStatusError(1)
			}
			return nil
		},
	}
	command.Flags().StringVarP(&repository, "repo", "C", ".", "repository path to validate")
	command.Flags().BoolVar(&outputJSON, "json", false, "output actionlint JSON Lines")
	return command
}
