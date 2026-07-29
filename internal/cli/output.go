package cli

import (
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

const fallbackTerminalWidth = 80

// sanitizeTerminal replaces terminal control characters while preserving
// newlines and tabs for callers that render multiline text.
func sanitizeTerminal(value string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\n' && r != '\t' || r >= 0x7f && r <= 0x9f {
			return '\uFFFD'
		}
		return r
	}, value)
}

// sanitizeTerminalLine replaces all terminal control characters, including
// newlines and tabs, for values rendered on one line.
func sanitizeTerminalLine(value string) string {
	value = strings.NewReplacer("\n", `\n`, "\t", `\t`).Replace(value)
	return sanitizeTerminal(value)
}

// FormatErrorForTerminal returns an error message safe to print to a terminal.
func FormatErrorForTerminal(err error) string {
	return sanitizeTerminalLine(err.Error())
}

type outputStyles struct {
	action    lipgloss.Style
	secondary lipgloss.Style
	success   lipgloss.Style
	danger    lipgloss.Style
	unknown   lipgloss.Style
}

func newOutputStyles(renderer *lipgloss.Renderer) outputStyles {
	return outputStyles{
		action:    renderer.NewStyle().Foreground(lipgloss.Color("6")),
		secondary: renderer.NewStyle().Foreground(lipgloss.Color("240")),
		success:   renderer.NewStyle().Foreground(lipgloss.Color("2")),
		danger:    renderer.NewStyle().Foreground(lipgloss.Color("1")),
		unknown:   renderer.NewStyle().Foreground(lipgloss.Color("3")),
	}
}

func newOutputRenderer(output io.Writer) *lipgloss.Renderer {
	return newRenderer(output, outputUsesColor(output))
}

func newRenderer(output io.Writer, useColor bool) *lipgloss.Renderer {
	renderer := lipgloss.NewRenderer(output)
	if useColor {
		renderer.SetColorProfile(termenv.ANSI256)
	} else {
		renderer.SetColorProfile(termenv.Ascii)
	}
	return renderer
}

func outputUsesColor(output io.Writer) bool {
	if _, disabled := os.LookupEnv("NO_COLOR"); disabled {
		return false
	}
	return outputIsTerminal(output)
}

func outputWidth(output io.Writer) int {
	file, ok := output.(interface{ Fd() uintptr })
	if !ok || !term.IsTerminal(int(file.Fd())) {
		return 0
	}
	width, _, err := term.GetSize(int(file.Fd()))
	if err != nil || width <= 0 {
		return fallbackTerminalWidth
	}
	return width
}

func outputIsTerminal(output io.Writer) bool {
	file, ok := output.(interface{ Fd() uintptr })
	return ok && term.IsTerminal(int(file.Fd()))
}
