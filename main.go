package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/joshuadavidthomas/gh-actionkit/internal/cli"
)

var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cli.NewRootCommand(version, os.Stdout, os.Stderr).ExecuteContext(ctx); err != nil {
		exitCode := 2
		var statusError interface{ ExitCode() int }
		if errors.As(err, &statusError) {
			exitCode = statusError.ExitCode()
		}
		if err.Error() != "" {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(exitCode)
	}
}
