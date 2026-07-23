package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
	"github.com/spf13/cobra"
)

type VersionLookup func(context.Context, string) (actions.VersionInfo, error)

func NewRootCommand(stdout, stderr io.Writer, lookup VersionLookup) *cobra.Command {
	command := &cobra.Command{
		Use:           "actionkit",
		Short:         "Find, check, and validate GitHub Actions",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	command.SetOut(stdout)
	command.SetErr(stderr)
	command.AddCommand(newVersionCommand(lookup))
	return command
}

func newVersionCommand(lookup VersionLookup) *cobra.Command {
	var outputJSON bool
	command := &cobra.Command{
		Use:   "version OWNER/REPO",
		Short: "Show the latest stable version of a GitHub Action",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			info, err := lookup(command.Context(), args[0])
			if err != nil {
				return err
			}
			if outputJSON {
				encoder := json.NewEncoder(command.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(info)
			}
			writeVersion(command.OutOrStdout(), info)
			return nil
		},
	}
	command.Flags().BoolVar(&outputJSON, "json", false, "output JSON")
	return command
}

func writeVersion(output io.Writer, info actions.VersionInfo) {
	fmt.Fprintln(output, info.Action)
	fmt.Fprintln(output, "  major")
	fmt.Fprintf(output, "    tag: %s\n", info.Major.Tag)
	fmt.Fprintf(output, "    sha: %s\n", formatSHA(info.Major.SHA))
	fmt.Fprintln(output, "  latest")
	fmt.Fprintf(output, "    tag: %s\n", info.Latest.Tag)
	fmt.Fprintf(output, "    sha: %s\n", formatSHA(info.Latest.SHA))
}

func formatSHA(sha *string) string {
	if sha == nil {
		return "unknown"
	}
	return *sha
}
