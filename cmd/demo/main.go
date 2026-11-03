package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/feat7/ingestkit/internal/runenv"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	recovery := flag.Bool("recovery", false, "stop PostgreSQL during ingestion")
	cleanup := flag.Bool("cleanup", false, "destroy only this demo's own resources afterward")
	out := flag.String("out", "", "evidence directory")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := runenv.RunDemo(ctx, runenv.DemoOptions{Recovery: *recovery, Cleanup: *cleanup, OutputDirectory: *out}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
