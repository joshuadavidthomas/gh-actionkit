package cli

import (
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

const fallbackTerminalWidth = 80

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
	file, ok := outputFile(output)
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
	file, ok := outputFile(output)
	return ok && term.IsTerminal(int(file.Fd()))
}

func outputFile(output io.Writer) (interface{ Fd() uintptr }, bool) {
	file, ok := output.(interface{ Fd() uintptr })
	return file, ok
}
