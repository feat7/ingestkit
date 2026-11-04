package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/feat7/ingestkit/internal/benchmark"
	"github.com/feat7/ingestkit/internal/runenv"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error { return runArgs(os.Args[1:]) }
func measuredOptions(mode string) runenv.Options {
	return runenv.Options{Purpose: "bench-" + mode, Profile: "selfhost", ExposeDependencies: true}
}
func runArgs(args []string) (result error) {
	flags := flag.NewFlagSet("bench", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	mode := flags.String("mode", "database", "database, pipeline, study or calibrate")
	pattern := flags.String("pattern", "steady", "steady or burst (same count in first quarter of window)")
	count := flags.Int("count", 10000, "database submissions per method/repetition")
	size := flags.Int("payload", 256, "encoded payload bytes")
	batch := flags.Int("batch", 100, "events per database/HTTP batch")
	seed := flags.Int64("seed", 42, "deterministic workload seed")
	duplicates := flags.Float64("duplicates", .1, "database duplicate fraction")
	rate := flags.Int("rate", 1000, "pipeline target offered events/sec over complete window")
	concurrency := flags.Int("concurrency", 8, "HTTP workers")
	duration := flags.Duration("duration", 10*time.Second, "measured window per repetition")
	warmup := flags.Duration("warmup", 2*time.Second, "pipeline warm-up window")
	out := flags.String("out", "", "report directory (default own run directory)")
	smoke := flags.Bool("smoke", false, "reduced study correctness cases (not performance evidence)")
	resume := flags.Bool("resume", false, "resume a matching study output directory")
	suite := flags.String("suite", "core", "registered study suite: core or connections")
	activeCap := flags.Int("max-conns", 0, "HTTP active connection cap; zero unlimited")
	idleCap := flags.Int("max-idle-conns", 128, "per-host idle retention")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if *mode != "database" && *mode != "pipeline" && *mode != "study" && *mode != "calibrate" {
		return errors.New("mode must be database, pipeline, study or calibrate")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if *mode == "study" {
		var unsupported string
		flags.Visit(func(f *flag.Flag) {
			switch f.Name {
			case "mode", "smoke", "resume", "out", "seed", "suite":
			default:
				unsupported = f.Name
			}
		})
		if unsupported != "" {
			return fmt.Errorf("study uses registered controls; -%s is only for single-run modes", unsupported)
		}
		if *resume && *out == "" {
			return errors.New("resume requires the original -out directory")
		}
		return benchmark.RunStudy(ctx, benchmark.StudyConfig{Suite: *suite, OutputDirectory: *out, Smoke: *smoke, Resume: *resume, Seed: *seed, Writer: os.Stdout})
	}
	suiteSpecified := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "suite" {
			suiteSpecified = true
		}
	})
	if suiteSpecified {
		return errors.New("suite applies only to study mode")
	}
	if *smoke || *resume {
		return errors.New("smoke/resume flags require study mode")
	}
	if *activeCap < 0 || *idleCap < 0 {
		return errors.New("connection caps must be nonnegative")
	}
	if *mode == "pipeline" || *mode == "calibrate" {
		if err := benchmark.ValidatePipelineWorkload(benchmark.PipelineConfig{Rate: *rate, Duration: *duration, BatchSize: *batch, PayloadBytes: *size}); err != nil {
			return err
		}
	}
	if *mode == "calibrate" {
		if *out == "" {
			return errors.New("calibration requires -out directory")
		}
		r, err := benchmark.RunCalibration(ctx, benchmark.PipelineConfig{Rate: *rate, Duration: *duration, Concurrency: *concurrency, BatchSize: *batch, PayloadBytes: *size, Seed: *seed, Repetitions: 3, HTTPMaxConnsPerHost: *activeCap, HTTPMaxIdleConnsPerHost: *idleCap})
		return writeCalibrationResult(*out, r, err)
	}
	s, e := runenv.Start(ctx, measuredOptions(*mode))
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
		evidenceErr := s.CollectEvidence(end, s.Directory)
		destroyErr := s.Destroy(end)
		result = errors.Join(result, evidenceErr, destroyErr)
		status := map[string]any{"completed": result == nil, "resources_destroyed": destroyErr == nil}
		result = errors.Join(result, benchmark.WriteJSON(filepath.Join(dir, "status.json"), status))
	}()
	p, e := pgxpool.New(ctx, s.DatabaseURL)
	if e != nil {
		return e
	}
	defer p.Close()
	manifest, e := benchmark.CaptureManifest(ctx, s, p)
	if e != nil {
		return e
	}
	if e = benchmark.WriteJSON(filepath.Join(dir, "manifest.json"), manifest); e != nil {
		return e
	}
	if manifest.SourceDirty {
		return errors.New("commit source changes before measuring; manifest marks this run dirty")
	}
	sampleCtx, stopSamples := context.WithCancel(ctx)
	samples := benchmark.SampleResources(sampleCtx, s, p)
	defer func() {
		stopSamples()
		result = errors.Join(result, benchmark.WriteJSON(filepath.Join(dir, "resources.json"), <-samples))
	}()
	if *mode == "database" {
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
	} else {
		client, err := s.HTTPClient()
		if err != nil {
			return err
		}
		defer client.CloseIdleConnections()
		r, e := benchmark.RunPipeline(ctx, benchmark.PipelineConfig{HTTPClient: client, HTTPMaxConnsPerHost: *activeCap, HTTPMaxIdleConnsPerHost: *idleCap, SampleFinished: func(int) error { return benchmark.WaitConsumerCaughtUp(ctx, s) }, Pool: p, APIURL: s.APIURL, APIKey: s.APIKey(), Tenant: "default", Pattern: *pattern, Seed: *seed, Rate: *rate, Concurrency: *concurrency, Duration: *duration, Warmup: *warmup, DrainDeadline: 30 * time.Second, PollInterval: 50 * time.Millisecond, BatchSize: *batch, PayloadBytes: *size, Repetitions: 3})
		if e != nil {
			_ = benchmark.WriteJSON(filepath.Join(dir, "failed-pipeline.json"), r)
			return e
		}
		if e = benchmark.WriteReport(dir, r); e != nil {
			return e
		}
		for _, v := range r.Runs {
			visibility := "unavailable"
			if v.VisibilityMetric != nil {
				visibility = fmt.Sprintf("%.1fms", float64(v.VisibilityMetric.P95)/1e6)
			}
			fmt.Printf("rep %d: attempted=%d accepted=%d persisted=%d missed=%d; %.0f audit-completion/s; ack p95=%.1fms, visibility p95=%s\n", v.Repetition, v.Attempted, v.Accepted, v.Persisted, v.GeneratorMissed, v.VerifiedCompletionEPS, float64(v.Ack.P95)/1e6, visibility)
		}
	}
	return nil
}

func writeCalibrationResult(out string, report benchmark.CalibrationReport, runErr error) error {
	if runErr != nil {
		report.Status = "incomplete"
	}
	return errors.Join(runErr, benchmark.WriteJSON(filepath.Join(out, "calibration.json"), report))
}
