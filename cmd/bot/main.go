// cmd/bot/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Ask149/iodevz-news-bot/internal/pipeline"
)

func main() {
	fmt.Println("iodevz-news-bot starting...")

	// 10-minute overall pipeline timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
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
