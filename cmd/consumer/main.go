package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/feat7/ingestkit/generated/models"
	"github.com/feat7/ingestkit/generated/storage"
	"github.com/feat7/ingestkit/internal/messaging"
	dlqstorage "github.com/feat7/ingestkit/internal/storage"
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

	// Create database writer with connection pooling
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	writer, err := storage.NewWriter(connStr)
	if err != nil {
		log.Fatalf("Failed to create writer: %v", err)
	}
	defer writer.Close()

	log.Printf("✓ Connected to PostgreSQL at %s:%s (pool: 50 max conns)", dbHost, dbPort)

	// Create DLQ writer
	dlqWriter, err := dlqstorage.NewDLQWriter(connStr)
	if err != nil {
		log.Fatalf("Failed to create DLQ writer: %v", err)
	}
	defer dlqWriter.Close()

	log.Printf("✓ DLQ writer initialized")

	// Batch event handler - processes events in batches
	batchHandler := func(ctx context.Context, envelopes []*messaging.EventEnvelope) error {
		log.Printf("📦 Processing batch: %d events", len(envelopes))

		// Group events by type for batch insertion
		userSignups := make([]*models.UserSignup, 0)
		purchases := make([]*models.Purchase, 0)
		pageViews := make([]*models.PageView, 0)

		// Unmarshal and group events
		for _, envelope := range envelopes {
			switch envelope.EventType {
			case "user_signup":
				var event models.UserSignup
				if err := unmarshalEvent(envelope, &event); err != nil {
					return fmt.Errorf("failed to unmarshal user_signup: %w", err)
				}
				userSignups = append(userSignups, &event)

			case "purchase":
				var event models.Purchase
				if err := unmarshalEvent(envelope, &event); err != nil {
					return fmt.Errorf("failed to unmarshal purchase: %w", err)
				}
				purchases = append(purchases, &event)

			case "page_view":
				var event models.PageView
				if err := unmarshalEvent(envelope, &event); err != nil {
					return fmt.Errorf("failed to unmarshal page_view: %w", err)
				}
				pageViews = append(pageViews, &event)

			default:
				return fmt.Errorf("unknown event type: %s", envelope.EventType)
			}
		}

		// Write batches to database
		if len(userSignups) > 0 {
			if err := writer.WriteUserSignupBatch(userSignups); err != nil {
				return fmt.Errorf("failed to write user_signup batch: %w", err)
			}
			log.Printf("✅ Wrote %d user_signup events", len(userSignups))
		}

		if len(purchases) > 0 {
			if err := writer.WritePurchaseBatch(purchases); err != nil {
				return fmt.Errorf("failed to write purchase batch: %w", err)
			}
			log.Printf("✅ Wrote %d purchase events", len(purchases))
		}

		if len(pageViews) > 0 {
			if err := writer.WritePageViewBatch(pageViews); err != nil {
				return fmt.Errorf("failed to write page_view batch: %w", err)
			}
			log.Printf("✅ Wrote %d page_view events", len(pageViews))
		}

		return nil
	}

	// Create consumer with batch processing
	consumer, err := messaging.NewBatchConsumer(
		[]string{redpandaAddr},
		topic,
		groupID,
		batchHandler,
		messaging.DefaultConsumerConfig(),
		dlqWriter,
	)
	if err != nil {
		log.Fatalf("Failed to create consumer: %v", err)
	}
	defer consumer.Close()

	log.Printf("✓ Connected to Redpanda at %s (topic: %s, group: %s)", redpandaAddr, topic, groupID)

	// Start metrics HTTP server
	metricsPort := getEnv("METRICS_PORT", "8081")
	go startMetricsServer(metricsPort, consumer)
	log.Printf("✓ Metrics server started on :%s (endpoint: /metrics)", metricsPort)

	log.Println("🚀 Consumer worker started with batch processing - waiting for events...")

	// Graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Println("⏹️  Shutting down gracefully...")
		cancel()
	}()

	// Start consuming
	if err := consumer.Start(ctx); err != nil && err != context.Canceled {
		log.Fatalf("Consumer error: %v", err)
	}

	// Print final metrics
	metrics := consumer.GetMetrics()
	log.Printf("📊 Final metrics: processed=%d failed=%d dlq=%d batches=%d",
		metrics.EventsProcessed, metrics.EventsFailed, metrics.EventsDLQ, metrics.BatchesProcessed)

	log.Println("✅ Consumer stopped")
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

// startMetricsServer starts an HTTP server for metrics exposition
func startMetricsServer(port string, consumer *messaging.Consumer) {
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics := consumer.GetMetrics()

		// Calculate average latency
		avgLatency := int64(0)
		if metrics.BatchesProcessed > 0 {
			avgLatency = metrics.TotalLatencyMs / metrics.BatchesProcessed
		}

		// Calculate events per second
		eventsPerSecond := float64(0)
		if !metrics.LastProcessedTime.IsZero() {
			duration := time.Since(metrics.LastProcessedTime).Seconds()
			if duration > 0 {
				eventsPerSecond = float64(metrics.EventsProcessed) / duration
			}
		}

		// Prometheus format
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "# HELP ingestkit_events_processed_total Total number of events successfully processed\n")
		fmt.Fprintf(w, "# TYPE ingestkit_events_processed_total counter\n")
		fmt.Fprintf(w, "ingestkit_events_processed_total %d\n", metrics.EventsProcessed)

		fmt.Fprintf(w, "# HELP ingestkit_events_failed_total Total number of events that failed processing\n")
		fmt.Fprintf(w, "# TYPE ingestkit_events_failed_total counter\n")
		fmt.Fprintf(w, "ingestkit_events_failed_total %d\n", metrics.EventsFailed)

		fmt.Fprintf(w, "# HELP ingestkit_events_dlq_total Total number of events sent to DLQ\n")
		fmt.Fprintf(w, "# TYPE ingestkit_events_dlq_total counter\n")
		fmt.Fprintf(w, "ingestkit_events_dlq_total %d\n", metrics.EventsDLQ)

		fmt.Fprintf(w, "# HELP ingestkit_db_write_errors_total Total number of DB write errors\n")
		fmt.Fprintf(w, "# TYPE ingestkit_db_write_errors_total counter\n")
		fmt.Fprintf(w, "ingestkit_db_write_errors_total %d\n", metrics.DBWriteErrors)

		fmt.Fprintf(w, "# HELP ingestkit_unmarshal_errors_total Total number of unmarshal errors\n")
		fmt.Fprintf(w, "# TYPE ingestkit_unmarshal_errors_total counter\n")
		fmt.Fprintf(w, "ingestkit_unmarshal_errors_total %d\n", metrics.UnmarshalErrors)

		fmt.Fprintf(w, "# HELP ingestkit_batches_processed_total Total number of batches processed\n")
		fmt.Fprintf(w, "# TYPE ingestkit_batches_processed_total counter\n")
		fmt.Fprintf(w, "ingestkit_batches_processed_total %d\n", metrics.BatchesProcessed)

		fmt.Fprintf(w, "# HELP ingestkit_batches_failed_total Total number of batches that failed processing\n")
		fmt.Fprintf(w, "# TYPE ingestkit_batches_failed_total counter\n")
		fmt.Fprintf(w, "ingestkit_batches_failed_total %d\n", metrics.BatchesFailed)

		fmt.Fprintf(w, "# HELP ingestkit_batch_latency_ms_avg Average batch processing latency in milliseconds\n")
		fmt.Fprintf(w, "# TYPE ingestkit_batch_latency_ms_avg gauge\n")
		fmt.Fprintf(w, "ingestkit_batch_latency_ms_avg %d\n", avgLatency)

		fmt.Fprintf(w, "# HELP ingestkit_events_per_second Current events per second rate\n")
		fmt.Fprintf(w, "# TYPE ingestkit_events_per_second gauge\n")
		fmt.Fprintf(w, "ingestkit_events_per_second %.2f\n", eventsPerSecond)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "healthy",
			"metrics": consumer.GetMetrics(),
		})
	})

	server := &http.Server{Addr: ":" + port}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("⚠️  Metrics server error: %v", err)
	}
}
