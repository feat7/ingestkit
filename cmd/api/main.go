package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/yourusername/ingestkit/internal/messaging"
)

const (
	defaultPort         = "8080"
	defaultRedpandaAddr = "localhost:19092"
	defaultTopic        = "ingestkit.events"
)

var producer *messaging.Producer

func main() {
	// Configuration
	port := getEnv("API_PORT", defaultPort)
	redpandaAddr := getEnv("REDPANDA_ADDR", defaultRedpandaAddr)
	topic := getEnv("REDPANDA_TOPIC", defaultTopic)

	// Create producer
	var err error
	producer, err = messaging.NewProducer([]string{redpandaAddr}, topic)
	if err != nil {
		log.Fatalf("Failed to create producer: %v", err)
	}
	defer producer.Close()

	log.Printf("✓ Connected to Redpanda at %s", redpandaAddr)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		AppName: "IngestKit API v1.0",
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))

	// Routes
	app.Get("/health", healthHandler)
	app.Post("/v1/events/:type", ingestHandler)

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down gracefully...")
		app.Shutdown()
	}()

	// Start server
	log.Printf("🚀 IngestKit API starting on port %s", port)
	log.Printf("   POST /v1/events/:type - Ingest events")
	log.Printf("   GET  /health          - Health check")
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func healthHandler(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status": "healthy",
		"service": "ingestkit-api",
		"timestamp": time.Now().Unix(),
	})
}

func ingestHandler(c *fiber.Ctx) error {
	eventType := c.Params("type")

	// Parse request body
	var payload map[string]interface{}
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"error": "Invalid JSON payload",
		})
	}

	// Extract tenant_id (for POC, default to "default")
	tenantID, ok := payload["tenant_id"].(string)
	if !ok || tenantID == "" {
		tenantID = "default"
	}

	// Create envelope
	envelope := &messaging.EventEnvelope{
		SchemaVersion: "v1",
		EventType:     eventType,
		TenantID:      tenantID,
		EventID:       fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		Timestamp:     time.Now(),
		Payload:       payload,
	}

	// Publish to Redpanda
	if err := producer.Publish(c.Context(), envelope); err != nil {
		log.Printf("Failed to publish event: %v", err)
		return c.Status(500).JSON(fiber.Map{
			"error": "Failed to publish event",
		})
	}

	// Success response
	return c.Status(202).JSON(fiber.Map{
		"status":     "accepted",
		"event_id":   envelope.EventID,
		"event_type": eventType,
		"tenant_id":  tenantID,
	})
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
