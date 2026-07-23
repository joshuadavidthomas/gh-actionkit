package cli

import (
	"io"

	"github.com/spf13/cobra"
)

func NewRootCommand(version string, stdout, stderr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:           "actionkit",
		Short:         "Find, check, and validate GitHub Actions",
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	command.SetOut(stdout)
	command.SetErr(stderr)
	command.AddCommand(
		newVersionCommand(),
		newSearchCommand(),
		newLintCommand(),
		newValidateCommand(),
		newCheckCommand(),
	)
	return command
}
