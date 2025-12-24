package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/feat7/ingestkit/generated/consumer"
	"github.com/feat7/ingestkit/generated/storage"
	applogger "github.com/feat7/ingestkit/internal/logger"
	"github.com/feat7/ingestkit/internal/messaging"
	dlqstorage "github.com/feat7/ingestkit/internal/storage"
	partstorage "github.com/feat7/ingestkit/internal/storage"
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
	applogger.Setup()
	log.Info().Msg("🔄 IngestKit Consumer Worker starting...")

	// Configuration
	redpandaAddr := getEnv("REDPANDA_ADDR", defaultRedpandaAddr)
	topic := getEnv("REDPANDA_TOPIC", defaultTopic)
	groupID := getEnv("CONSUMER_GROUP_ID", defaultGroupID)

	// Validate messaging configuration (always validate, regardless of DB connection method)
	if err := validateMessagingConfig(redpandaAddr, topic, groupID); err != nil {
		log.Fatal().Err(err).Msg("Messaging configuration validation failed")
	}

	// Get database connection string (support both DATABASE_URL and individual params)
	var connStr string
	var dbHost, dbPort string
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL != "" {
		// Use DATABASE_URL if provided (12-factor app pattern)
		connStr = databaseURL
		log.Info().Msg("✓ Using DATABASE_URL for connection")
	} else {
		// Fallback to individual parameters
		dbHost = getEnv("DB_HOST", defaultDBHost)
		dbPort = getEnv("DB_PORT", defaultDBPort)
		dbName := getEnv("DB_NAME", defaultDBName)
		dbUser := getEnv("DB_USER", defaultDBUser)
		dbPassword := getEnv("DB_PASSWORD", defaultDBPassword)

		// Validate database configuration
		if err := validateDatabaseConfig(dbHost, dbPort, dbName, dbUser); err != nil {
			log.Fatal().Err(err).Msg("Database configuration validation failed")
		}

		// Create database writer with connection pooling
		connStr = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			dbHost, dbPort, dbUser, dbPassword, dbName)
	}

	writer, err := storage.NewWriter(connStr)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create writer")
	}
	defer writer.Close()

	if dbHost != "" && dbPort != "" {
		log.Info().Str("host", dbHost).Str("port", dbPort).Msg("✓ Connected to PostgreSQL (pool: 50 max conns)")
	} else {
		log.Info().Msg("✓ Connected to PostgreSQL (pool: 50 max conns)")
	}

	// Create DLQ writer
	dlqWriter, err := dlqstorage.NewDLQWriter(connStr)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create DLQ writer")
	}
	defer dlqWriter.Close()

	log.Info().Msg("✓ DLQ writer initialized")

	// Create partition manager for automatic partition creation
	partitionMgr := partstorage.NewPartitionManager(writer.GetPool())
	log.Info().Msg("✓ Partition manager initialized (auto-creates partitions on demand)")

	// Create generated batch handler (auto-generated from schema)
	generatedHandler := consumer.NewBatchHandler(writer)

	// Wrap generated handler with partition management and context
	batchHandler := func(ctx context.Context, envelopes []*messaging.EventEnvelope) error {
		// Ensure partitions exist for all tenants and event types in this batch
		tenantEventPairs := make(map[string]map[string]bool)
		for _, env := range envelopes {
			if tenantEventPairs[env.TenantID] == nil {
				tenantEventPairs[env.TenantID] = make(map[string]bool)
			}
			tenantEventPairs[env.TenantID][env.EventType] = true
		}

		// Create partitions concurrently for all unique (tenant, event_type) pairs
		for tenantID, eventTypes := range tenantEventPairs {
			for eventType := range eventTypes {
				tableName := partstorage.GetEventTableName(eventType)
				if err := partitionMgr.EnsurePartition(ctx, tableName, tenantID); err != nil {
					log.Error().
						Str("tenant_id", tenantID).
						Str("event_type", eventType).
						Err(err).
						Msg("Failed to ensure partition")
					return fmt.Errorf("failed to ensure partition for %s/%s: %w", tenantID, eventType, err)
				}
			}
		}

		// Now process the batch with partitions ready
		return generatedHandler.ProcessBatch(ctx, envelopes)
	}

	// Create consumer with batch processing
	consumerWorker, err := messaging.NewBatchConsumer(
		[]string{redpandaAddr},
		topic,
		groupID,
		batchHandler,
		messaging.DefaultConsumerConfig(),
		dlqWriter,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create consumer")
	}
	defer consumerWorker.Close()

	log.Info().Str("address", redpandaAddr).Str("topic", topic).Str("group", groupID).Msg("✓ Connected to Redpanda")

	// Start metrics HTTP server
	metricsPort := getEnv("METRICS_PORT", "8081")
	go startMetricsServer(metricsPort, consumerWorker)
	log.Info().Str("port", metricsPort).Msg("✓ Metrics server started (endpoint: /metrics)")

	log.Info().Msg("🚀 Consumer worker started with batch processing - waiting for events...")

	// Graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		<-sigChan
		log.Info().Msg("⏹️  Shutting down gracefully...")
		cancel()
	}()

	// Start consuming
	if err := consumerWorker.Start(ctx); err != nil && err != context.Canceled {
		log.Fatal().Err(err).Msg("Consumer error")
	}

	// Print final metrics
	metrics := consumerWorker.GetMetrics()
	log.Info().
		Int64("processed", metrics.EventsProcessed).
		Int64("failed", metrics.EventsFailed).
		Int64("dlq", metrics.EventsDLQ).
		Int64("batches", metrics.BatchesProcessed).
		Msg("📊 Final metrics")

	log.Info().Msg("✅ Consumer stopped")
}

// validateMessagingConfig validates Redpanda/Kafka configuration
func validateMessagingConfig(redpandaAddr, topic, groupID string) error {
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

	// Validate consumer group ID
	if groupID == "" {
		return fmt.Errorf("CONSUMER_GROUP_ID cannot be empty")
	}

	log.Info().
		Str("redpanda", redpandaAddr).
		Str("topic", topic).
		Str("group", groupID).
		Msg("✓ Messaging configuration validated")

	return nil
}

// validateDatabaseConfig validates database connection parameters
func validateDatabaseConfig(dbHost, dbPort, dbName, dbUser string) error {
	// Validate database host
	if dbHost == "" {
		return fmt.Errorf("DB_HOST cannot be empty")
	}

	// Validate database port
	if dbPort == "" {
		return fmt.Errorf("DB_PORT cannot be empty")
	}
	port, err := strconv.Atoi(dbPort)
	if err != nil {
		return fmt.Errorf("DB_PORT must be a valid port number: %w", err)
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("DB_PORT must be between 1 and 65535, got %d", port)
	}

	// Validate database name
	if dbName == "" {
		return fmt.Errorf("DB_NAME cannot be empty")
	}

	// Validate database user
	if dbUser == "" {
		return fmt.Errorf("DB_USER cannot be empty")
	}

	log.Info().
		Str("database", fmt.Sprintf("%s@%s:%s/%s", dbUser, dbHost, dbPort, dbName)).
		Msg("✓ Database configuration validated")

	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// startMetricsServer starts an HTTP server for metrics exposition
func startMetricsServer(port string, consumer *messaging.Consumer) {
	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics := consumer.GetMetrics() // Returns a copy, thread-safe

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

	// Health endpoint - intentionally unprotected for monitoring/orchestration
	// (Kubernetes probes, load balancers, etc.)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "healthy",
			"metrics": consumer.GetMetrics(),
		})
	})

	server := &http.Server{Addr: ":" + port}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error().Err(err).Msg("Metrics server error")
	}
}
