package api

import (
	"context"
	"errors"
	"github.com/feat7/ingestkit/internal/api/middleware"
	"github.com/feat7/ingestkit/internal/event"
	"github.com/feat7/ingestkit/internal/telemetry"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"
	"strconv"
	"sync"
	"time"
)

type Config struct {
	APIKeys        map[string]string
	RateLimitRPS   int
	PublishTimeout time.Duration
}
type EventPublisher interface {
	Publish(context.Context, []event.Envelope) error
	Ping(context.Context) error
}

func NewApp(config Config, publisher EventPublisher, metrics *telemetry.Metrics) (*fiber.App, func(), error) {
	if len(config.APIKeys) == 0 || config.RateLimitRPS <= 0 || config.PublishTimeout <= 0 || publisher == nil {
		return nil, nil, errors.New("invalid API configuration")
	}
	keys := middleware.NewAPIKeyConfig()
	for key, tenant := range config.APIKeys {
		if key == "" || len(tenant) < 1 || len(tenant) > 128 {
			return nil, nil, errors.New("invalid API key mapping")
		}
		keys.AddKey(key, tenant)
	}
	if metrics == nil {
		metrics = telemetry.New()
	}
	app := fiber.New(fiber.Config{BodyLimit: event.MaxRequestBytes, DisableStartupMessage: true, ErrorHandler: func(c *fiber.Ctx, err error) error {
		status := 500
		var fe *fiber.Error
		if errors.As(err, &fe) {
			status = fe.Code
		}
		ensureRequestID(c)
		return middleware.SendError(c, status, "request_failed", "request could not be processed")
	}})
	app.Use(middleware.RequestID(), recover.New())
	app.Use(func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		route := "other"
		switch c.Path() {
		case "/v1/events":
			route = "single"
		case "/v1/events/batch":
			route = "batch"
		case "/ready":
			route = "ready"
		case "/health":
			route = "health"
		case "/metrics":
			route = "metrics"
		}
		metrics.ObserveHTTP(route, strconv.Itoa(c.Response().StatusCode()), time.Since(start))
		return err
	})
	app.Get("/health", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"status": "alive"}) })
	app.Get("/ready", func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if publisher.Ping(ctx) != nil {
			return middleware.SendError(c, 503, "unavailable", "broker is not ready")
		}
		return c.JSON(fiber.Map{"status": "ready"})
	})
	app.Get("/metrics", adaptor.HTTPHandler(metrics.Handler()))
	limiter := middleware.NewRateLimiter(config.RateLimitRPS)
	protected := app.Group("/v1", middleware.APIKeyAuth(keys), middleware.RateLimit(limiter))
	protected.Post("/events", ingest(publisher, config.PublishTimeout, false))
	protected.Post("/events/batch", ingest(publisher, config.PublishTimeout, true))
	var once sync.Once
	return app, func() { once.Do(limiter.Stop) }, nil
}
func ensureRequestID(c *fiber.Ctx) {
	if _, ok := c.Locals("request_id").(string); !ok {
		c.Locals("request_id", uuid.NewString())
	}
	c.Set("X-Request-ID", c.Locals("request_id").(string))
}
