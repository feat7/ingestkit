package main

import (
	"context"
	"errors"
	"github.com/feat7/ingestkit/internal/config"
	"github.com/feat7/ingestkit/internal/messaging"
	"github.com/feat7/ingestkit/internal/persistence"
	"github.com/feat7/ingestkit/internal/service"
	"github.com/feat7/ingestkit/internal/telemetry"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

func main() {
	service.ConfigureLogging()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if len(os.Args) == 2 && os.Args[1] == "check" {
		c, end := context.WithTimeout(ctx, 2*time.Second)
		defer end()
		addr := os.Getenv("METRICS_ADDR")
		if addr == "" {
			addr = ":8081"
		}
		if service.Check(c, addr) != nil {
			os.Exit(1)
		}
		return
	}
	if err := run(ctx); err != nil {
		service.NewDiagnostics(slog.Default(), 10*time.Second).Report("startup", "", -1, -1, err)
		os.Exit(1)
	}
}
func run(ctx context.Context) error {
	cfg, err := config.LoadConsumerEnv(os.Getenv)
	if err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	metrics := telemetry.New()
	store := persistence.NewEventStore(pool)
	var current atomic.Pointer[messaging.Reader]
	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	health := service.NewProcessingHealth()
	diagnostics := service.NewDiagnostics(slog.Default(), 10*time.Second)
	mux.HandleFunc("/ready", readiness(health, store.Ping, func(c context.Context) error {
		reader := current.Load()
		if reader == nil {
			return errors.New("reader unavailable")
		}
		return reader.Ping(c)
	}))
	listener, err := net.Listen("tcp", cfg.ListenAddress)
	if err != nil {
		pool.Close()
		return err
	}
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()
	workerDone := make(chan error, 1)
	go func() {
		workerDone <- service.Supervise(ctx, workerCtx, func(c context.Context) error {
			probe, end := context.WithTimeout(c, time.Second)
			err := store.Ping(probe)
			end()
			diagnostics.Report("startup", "", -1, -1, err)
			if err != nil {
				return err
			}
			if ctx.Err() != nil {
				return nil
			}
			reader, err := messaging.NewReader(messaging.ReaderConfig{OnOperation: func(op messaging.Operation) {
				health.Observe(op.Kind, op.Err)
				diagnostics.Report(op.Kind, op.Topic, op.Partition, op.Offset, op.Err)
			}, Brokers: cfg.Brokers, Topic: cfg.EventTopic, DLQTopic: cfg.DLQTopic, GroupID: cfg.GroupID}, store, metrics)
			if err != nil {
				diagnostics.Report("startup", "", -1, -1, err)
				return err
			}
			defer reader.Close()
			current.Store(reader)
			if ctx.Err() != nil {
				reader.StopPolling()
			}
			defer current.Store(nil)
			return reader.Run(c)
		}, time.Second)
	}()
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	var serveErr error
	select {
	case serveErr = <-serveDone:
		cancelWorker()
	case <-ctx.Done():
	}
	health.Observe("shutdown", context.Canceled)
	end, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return errors.Join(serveErr, service.Drain(end, func(c context.Context) error {
		drained := drainConsumer(c, workerDone, func() {
			if reader := current.Load(); reader != nil {
				reader.StopPolling()
			}
		}, cancelWorker)
		return errors.Join(drained, server.Shutdown(c))
	}, func(context.Context) error { return nil }, pool.Close))
}
func readiness(health *service.ProcessingHealth, storePing, readerPing func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if !health.Ready() || storePing(ctx) != nil || readerPing(ctx) != nil {
			http.Error(w, "dependencies or processing unavailable", 503)
			return
		}
		w.WriteHeader(200)
	}
}
func drainConsumer(ctx context.Context, done <-chan error, stopPolling, cancelWork func()) error {
	stopPolling()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		cancelWork()
		return ctx.Err()
	}
}
