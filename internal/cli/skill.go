package cli

import (
	"bytes"
	"io"

	"github.com/spf13/cobra"
)

func newSkillCommand(content []byte) *cobra.Command {
	return &cobra.Command{
		Use:   "skill",
		Short: "Print the bundled gh-actionkit agent skill",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			_, err := io.Copy(command.OutOrStdout(), bytes.NewReader(content))
			return err
		},
	}
}
