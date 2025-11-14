package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/feat7/ingestkit/internal/api/middleware"
	applogger "github.com/feat7/ingestkit/internal/logger"
	"github.com/feat7/ingestkit/internal/messaging"
	"github.com/feat7/ingestkit/internal/validation"
)

const (
	defaultPort         = "8080"
	defaultRedpandaAddr = "localhost:19092"
	defaultTopic        = "ingestkit.events"
	defaultRateLimit    = 1000 // requests per second
	defaultSchemaPath   = "schema/events.yaml"

	// asyncPublishTimeout is the max time to wait for immediate publish errors
	// This allows fast failure detection while keeping request latency low
	asyncPublishTimeout = 10 * time.Millisecond

	// maxBatchSize is the maximum number of events allowed in a batch request
	maxBatchSize = 1000
)

var (
	producer  *messaging.Producer
	validator *validation.Validator
)

func main() {
	// Initialize structured logging
	applogger.Setup()
	log.Info().Msg("🚀 IngestKit API starting...")

	// Configuration
	port := getEnv("API_PORT", defaultPort)
	redpandaAddr := getEnv("REDPANDA_ADDR", defaultRedpandaAddr)
	topic := getEnv("REDPANDA_TOPIC", defaultTopic)
	schemaPath := getEnv("SCHEMA_PATH", defaultSchemaPath)

	rateLimitRPSStr := getEnv("RATE_LIMIT_RPS", fmt.Sprintf("%d", defaultRateLimit))
	rateLimitRPS, err := strconv.Atoi(rateLimitRPSStr)
	if err != nil {
		log.Fatal().Str("value", rateLimitRPSStr).Msg("Invalid RATE_LIMIT_RPS value: must be an integer")
	}

	// Validate configuration
	if err = validateConfig(port, redpandaAddr, topic, schemaPath, rateLimitRPS); err != nil {
		log.Fatal().Err(err).Msg("Configuration validation failed")
	}

	// Create producer
	producer, err = messaging.NewProducer([]string{redpandaAddr}, topic)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create producer")
	}
	defer producer.Close()
	log.Info().Str("address", redpandaAddr).Msg("✓ Connected to Redpanda")

	// Create validator
	validator, err = validation.NewValidator(schemaPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load schema")
	}
	log.Info().Str("path", schemaPath).Msg("✓ Loaded schema")
	log.Info().Interface("event_types", validator.GetEventTypes()).Msg("Available event types")

	// Setup API keys (from environment for MVP)
	apiKeyConfig := setupAPIKeys()
	log.Info().Int("count", len(apiKeyConfig.Keys)).Msg("✓ Configured API keys")

	// Setup rate limiter
	rateLimiter := middleware.NewRateLimiter(rateLimitRPS)
	log.Info().Int("rps", rateLimitRPS).Msg("✓ Rate limiting enabled")

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

	// Public routes (no auth or rate limit required)
	// Health endpoint is intentionally unprotected for:
	// - Load balancer health checks
	// - Kubernetes liveness/readiness probes
	// - Monitoring systems (Prometheus, Datadog, etc.)
	// DDoS protection should be handled at infrastructure layer (reverse proxy, CDN)
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
		log.Info().Msg("⏳ Shutting down gracefully...")

		// Flush any pending messages
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		producer.Flush(ctx)

		app.Shutdown()
	}()

	// Start server
	log.Info().Str("port", port).Msg("🎯 IngestKit API ready")
	log.Info().Msg("   POST /v1/events/:type       - Ingest single event")
	log.Info().Msg("   POST /v1/events/:type/batch - Ingest batch events")
	log.Info().Msg("   GET  /health                - Health check")
	if err := app.Listen(":" + port); err != nil {
		log.Fatal().Err(err).Msg("Failed to start server")
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
			fmt.Sprintf("unknown event type: %s", eventType))
	}

	// Parse request body
	var payload map[string]interface{}
	if err := c.BodyParser(&payload); err != nil {
		return middleware.SendError(c, fiber.StatusBadRequest, middleware.ErrCodeBadRequest,
			"invalid JSON payload")
	}

	// Validate event against schema
	if err := validator.ValidateEvent(eventType, payload); err != nil {
		return middleware.SendError(c, fiber.StatusBadRequest, middleware.ErrCodeValidation,
			err.Error())
	}

	// Get tenant_id from auth middleware
	tenantID, ok := c.Locals("tenant_id").(string)
	if !ok {
		return middleware.SendError(c, fiber.StatusInternalServerError, middleware.ErrCodeInternal,
			"missing tenant context")
	}

	// Marshal payload to JSON for envelope (avoids re-marshaling in consumer)
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return middleware.SendError(c, fiber.StatusInternalServerError, middleware.ErrCodeInternal,
			"failed to marshal payload")
	}

	// Create envelope
	requestID, ok := c.Locals("request_id").(string)
	if !ok {
		return middleware.SendError(c, fiber.StatusInternalServerError, middleware.ErrCodeInternal,
			"missing request ID")
	}
	envelope := &messaging.EventEnvelope{
		SchemaVersion: "v1",
		EventType:     eventType,
		TenantID:      tenantID,
		EventID:       uuid.Must(uuid.NewV7()).String(), // UUID v7 - time-ordered, sortable UUIDs
		Timestamp:     time.Now(),
		Payload:       payloadJSON,
	}

	// Publish asynchronously
	errChan := make(chan error, 1)
	producer.PublishAsync(c.Context(), envelope, errChan)

	// Check for immediate errors (non-blocking)
	select {
	case err := <-errChan:
		log.Error().
			Str("event_id", envelope.EventID).
			Str("event_type", eventType).
			Err(err).
			Msg("Failed to publish event")
		return middleware.SendError(c, fiber.StatusInternalServerError, middleware.ErrCodeInternal,
			"Failed to publish event")
	case <-time.After(asyncPublishTimeout):
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
			fmt.Sprintf("unknown event type: %s", eventType))
	}

	// Parse request body (array of events)
	var request struct {
		Events []map[string]interface{} `json:"events"`
	}

	if err := c.BodyParser(&request); err != nil {
		return middleware.SendError(c, fiber.StatusBadRequest, middleware.ErrCodeBadRequest,
			"invalid JSON payload: expected {\"events\": [...]}")
	}

	// Validate batch size
	if len(request.Events) == 0 {
		return middleware.SendError(c, fiber.StatusBadRequest, middleware.ErrCodeBadRequest,
			"batch is empty")
	}

	if len(request.Events) > maxBatchSize {
		return middleware.SendError(c, fiber.StatusBadRequest, middleware.ErrCodeBadRequest,
			fmt.Sprintf("batch too large (max %d events)", maxBatchSize))
	}

	// Get tenant_id from auth middleware
	tenantID, ok := c.Locals("tenant_id").(string)
	if !ok {
		return middleware.SendError(c, fiber.StatusInternalServerError, middleware.ErrCodeInternal,
			"missing tenant context")
	}
	requestID, ok := c.Locals("request_id").(string)
	if !ok {
		return middleware.SendError(c, fiber.StatusInternalServerError, middleware.ErrCodeInternal,
			"missing request ID")
	}

	// Validate all events first (fail fast)
	for i, payload := range request.Events {
		if err := validator.ValidateEvent(eventType, payload); err != nil {
			return middleware.SendError(c, fiber.StatusBadRequest, middleware.ErrCodeValidation,
				fmt.Sprintf("event %d validation failed: %v", i, err))
		}
	}

	// Create envelopes
	envelopes := make([]*messaging.EventEnvelope, len(request.Events))
	eventIDs := make([]string, len(request.Events))

	for i, payload := range request.Events {
		eventID := uuid.Must(uuid.NewV7()).String() // UUID v7 - time-ordered, sortable UUIDs
		eventIDs[i] = eventID

		// Marshal payload to JSON (avoids re-marshaling in consumer)
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			return middleware.SendError(c, fiber.StatusInternalServerError, middleware.ErrCodeInternal,
				fmt.Sprintf("failed to marshal event %d payload", i))
		}

		envelopes[i] = &messaging.EventEnvelope{
			SchemaVersion: "v1",
			EventType:     eventType,
			TenantID:      tenantID,
			EventID:       eventID,
			Timestamp:     time.Now(),
			Payload:       payloadJSON,
		}
	}

	// Publish batch asynchronously
	errChan := make(chan error, len(envelopes))
	producer.PublishAsyncBatch(c.Context(), envelopes, errChan)

	// Check for immediate errors (non-blocking)
	select {
	case err := <-errChan:
		log.Error().
			Str("event_type", eventType).
			Int("batch_size", len(envelopes)).
			Err(err).
			Msg("Failed to publish batch")
		return middleware.SendError(c, fiber.StatusInternalServerError, middleware.ErrCodeInternal,
			"Failed to publish some events")
	case <-time.After(asyncPublishTimeout):
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
			parts := strings.SplitN(value, ":", 2)
			if len(parts) == 2 {
				apiKey = parts[0]
				tenantID = parts[1]
			}

			config.AddKey(apiKey, tenantID)
		}
	}

	// Add default dev key if no keys configured
	if len(config.Keys) == 0 {
		log.Warn().Msg("⚠️  No API keys configured, adding default dev key")
		config.AddKey("dev_key_1234567890", "default")
	}

	return config
}

// validateConfig validates API server configuration
func validateConfig(port, redpandaAddr, topic, schemaPath string, rateLimitRPS int) error {
	// Validate port
	if port == "" {
		return fmt.Errorf("API_PORT cannot be empty")
	}
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("API_PORT must be a valid port number: %w", err)
	}
	if portNum < 1 || portNum > 65535 {
		return fmt.Errorf("API_PORT must be between 1 and 65535, got %d", portNum)
	}

	// Validate Redpanda address
	if redpandaAddr == "" {
		return fmt.Errorf("REDPANDA_ADDR cannot be empty")
	}
	if !strings.Contains(redpandaAddr, ":") {
		return fmt.Errorf("REDPANDA_ADDR must include port (e.g., localhost:19092)")
	}

	// Validate topic
	if topic == "" {
		return fmt.Errorf("REDPANDA_TOPIC cannot be empty")
	}

	// Validate schema path
	if schemaPath == "" {
		return fmt.Errorf("SCHEMA_PATH cannot be empty")
	}
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
		return fmt.Errorf("schema file does not exist: %s", schemaPath)
	}

	// Validate rate limit
	if rateLimitRPS < 1 {
		return fmt.Errorf("RATE_LIMIT_RPS must be positive, got %d", rateLimitRPS)
	}
	if rateLimitRPS > 1000000 {
		return fmt.Errorf("RATE_LIMIT_RPS is unrealistically high (max 1000000), got %d", rateLimitRPS)
	}

	log.Info().
		Str("port", port).
		Str("redpanda", redpandaAddr).
		Str("topic", topic).
		Str("schema", schemaPath).
		Int("rate_limit_rps", rateLimitRPS).
		Msg("✓ Configuration validated successfully")

	return nil
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
