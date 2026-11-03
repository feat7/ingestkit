package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/feat7/ingestkit/internal/benchmark"
	"github.com/feat7/ingestkit/internal/runenv"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() (result error) {
	count := flag.Int("count", 10000, "database submissions per method/repetition")
	size := flag.Int("payload", 256, "encoded payload bytes")
	batch := flag.Int("batch", 100, "events per database batch")
	seed := flag.Int64("seed", 42, "deterministic workload seed")
	duplicates := flag.Float64("duplicates", .1, "fraction of submissions repeating earlier IDs")
	out := flag.String("out", "", "report directory (default own run directory)")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	s, e := runenv.Start(ctx, runenv.Options{Purpose: "bench-db"})
	if e != nil {
		return e
	}
	dir := *out
	if dir == "" {
		dir = s.Directory
	}
	fmt.Println("Evidence:", dir)
	defer func() {
		end, c := context.WithTimeout(context.Background(), 30*time.Second)
		defer c()
		result = join(result, s.CollectEvidence(end, s.Directory))
		result = join(result, s.Destroy(end))
	}()
	p, e := pgxpool.New(ctx, s.DatabaseURL)
	if e != nil {
		return e
	}
	defer p.Close()
	r, e := benchmark.RunDatabase(ctx, benchmark.DatabaseConfig{Pool: p, Workload: benchmark.WorkloadConfig{Seed: *seed, Count: *count, PayloadBytes: *size, DuplicateFraction: *duplicates}, BatchSize: *batch, Repetitions: 3})
	if e != nil {
		return e
	}
	if e = benchmark.WriteReport(dir, r); e != nil {
		return e
	}
	for _, v := range r.Summaries() {
		fmt.Printf("%s: median %.3fs, %.0f submitted/s\n", v.Method, float64(v.MedianElapsedNS)/1e9, v.MedianSubmittedEPS)
	}
	return nil
}
func join(a, b error) error {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return fmt.Errorf("%w; %v", a, b)
}
