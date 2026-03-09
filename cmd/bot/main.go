// cmd/bot/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Ask149/iodevz-news-bot/internal/pipeline"
)

func main() {
	fmt.Println("iodevz-news-bot starting...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		cancel()
	}()

	cfg := pipeline.DefaultConfig()

	// Override from env.
	if os.Getenv("DRY_RUN") == "true" {
		cfg.DryRun = true
	}

	if err := pipeline.Run(ctx, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
