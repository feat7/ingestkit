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
		slog.Error("consumer stopped")
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
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		c, end := context.WithTimeout(r.Context(), time.Second)
		defer end()
		reader := current.Load()
		if reader == nil || store.Ping(c) != nil || reader.Ping(c) != nil {
			http.Error(w, "dependencies unavailable", 503)
			return
		}
		w.WriteHeader(200)
	})
	listener, err := net.Listen("tcp", cfg.ListenAddress)
	if err != nil {
		pool.Close()
		return err
	}
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	workerCtx, cancelWorker := context.WithCancel(ctx)
	defer cancelWorker()
	workerDone := make(chan error, 1)
	go func() {
		workerDone <- service.Supervise(workerCtx, func(c context.Context) error {
			probe, end := context.WithTimeout(c, time.Second)
			err := store.Ping(probe)
			end()
			if err != nil {
				return err
			}
			reader, err := messaging.NewReader(messaging.ReaderConfig{Brokers: cfg.Brokers, Topic: cfg.EventTopic, DLQTopic: cfg.DLQTopic, GroupID: cfg.GroupID}, store, metrics)
			if err != nil {
				return err
			}
			defer reader.Close()
			current.Store(reader)
			defer current.Store(nil)
			return reader.Run(c)
		}, time.Second)
	}()
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	select {
	case err := <-serveDone:
		cancelWorker()
		pool.Close()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}
	end, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return service.Drain(end, func(c context.Context) error {
		cancelWorker()
		if err := server.Shutdown(c); err != nil {
			return err
		}
		select {
		case <-workerDone:
			return nil
		case <-c.Done():
			return c.Err()
		}
	}, func(context.Context) error { return nil }, pool.Close)
}
