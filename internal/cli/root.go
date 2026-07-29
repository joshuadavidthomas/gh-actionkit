package cli

import (
	"io"

	"github.com/joshuadavidthomas/gh-actionkit/internal/tools"
	"github.com/spf13/cobra"
)

func NewRootCommand(version string, skill []byte, stdout, stderr io.Writer) *cobra.Command {
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
		newVersionCommand(lookupVersion),
		newSearchCommand(searchActions),
		newInspectCommand(inspectAction),
		newLintCommand(lintWorkflows),
		newValidateCommand(tools.Validate),
		newCheckCommand(checkActions),
		newSkillCommand(skill),
	)
	return command
}
