package cli

import (
	"fmt"
	"io"
	"time"
)

const spinnerInterval = 80 * time.Millisecond

var spinnerFrames = [...]string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type spinner struct {
	output io.Writer
	label  string
	stop   chan struct{}
	done   chan struct{}
}

func startCommandSpinner(stdout, stderr io.Writer, outputJSON bool, label string) *spinner {
	enabled := !outputJSON && outputIsTerminal(stdout) && outputIsTerminal(stderr)
	return newSpinner(stderr, label, enabled)
}

func newSpinner(output io.Writer, label string, enabled bool) *spinner {
	indicator := &spinner{output: output, label: label}
	if !enabled {
		return indicator
	}

	indicator.stop = make(chan struct{})
	indicator.done = make(chan struct{})
	indicator.render(spinnerFrames[0])
	go indicator.animate()
	return indicator
}

func (s *spinner) animate() {
	defer close(s.done)
	ticker := time.NewTicker(spinnerInterval)
	defer ticker.Stop()

	frame := 1
	for {
		select {
		case <-ticker.C:
			s.render(spinnerFrames[frame])
			frame = (frame + 1) % len(spinnerFrames)
		case <-s.stop:
			_, _ = fmt.Fprint(s.output, "\r\x1b[2K")
			return
		}
	}
}

func (s *spinner) render(frame string) {
	styles := newOutputStyles(newOutputRenderer(s.output))
	text := styles.action.Bold(true).Render(frame + " " + s.label)
	_, _ = fmt.Fprintf(s.output, "\r\x1b[2K%s", text)
}

func (s *spinner) Stop() {
	if s.stop == nil {
		return
	}
	close(s.stop)
	<-s.done
}
