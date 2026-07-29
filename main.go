package main

import (
	"context"
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

	if err := cli.NewRootCommand(version, bundledSkill, os.Stdout, os.Stderr).ExecuteContext(ctx); err != nil {
		if status, ok := cli.ExitStatus(err); ok {
			os.Exit(status)
		}
		_, _ = fmt.Fprintln(os.Stderr, cli.FormatErrorForTerminal(err))
		os.Exit(2)
	}
}
