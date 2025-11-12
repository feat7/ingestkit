package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/yourusername/ingestkit/generated/models"
	"github.com/yourusername/ingestkit/generated/storage"
	"github.com/yourusername/ingestkit/internal/messaging"
)

const (
	defaultRedpandaAddr = "localhost:19092"
	defaultTopic        = "ingestkit.events"
	defaultGroupID      = "ingestkit-consumer"
	defaultDBHost       = "localhost"
	defaultDBPort       = "5433"
	defaultDBName       = "ingestkit"
	defaultDBUser       = "ingestkit"
	defaultDBPassword   = "ingestkit_dev"
)

func main() {
	log.Println("🔄 IngestKit Consumer Worker starting...")

	// Configuration
	redpandaAddr := getEnv("REDPANDA_ADDR", defaultRedpandaAddr)
	topic := getEnv("REDPANDA_TOPIC", defaultTopic)
	groupID := getEnv("CONSUMER_GROUP_ID", defaultGroupID)

	dbHost := getEnv("DB_HOST", defaultDBHost)
	dbPort := getEnv("DB_PORT", defaultDBPort)
	dbName := getEnv("DB_NAME", defaultDBName)
	dbUser := getEnv("DB_USER", defaultDBUser)
	dbPassword := getEnv("DB_PASSWORD", defaultDBPassword)

	// Create database writer
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	writer, err := storage.NewWriter(connStr)
	if err != nil {
		log.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	log.Printf("✓ Connected to PostgreSQL at %s:%s", dbHost, dbPort)

	// Event handler - routes to the appropriate generated writer method
	handler := func(ctx context.Context, envelope *messaging.EventEnvelope) error {
		log.Printf("📥 Processing event: type=%s tenant=%s event_id=%s",
			envelope.EventType, envelope.TenantID, envelope.EventID)

		// Route to appropriate handler based on event type
		switch envelope.EventType {
		case "user_signup":
			var event models.UserSignup
			if err := unmarshalEvent(envelope, &event); err != nil {
				return err
			}
			if err := writer.WriteUserSignup(&event); err != nil {
				return fmt.Errorf("failed to write user_signup: %w", err)
			}

		case "purchase":
			var event models.Purchase
			if err := unmarshalEvent(envelope, &event); err != nil {
				return err
			}
			if err := writer.WritePurchase(&event); err != nil {
				return fmt.Errorf("failed to write purchase: %w", err)
			}

		case "page_view":
			var event models.PageView
			if err := unmarshalEvent(envelope, &event); err != nil {
				return err
			}
			if err := writer.WritePageView(&event); err != nil {
				return fmt.Errorf("failed to write page_view: %w", err)
			}

		default:
			return fmt.Errorf("unknown event type: %s", envelope.EventType)
		}

		log.Printf("✅ Event written to database: %s", envelope.EventID)
		return nil
	}

	// Create consumer
	consumer, err := messaging.NewConsumer([]string{redpandaAddr}, topic, groupID, handler)
	if err != nil {
		log.Fatalf("Failed to create consumer: %v", err)
	}
	defer consumer.Close()

	log.Printf("✓ Connected to Redpanda at %s (topic: %s, group: %s)", redpandaAddr, topic, groupID)
	log.Println("🚀 Consumer worker started - waiting for events...")

	// Graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down gracefully...")
		cancel()
	}()

	// Start consuming
	if err := consumer.Start(ctx); err != nil && err != context.Canceled {
		log.Fatalf("Consumer error: %v", err)
	}

	log.Println("Consumer stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// unmarshalEvent unmarshals the event envelope payload into a typed event struct
func unmarshalEvent(envelope *messaging.EventEnvelope, event interface{}) error {
	// Marshal the payload map back to JSON
	payloadJSON, err := json.Marshal(envelope.Payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Unmarshal into the typed event struct
	if err := json.Unmarshal(payloadJSON, event); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	// Set tenant_id from envelope
	switch e := event.(type) {
	case *models.UserSignup:
		e.TenantID = envelope.TenantID
	case *models.Purchase:
		e.TenantID = envelope.TenantID
	case *models.PageView:
		e.TenantID = envelope.TenantID
	}

	return nil
}
