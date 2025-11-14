package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/feat7/ingestkit/internal/api/middleware"
	"github.com/feat7/ingestkit/internal/messaging"
	"github.com/feat7/ingestkit/internal/validation"
)

const (
	defaultPort         = "8080"
	defaultRedpandaAddr = "localhost:19092"
	defaultTopic        = "ingestkit.events"
	defaultRateLimit    = 1000 // requests per second
	defaultSchemaPath   = "schema/events.yaml"
)

var (
	producer  *messaging.Producer
	validator *validation.Validator
)

func main() {
	log.Println("🚀 IngestKit API starting...")

	// Configuration
	port := getEnv("API_PORT", defaultPort)
	redpandaAddr := getEnv("REDPANDA_ADDR", defaultRedpandaAddr)
	topic := getEnv("REDPANDA_TOPIC", defaultTopic)
	schemaPath := getEnv("SCHEMA_PATH", defaultSchemaPath)
	rateLimitRPS, _ := strconv.Atoi(getEnv("RATE_LIMIT_RPS", fmt.Sprintf("%d", defaultRateLimit)))

	// Create producer
	var err error
	producer, err = messaging.NewProducer([]string{redpandaAddr}, topic)
	if err != nil {
		log.Fatalf("Failed to create producer: %v", err)
	}
	defer producer.Close()
	log.Printf("✓ Connected to Redpanda at %s", redpandaAddr)

	// Create validator
	validator, err = validation.NewValidator(schemaPath)
	if err != nil {
		log.Fatalf("Failed to load schema: %v", err)
	}
	log.Printf("✓ Loaded schema from %s", schemaPath)
	log.Printf("  Available event types: %v", validator.GetEventTypes())

	// Setup API keys (from environment for MVP)
	apiKeyConfig := setupAPIKeys()
	log.Printf("✓ Configured %d API key(s)", len(apiKeyConfig.Keys))

	// Setup rate limiter
	rateLimiter := middleware.NewRateLimiter(rateLimitRPS)
	log.Printf("✓ Rate limiting enabled: %d requests/second", rateLimitRPS)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName:      "IngestKit API v1.0",
		ErrorHandler: customErrorHandler,
	})

	// Global middleware (order matters!)
	app.Use(recover.New())
	app.Use(middleware.RequestID())
	app.Use(middleware.CORS(middleware.NewCORSConfig()))
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${locals:request_id} ${status} - ${latency} ${method} ${path}\n",
	}))

	// Public routes (no auth required)
	app.Get("/health", healthHandler)

	// Protected routes (require auth + rate limiting)
	api := app.Group("/v1")
	api.Use(middleware.APIKeyAuth(apiKeyConfig))
	api.Use(middleware.RateLimit(rateLimiter))

	// Event ingestion endpoints
	api.Post("/events/:type", ingestHandler)
	api.Post("/events/:type/batch", ingestBatchHandler)

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Println("\n⏳ Shutting down gracefully...")

		// Flush any pending messages
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		producer.Flush(ctx)

		app.Shutdown()
	}()

	// Start server
	log.Printf("🎯 IngestKit API ready on port %s", port)
	log.Printf("   POST /v1/events/:type       - Ingest single event")
	log.Printf("   POST /v1/events/:type/batch - Ingest batch events")
	log.Printf("   GET  /health                - Health check")
	log.Println()
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func healthHandler(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":      "healthy",
		"service":     "ingestkit-api",
		"timestamp":   time.Now().Unix(),
		"event_types": validator.GetEventTypes(),
	})
}

func ingestHandler(c *fiber.Ctx) error {
	eventType := c.Params("type")

	// Check if event type exists
	if !validator.EventTypeExists(eventType) {
		return middleware.SendError(c, fiber.StatusBadRequest, middleware.ErrCodeUnknownEvent,
			fmt.Sprintf("Unknown event type: %s", eventType))
	}

	// Parse request body
	var payload map[string]interface{}
	if err := c.BodyParser(&payload); err != nil {
		return middleware.SendError(c, fiber.StatusBadRequest, middleware.ErrCodeBadRequest,
			"Invalid JSON payload")
	}

	// Validate event against schema
	if err := validator.ValidateEvent(eventType, payload); err != nil {
		return middleware.SendError(c, fiber.StatusBadRequest, middleware.ErrCodeValidation,
			err.Error())
	}

	// Get tenant_id from auth middleware
	tenantID := c.Locals("tenant_id").(string)

	// Create envelope
	requestID := c.Locals("request_id").(string)
	envelope := &messaging.EventEnvelope{
		SchemaVersion: "v1",
		EventType:     eventType,
		TenantID:      tenantID,
		EventID:       fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		Timestamp:     time.Now(),
		Payload:       payload,
	}

	// Publish asynchronously
	errChan := make(chan error, 1)
	producer.PublishAsync(c.Context(), envelope, errChan)

	// Check for immediate errors (non-blocking)
	select {
	case err := <-errChan:
		log.Printf("Failed to publish event %s: %v", envelope.EventID, err)
		return middleware.SendError(c, fiber.StatusInternalServerError, middleware.ErrCodeInternal,
			"Failed to publish event")
	case <-time.After(10 * time.Millisecond):
		// Message queued successfully, return immediately
	}

	// Success response
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"status":     "accepted",
		"event_id":   envelope.EventID,
		"event_type": eventType,
		"tenant_id":  tenantID,
		"request_id": requestID,
	})
}

func ingestBatchHandler(c *fiber.Ctx) error {
	eventType := c.Params("type")

	// Check if event type exists
	if !validator.EventTypeExists(eventType) {
		return middleware.SendError(c, fiber.StatusBadRequest, middleware.ErrCodeUnknownEvent,
			fmt.Sprintf("Unknown event type: %s", eventType))
	}

	// Parse request body (array of events)
	var request struct {
		Events []map[string]interface{} `json:"events"`
	}

	if err := c.BodyParser(&request); err != nil {
		return middleware.SendError(c, fiber.StatusBadRequest, middleware.ErrCodeBadRequest,
			"Invalid JSON payload: expected {\"events\": [...]}")
	}

	// Validate batch size
	if len(request.Events) == 0 {
		return middleware.SendError(c, fiber.StatusBadRequest, middleware.ErrCodeBadRequest,
			"Batch is empty")
	}

	if len(request.Events) > 1000 {
		return middleware.SendError(c, fiber.StatusBadRequest, middleware.ErrCodeBadRequest,
			"Batch too large (max 1000 events)")
	}

	// Get tenant_id from auth middleware
	tenantID := c.Locals("tenant_id").(string)
	requestID := c.Locals("request_id").(string)

	// Validate all events first (fail fast)
	for i, payload := range request.Events {
		if err := validator.ValidateEvent(eventType, payload); err != nil {
			return middleware.SendError(c, fiber.StatusBadRequest, middleware.ErrCodeValidation,
				fmt.Sprintf("Event %d validation failed: %v", i, err))
		}
	}

	// Create envelopes
	envelopes := make([]*messaging.EventEnvelope, len(request.Events))
	eventIDs := make([]string, len(request.Events))

	for i, payload := range request.Events {
		eventID := fmt.Sprintf("evt_%d_%d", time.Now().UnixNano(), i)
		eventIDs[i] = eventID

		envelopes[i] = &messaging.EventEnvelope{
			SchemaVersion: "v1",
			EventType:     eventType,
			TenantID:      tenantID,
			EventID:       eventID,
			Timestamp:     time.Now(),
			Payload:       payload,
		}
	}

	// Publish batch asynchronously
	errChan := make(chan error, len(envelopes))
	producer.PublishAsyncBatch(c.Context(), envelopes, errChan)

	// Check for immediate errors (non-blocking)
	select {
	case err := <-errChan:
		log.Printf("Failed to publish batch: %v", err)
		return middleware.SendError(c, fiber.StatusInternalServerError, middleware.ErrCodeInternal,
			"Failed to publish some events")
	case <-time.After(10 * time.Millisecond):
		// Batch queued successfully
	}

	// Success response
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"status":      "accepted",
		"event_type":  eventType,
		"tenant_id":   tenantID,
		"request_id":  requestID,
		"event_count": len(eventIDs),
		"event_ids":   eventIDs,
	})
}

func setupAPIKeys() *middleware.APIKeyConfig {
	config := middleware.NewAPIKeyConfig()

	// Load API keys from environment
	// Format: API_KEY_1=key:tenant_id, API_KEY_2=key:tenant_id, etc.
	for i := 1; i <= 10; i++ {
		envKey := fmt.Sprintf("API_KEY_%d", i)
		value := os.Getenv(envKey)
		if value != "" {
			// Parse "key:tenant_id" format
			// For MVP, if no colon, use the key as both key and tenant_id
			apiKey := value
			tenantID := "default"

			// Check if format is "key:tenant_id"
			parts := splitOnce(value, ":")
			if len(parts) == 2 {
				apiKey = parts[0]
				tenantID = parts[1]
			}

			config.AddKey(apiKey, tenantID)
		}
	}

	// Add default dev key if no keys configured
	if len(config.Keys) == 0 {
		log.Println("⚠️  No API keys configured, adding default dev key")
		config.AddKey("dev_key_1234567890", "default")
	}

	return config
}

func splitOnce(s, sep string) []string {
	for i := 0; i < len(s)-len(sep)+1; i++ {
		if i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			return []string{s[:i], s[i+len(sep):]}
		}
	}
	return []string{s}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func customErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	return middleware.SendError(c, code, middleware.ErrCodeInternal, err.Error())
}
