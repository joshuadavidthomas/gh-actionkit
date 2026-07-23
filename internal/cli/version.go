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
	renderer := newOutputRenderer(output)
	styles := newOutputStyles(renderer)
	actionStyle := styles.action.Bold(true)
	sectionStyle := renderer.NewStyle().Bold(true)

	if _, err := fmt.Fprintln(output, actionStyle.Render(info.Action)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "  %s\n", sectionStyle.Render("major")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "    %s %s\n", styles.secondary.Render("tag:"), info.Major.Tag); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "    %s %s\n", styles.secondary.Render("sha:"), styleSHA(info.Major.SHA, styles)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "  %s\n", sectionStyle.Render("latest")); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "    %s %s\n", styles.secondary.Render("tag:"), info.Latest.Tag); err != nil {
		return err
	}
	_, err := fmt.Fprintf(output, "    %s %s\n", styles.secondary.Render("sha:"), styleSHA(info.Latest.SHA, styles))
	return err
}

func styleSHA(sha *string, styles outputStyles) string {
	if sha == nil {
		return styles.unknown.Render("unknown")
	}
	return *sha
}
