package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
	"github.com/joshuadavidthomas/gh-actionkit/internal/githubapi"
	"github.com/spf13/cobra"
)

type versionLookup func(context.Context, string) (actions.VersionInfo, error)

func newVersionCommand() *cobra.Command {
	return newVersionCommandWithLookup(lookupVersion)
}

func lookupVersion(ctx context.Context, action string) (actions.VersionInfo, error) {
	client, err := githubapi.New()
	if err != nil {
		return actions.VersionInfo{}, fmt.Errorf("connect to GitHub: %w", err)
	}
	return actions.NewVersionService(client).Lookup(ctx, action)
}

func newVersionCommandWithLookup(lookup versionLookup) *cobra.Command {
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
			return writeVersion(command.OutOrStdout(), info)
		},
	}
	command.Flags().BoolVar(&outputJSON, "json", false, "output JSON")
	return command
}

func writeVersion(output io.Writer, info actions.VersionInfo) error {
	if _, err := fmt.Fprintln(output, info.Action); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(output, "  major"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "    tag: %s\n", info.Major.Tag); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "    sha: %s\n", formatSHA(info.Major.SHA)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(output, "  latest"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "    tag: %s\n", info.Latest.Tag); err != nil {
		return err
	}
	_, err := fmt.Fprintf(output, "    sha: %s\n", formatSHA(info.Latest.SHA))
	return err
}

func formatSHA(sha *string) string {
	if sha == nil {
		return "unknown"
	}
	return *sha
}
