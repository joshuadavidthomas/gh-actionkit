package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
	"github.com/joshuadavidthomas/gh-actionkit/internal/githubapi"
	"github.com/spf13/cobra"
)

type actionInspect func(context.Context, string) (actions.InspectResult, error)

func newInspectCommand() *cobra.Command {
	return newInspectCommandWithInspect(inspectAction)
}

func inspectAction(ctx context.Context, action string) (actions.InspectResult, error) {
	client, err := githubapi.New()
	if err != nil {
		return actions.InspectResult{}, fmt.Errorf("connect to GitHub: %w", err)
	}
	return actions.NewInspectService(client).Inspect(ctx, action)
}

func newInspectCommandWithInspect(inspect actionInspect) *cobra.Command {
	var outputJSON bool
	command := &cobra.Command{
		Use:   "inspect OWNER/REPO",
		Short: "Inspect a GitHub Action",
		Long:  "Show repository, manifest, input, output, runtime, and stable version details for a GitHub Action.",
		Example: "  gh actionkit inspect actions/checkout\n" +
			"  gh actionkit inspect actions/checkout --json",
		Args: cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			indicator := startCommandSpinner(
				command.OutOrStdout(),
				command.ErrOrStderr(),
				outputJSON,
				"Inspecting action...",
			)
			defer indicator.Stop()
			result, err := inspect(command.Context(), args[0])
			indicator.Stop()
			if err != nil {
				return err
			}
			if outputJSON {
				encoder := json.NewEncoder(command.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(result)
			}
			return writeInspection(command.OutOrStdout(), result)
		},
	}
	command.Flags().BoolVar(&outputJSON, "json", false, "output JSON")
	return command
}

func writeInspection(output io.Writer, result actions.InspectResult) error {
	renderer := newOutputRenderer(output)
	styles := newOutputStyles(renderer)
	actionStyle := styles.action.Bold(true)
	sectionStyle := renderer.NewStyle().Bold(true)

	if _, err := fmt.Fprintln(output, actionStyle.Render(result.Action)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(output, "  %s\n", sectionStyle.Render("repository")); err != nil {
		return err
	}
	if err := writeInspectionField(output, styles, "owner", formatOwner(result.Repository.Owner)); err != nil {
		return err
	}
	if err := writeInspectionField(output, styles, "archived", fmt.Sprint(result.Repository.Archived)); err != nil {
		return err
	}
	pushedAt := "unknown"
	if result.Repository.PushedAt != nil {
		pushedAt = result.Repository.PushedAt.Format(time.RFC3339)
	}
	if err := writeInspectionField(output, styles, "last push", pushedAt); err != nil {
		return err
	}
	license := "unknown"
	if result.Repository.License != nil {
		license = result.Repository.License.SPDXID
		if license == "" {
			license = result.Repository.License.Name
		}
	}
	if err := writeInspectionField(output, styles, "license", license); err != nil {
		return err
	}
	if err := writeInspectionField(output, styles, "url", result.Repository.URL); err != nil {
		return err
	}
	if result.Repository.Description != nil && *result.Repository.Description != "" {
		if err := writeInspectionField(output, styles, "description", *result.Repository.Description); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(output, "  %s\n", sectionStyle.Render("manifest")); err != nil {
		return err
	}
	for _, field := range [][2]string{
		{"path", result.Manifest.Path},
		{"ref", result.Manifest.Ref},
		{"name", result.Manifest.Name},
		{"description", result.Manifest.Description},
		{"runtime", result.Manifest.Runtime},
	} {
		if err := writeInspectionField(output, styles, field[0], field[1]); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(output, "  %s\n", sectionStyle.Render("inputs")); err != nil {
		return err
	}
	if len(result.Manifest.Inputs) == 0 {
		if _, err := fmt.Fprintln(output, "    none"); err != nil {
			return err
		}
	}
	for _, input := range result.Manifest.Inputs {
		details := "optional"
		if input.Required {
			details = "required"
		}
		if input.Default != nil {
			defaultValue := *input.Default
			if strings.Contains(defaultValue, "\n") {
				defaultValue = strings.ReplaceAll(defaultValue, "\n", `\n`)
			}
			details += ", default: " + defaultValue
		}
		if _, err := fmt.Fprintf(output, "    %s (%s)\n", input.Name, details); err != nil {
			return err
		}
		if input.Description != nil && *input.Description != "" {
			if _, err := fmt.Fprintf(output, "      %s\n", indentInspectionText(*input.Description, "      ")); err != nil {
				return err
			}
		}
	}

	if _, err := fmt.Fprintf(output, "  %s\n", sectionStyle.Render("outputs")); err != nil {
		return err
	}
	if len(result.Manifest.Outputs) == 0 {
		if _, err := fmt.Fprintln(output, "    none"); err != nil {
			return err
		}
	}
	for _, actionOutput := range result.Manifest.Outputs {
		if _, err := fmt.Fprintf(output, "    %s\n", actionOutput.Name); err != nil {
			return err
		}
		if actionOutput.Description != nil && *actionOutput.Description != "" {
			if _, err := fmt.Fprintf(output, "      %s\n", indentInspectionText(*actionOutput.Description, "      ")); err != nil {
				return err
			}
		}
	}

	if _, err := fmt.Fprintf(output, "  %s\n", sectionStyle.Render("latest")); err != nil {
		return err
	}
	if result.Latest == nil {
		return writeInspectionField(output, styles, "version", "unknown")
	}
	if err := writeInspectionField(output, styles, "tag", result.Latest.Tag); err != nil {
		return err
	}
	sha := "unknown"
	if result.Latest.SHA != nil {
		sha = *result.Latest.SHA
	}
	if err := writeInspectionField(output, styles, "sha", sha); err != nil {
		return err
	}
	if result.PinnedUses != nil {
		_, err := fmt.Fprintf(output, "    %s\n", *result.PinnedUses)
		return err
	}
	return nil
}

func writeInspectionField(output io.Writer, styles outputStyles, label, value string) error {
	_, err := fmt.Fprintf(
		output,
		"    %s %s\n",
		styles.secondary.Render(label+":"),
		indentInspectionText(value, "      "),
	)
	return err
}

func indentInspectionText(value, indentation string) string {
	return strings.ReplaceAll(strings.TrimSpace(value), "\n", "\n"+indentation)
}

func formatOwner(owner actions.RepositoryOwner) string {
	if owner.Type == "" {
		return owner.Login
	}
	return owner.Login + " (" + owner.Type + ")"
}
