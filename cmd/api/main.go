package main

import (
	"context"
	"errors"
	"github.com/feat7/ingestkit/internal/api"
	"github.com/feat7/ingestkit/internal/config"
	"github.com/feat7/ingestkit/internal/messaging"
	"github.com/feat7/ingestkit/internal/service"
	"github.com/feat7/ingestkit/internal/telemetry"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if len(os.Args) == 2 && os.Args[1] == "check" {
		c, end := context.WithTimeout(ctx, 2*time.Second)
		defer end()
		addr := os.Getenv("HTTP_ADDR")
		if addr == "" {
			addr = ":8080"
		}
		if service.Check(c, addr) != nil {
			os.Exit(1)
		}
		return
	}
	if err := run(ctx); err != nil {
		log.Print("API stopped with an error: ", err)
		os.Exit(1)
	}
}
func run(ctx context.Context) error {
	cfg, err := config.LoadAPIEnv(os.Getenv)
	if err != nil {
		return err
	}
	metrics := telemetry.New()
	p, err := messaging.NewPublisher(messaging.PublisherConfig{Brokers: cfg.Brokers, Topic: cfg.EventTopic, PublishTimeout: cfg.PublishTimeout}, metrics)
	if err != nil {
		return err
	}
	app, clean, err := api.NewApp(api.Config{APIKeys: cfg.APIKeys, RateLimitRPS: cfg.RateLimitRPS, PublishTimeout: cfg.PublishTimeout}, p, metrics)
	if err != nil {
		p.Close()
		return err
	}
	defer clean()
	listener, err := net.Listen("tcp", cfg.ListenAddress)
	if err != nil {
		p.Close()
		return err
	}
	app.Handler()
	done := make(chan error, 1)
	go func() { done <- app.Listener(listener) }()
	select {
	case err := <-done:
		p.Close()
		if errors.Is(err, net.ErrClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}
	end, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return service.Drain(end, func(c context.Context) error { _ = listener.Close(); return app.ShutdownWithContext(c) }, p.Flush, p.Close)
}
