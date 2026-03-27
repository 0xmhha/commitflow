package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/0xmhha/commitflow/cmd"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Make the signal context available to cobra commands
	cmd.SetContext(ctx)

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
