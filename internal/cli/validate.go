package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

type WorkflowValidate func(string, bool, io.Writer, io.Writer) (files int, findings int, err error)

func newValidateCommand(validate WorkflowValidate) *cobra.Command {
	var repository string
	var outputJSON bool

	command := &cobra.Command{
		Use:   "validate",
		Short: "Validate GitHub Actions workflow syntax with actionlint",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			repositoryPath, err := resolveRepository(repository)
			if err != nil {
				return err
			}
			files, findings, err := validate(
				repositoryPath,
				outputJSON,
				command.OutOrStdout(),
				command.ErrOrStderr(),
			)
			if err != nil {
				return fmt.Errorf("validate workflows: %w", err)
			}
			if files == 0 {
				output := command.OutOrStdout()
				if outputJSON {
					output = command.ErrOrStderr()
				}
				_, err := fmt.Fprintln(output, "No workflow files found in .github/workflows")
				return err
			}
			if findings > 0 {
				return StatusError{Code: 1}
			}
			return nil
		},
	}
	command.Flags().StringVarP(&repository, "repo", "C", ".", "repository path to validate")
	command.Flags().BoolVar(&outputJSON, "json", false, "output actionlint JSON Lines")
	return command
}
