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

type versionLookup func(context.Context, actions.Repository) (actions.RepositoryVersions, error)

type versionOutput struct {
	Action string          `json:"action"`
	Major  actions.Version `json:"major"`
	Latest actions.Version `json:"latest"`
}

func lookupVersion(ctx context.Context, repository actions.Repository) (actions.RepositoryVersions, error) {
	client, err := githubapi.New()
	if err != nil {
		return actions.RepositoryVersions{}, fmt.Errorf("connect to GitHub: %w", err)
	}
	return actions.LookupVersions(ctx, client, repository)
}

func newVersionCommand(lookup versionLookup) *cobra.Command {
	var outputJSON bool
	var outputSnippet bool
	command := &cobra.Command{
		Use:   "version OWNER/REPO[/PATH]",
		Short: "Show the latest stable version of a GitHub Action",
		Long:  "Show the latest stable release, major tag, and commit SHAs for pinning a GitHub Action.",
		Example: "  gh actionkit version actions/checkout\n" +
			"  gh actionkit version github/codeql-action/init --snippet\n" +
			"  gh actionkit version actions/checkout --json",
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			identifier, err := actions.ParseActionIdentifier(args[0])
			if err != nil {
				return err
			}
			indicator := startCommandSpinner(
				command.OutOrStdout(),
				command.ErrOrStderr(),
				outputJSON,
				"Fetching action version...",
			)
			versions, err := lookup(command.Context(), identifier.Repository())
			indicator.Stop()
			if err != nil {
				return err
			}
			result := versionOutput{
				Action: identifier.String(),
				Major:  versions.Major,
				Latest: versions.Latest,
			}
			if outputJSON {
				encoder := json.NewEncoder(command.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(result)
			}
			if outputSnippet {
				return writeVersionSnippet(command.OutOrStdout(), result)
			}
			return writeVersion(command.OutOrStdout(), result)
		},
	}
	command.Flags().BoolVar(&outputJSON, "json", false, "output JSON")
	command.Flags().BoolVar(&outputSnippet, "snippet", false, "output a pinned uses line for the latest stable version")
	command.MarkFlagsMutuallyExclusive("json", "snippet")
	return command
}

func writeVersionSnippet(output io.Writer, info versionOutput) error {
	sha := info.Latest.PinnedSHA()
	if sha == nil {
		return fmt.Errorf(
			"cannot write snippet for %s: tag %s does not resolve to a full commit SHA",
			info.Action,
			info.Latest.Tag,
		)
	}
	_, err := fmt.Fprintf(output, "uses: %s@%s # %s\n", info.Action, *sha, info.Latest.Tag)
	return err
}

func writeVersion(output io.Writer, info versionOutput) error {
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
