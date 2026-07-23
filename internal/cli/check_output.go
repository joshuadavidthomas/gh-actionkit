package cli

import (
	"io"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	lipglosstable "github.com/charmbracelet/lipgloss/table"
	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
)

const displayedSHALength = 12

var checkOutputHeaders = []string{"Action", "Status", "Workflows", "Used", "Major", "Latest"}

type checkOutputStyles struct {
	outputStyles
	header lipgloss.Style
	cell   lipgloss.Style
}

func renderCheckResults(results []actions.CheckResult, width int, useColor bool) string {
	renderer := newRenderer(io.Discard, useColor)
	styles := checkOutputStyles{
		outputStyles: newOutputStyles(renderer),
		header:       renderer.NewStyle().Bold(true).Padding(0, 1),
		cell:         renderer.NewStyle().Padding(0, 1),
	}
	rows := formatCheckRows(results, styles)

	table := lipglosstable.New().
		Headers(checkOutputHeaders...).
		Rows(rows...).
		Border(lipgloss.RoundedBorder()).
		BorderStyle(styles.secondary).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == lipglosstable.HeaderRow {
				return styles.header
			}
			return styles.cell
		})

	rendered := table.String()
	if fillsTerminalWidth(rendered, width) {
		rendered = renderCheckCards(rows, width, styles)
	}

	titleStyle := renderer.NewStyle().Bold(true)
	if width > 1 {
		titleStyle = titleStyle.MaxWidth(width - 1)
	}
	return titleStyle.Render("GitHub Actions workflow versions") + "\n" + rendered
}

func formatCheckRows(results []actions.CheckResult, styles checkOutputStyles) [][]string {
	rows := make([][]string, 0, len(results))
	for _, result := range results {
		rows = append(rows, []string{
			styles.action.Render(result.Action),
			formatCheckStatus(styles, result),
			formatCheckLocations(result),
			styleCheckVersion(styles, result.Used),
			styleCheckVersion(styles, result.Major),
			styleCheckVersion(styles, result.Latest),
		})
	}
	return rows
}

func renderCheckCards(rows [][]string, width int, styles checkOutputStyles) string {
	cards := make([]string, 0, len(rows))
	for _, row := range rows {
		fields := make([][]string, len(checkOutputHeaders))
		for index, header := range checkOutputHeaders {
			fields[index] = []string{header, row[index]}
		}
		card := lipglosstable.New().
			Rows(fields...).
			Border(lipgloss.RoundedBorder()).
			BorderStyle(styles.secondary).
			StyleFunc(func(_ int, column int) lipgloss.Style {
				if column == 0 {
					return styles.header
				}
				return styles.cell
			})
		rendered := card.String()
		if width > 1 && fillsTerminalWidth(rendered, width) {
			card.Width(width - 1)
			rendered = card.String()
		}
		cards = append(cards, rendered)
	}
	return strings.Join(cards, "\n\n")
}

func fillsTerminalWidth(rendered string, width int) bool {
	// Keep the final column empty so the following newline cannot auto-wrap.
	return width > 0 && lipgloss.Width(strings.SplitN(rendered, "\n", 2)[0]) >= width
}

func formatCheckLocations(result actions.CheckResult) string {
	locations := make([]string, 0, len(result.Locations))
	for _, location := range result.Locations {
		locations = append(locations, location.File+":"+strconv.Itoa(location.Line))
	}
	return strings.Join(locations, "\n")
}

func formatCheckStatus(styles checkOutputStyles, result actions.CheckResult) string {
	switch {
	case result.UpToDate:
		return styles.success.Render("up to date")
	case result.UpdateAvailable:
		return styles.danger.Render("update available")
	default:
		return styles.unknown.Render("unknown")
	}
}

func styleCheckVersion(styles checkOutputStyles, version actions.CheckVersion) string {
	switch {
	case version.Tag != nil && version.SHA != nil:
		return *version.Tag + "\n" + styles.secondary.Render(formatCheckSHA(*version.SHA))
	case version.Tag != nil:
		return *version.Tag + "\n" + styles.unknown.Render("unknown")
	case version.SHA != nil:
		return styles.secondary.Render(formatCheckSHA(*version.SHA))
	default:
		return styles.unknown.Render("unknown")
	}
}

func formatCheckSHA(sha string) string {
	if len(sha) <= displayedSHALength {
		return sha
	}
	return sha[:displayedSHALength]
}
