package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	// Kafka fetch configuration for high throughput
	maxFetchBytes          = 100 * 1024 * 1024 // 100MB - max total fetch size
	maxFetchBytesPerPart   = 50 * 1024 * 1024  // 50MB - max fetch per partition
	minFetchBytes          = 1                 // Start fetch immediately, don't wait
	maxConcurrentFetches   = 10                // Number of concurrent fetch requests

	// Logging configuration
	logPollInterval        = 100               // Log poll stats every N polls
)

// DLQWriter interface for writing failed events
type DLQWriter interface {
	WriteBatch(ctx context.Context, envelopes []*EventEnvelope, err error) error
}

// Consumer handles consuming events from Redpanda
type Consumer struct {
	client      *kgo.Client
	handler     BatchEventHandler
	config      *ConsumerConfig
	metrics     *ConsumerMetrics
	metricsMu   sync.RWMutex // Protects metrics access
	dlqWriter   DLQWriter
}

// ConsumerConfig holds consumer configuration
type ConsumerConfig struct {
	BatchSize       int           // Maximum batch size (default: 100)
	BatchTimeout    time.Duration // Maximum time to wait for batch (default: 1s)
	MaxRetries      int           // Maximum retry attempts (default: 3)
	RetryBackoffMin time.Duration // Minimum retry backoff (default: 1s)
	RetryBackoffMax time.Duration // Maximum retry backoff (default: 30s)
	Workers         int           // Number of concurrent workers (default: 4)
}

// DefaultConsumerConfig returns the default consumer configuration
func DefaultConsumerConfig() *ConsumerConfig {
	return &ConsumerConfig{
		BatchSize:       500,                   // Larger batch size for better throughput
		BatchTimeout:    20 * time.Millisecond, // Short timeout - flush quickly
		MaxRetries:      3,
		RetryBackoffMin: time.Second,
		RetryBackoffMax: 30 * time.Second,
		Workers:         4,
	}
}

// ConsumerMetrics tracks consumer performance
type ConsumerMetrics struct {
	EventsProcessed   int64
	EventsFailed      int64
	EventsDLQ         int64
	DBWriteErrors     int64 // Track DB write failures
	UnmarshalErrors   int64 // Track unmarshal failures separately
	BatchesProcessed  int64
	BatchesFailed     int64 // Track batch failures
	TotalLatencyMs    int64
	LastProcessedTime time.Time
}

// EventHandler processes a single consumed event
type EventHandler func(ctx context.Context, envelope *EventEnvelope) error

// BatchEventHandler processes a batch of events
type BatchEventHandler func(ctx context.Context, envelopes []*EventEnvelope) error

// RecordBatch groups records for batch processing
type RecordBatch struct {
	Records   []*kgo.Record
	Envelopes []*EventEnvelope
}

// NewConsumer creates a new Redpanda consumer with default config
func NewConsumer(brokers []string, topic string, groupID string, handler EventHandler, dlqWriter DLQWriter) (*Consumer, error) {
	// Wrap single event handler in batch handler
	batchHandler := func(ctx context.Context, envelopes []*EventEnvelope) error {
		for _, envelope := range envelopes {
			if err := handler(ctx, envelope); err != nil {
				return err
			}
		}
		return nil
	}

	return NewBatchConsumer(brokers, topic, groupID, batchHandler, DefaultConsumerConfig(), dlqWriter)
}

// NewBatchConsumer creates a new consumer with custom batch handler and configuration
func NewBatchConsumer(brokers []string, topic string, groupID string, handler BatchEventHandler, config *ConsumerConfig, dlqWriter DLQWriter) (*Consumer, error) {
	consumer := &Consumer{
		handler:   handler,
		config:    config,
		metrics:   &ConsumerMetrics{},
		dlqWriter: dlqWriter,
	}

	// OnPartitionsRevoked handler - ensure we commit before losing partitions
	onRevoked := func(ctx context.Context, c *kgo.Client, revoked map[string][]int32) {
		log.Printf("⚠️  Partitions revoked: %v - committing marked offsets before rebalance", revoked)
		if err := c.CommitMarkedOffsets(ctx); err != nil {
			log.Printf("❌ Failed to commit on revoke: %v", err)
		}
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(groupID),
		kgo.ConsumeTopics(topic),
		// Use AutoCommitMarks - only commit explicitly marked records
		kgo.AutoCommitMarks(),
		// Block rebalancing during poll/process cycle
		kgo.BlockRebalanceOnPoll(),
		// Handle partition revocation gracefully
		kgo.OnPartitionsRevoked(onRevoked),
		// Fetch configuration for high throughput
		kgo.FetchMaxBytes(maxFetchBytes),
		kgo.FetchMaxPartitionBytes(maxFetchBytesPerPart),
		kgo.FetchMinBytes(minFetchBytes),
		kgo.MaxConcurrentFetches(maxConcurrentFetches),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka client: %w", err)
	}

	consumer.client = client
	return consumer, nil
}

// Start begins consuming messages with batch processing
func (c *Consumer) Start(ctx context.Context) error {
	log.Printf("🚀 Starting consumer with batch_size=%d batch_timeout=%v workers=%d",
		c.config.BatchSize, c.config.BatchTimeout, c.config.Workers)

	pollCounter := 0 // Track polls for periodic logging

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Fetch records with timeout
			fetches := c.client.PollFetches(ctx)
			if fetches.IsClientClosed() {
				return fmt.Errorf("client closed")
			}

			// Handle fetch errors
			if errs := fetches.Errors(); len(errs) > 0 {
				for _, err := range errs {
					log.Printf("❌ Error fetching: %v", err.Err)
				}
				continue
			}

			// Count total records in fetches BEFORE iteration
			recordsInFetches := fetches.NumRecords()

			// Collect records into batch
			batch := &RecordBatch{
				Records:   make([]*kgo.Record, 0, c.config.BatchSize),
				Envelopes: make([]*EventEnvelope, 0, c.config.BatchSize),
			}

			// Track unmarshal failures and total fetched
			unmarshalFailures := 0
			totalFetched := 0

			// Use Records() to get all records as an iterator
			iter := fetches.RecordIter()
			for !iter.Done() {
				record := iter.Next()
				totalFetched++

				// Unmarshal envelope
				var envelope EventEnvelope
				if err := json.Unmarshal(record.Value, &envelope); err != nil {
					log.Printf("❌ Unmarshal failed (offset=%d partition=%d): %v",
						record.Offset, record.Partition, err)
					unmarshalFailures++
					c.metricsMu.Lock()
					c.metrics.UnmarshalErrors++
					c.metrics.EventsFailed++
					c.metricsMu.Unlock()
					// Do NOT add to batch - these records will not be marked for commit
					// and will be re-consumed on restart (at-least-once semantics)
					continue
				}

				batch.Records = append(batch.Records, record)
				batch.Envelopes = append(batch.Envelopes, &envelope)

				// Enforce batch size limit to prevent unbounded memory growth
				if len(batch.Records) >= c.config.BatchSize {
					// Process current batch before continuing
					if err := c.processBatch(ctx, batch); err != nil {
						log.Printf("❌ Failed to process batch: %v", err)
						c.metricsMu.Lock()
						c.metrics.BatchesFailed++
						c.metricsMu.Unlock()
						// Continue processing remaining records
					}
					// Reset batch for next chunk
					batch = &RecordBatch{
						Records:   make([]*kgo.Record, 0, c.config.BatchSize),
						Envelopes: make([]*EventEnvelope, 0, c.config.BatchSize),
					}
				}
			}

			// Log fetch statistics (only on errors or periodically to avoid log flooding)
			if recordsInFetches > 0 {
				pollCounter++
				hasError := unmarshalFailures > 0 || (recordsInFetches != totalFetched)
				shouldLog := hasError || (pollCounter%logPollInterval == 0)

				if shouldLog {
					log.Printf("📥 Poll #%d returned %d records -> iterated %d -> processed %d (unmarshal_failures=%d)",
						pollCounter, recordsInFetches, totalFetched, len(batch.Records), unmarshalFailures)
				}

				// CRITICAL: Always check for discrepancy
				if recordsInFetches != totalFetched {
					log.Printf("⚠️  DISCREPANCY: PollFetches returned %d but RecordIter only gave us %d! (missing: %d)",
						recordsInFetches, totalFetched, recordsInFetches-totalFetched)
				}
			}

			// Process batch if we have records
			if len(batch.Records) > 0 {
				if err := c.processBatch(ctx, batch); err != nil {
					log.Printf("❌ Failed to process batch: %v", err)
					c.metricsMu.Lock()
					c.metrics.BatchesFailed++
					c.metricsMu.Unlock()
					// Continue processing next batch
				}
			}

			// Allow rebalancing after processing this poll cycle
			// This is safe because we've already marked records for commit
			c.client.AllowRebalance()
		}
	}
}

func (c *Consumer) processBatch(ctx context.Context, batch *RecordBatch) error {
	startTime := time.Now()

	// Attempt to process with retries
	var lastErr error
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff
			backoff := c.calculateBackoff(attempt)
			log.Printf("🔄 Retry attempt %d/%d after %v", attempt, c.config.MaxRetries, backoff)
			time.Sleep(backoff)
		}

		// Call batch handler (DB write happens here)
		err := c.handler(ctx, batch.Envelopes)
		if err == nil {
			// Success - MARK records for auto-commit (only after successful DB write!)
			// AutoCommitMarks will handle the actual commit in background
			if len(batch.Records) > 0 {
				c.client.MarkCommitRecords(batch.Records...)
				log.Printf("✅ Marked %d records for commit (offsets: %d-%d)",
					len(batch.Records),
					batch.Records[0].Offset,
					batch.Records[len(batch.Records)-1].Offset)
			}

			// Update metrics
			duration := time.Since(startTime)
			c.metricsMu.Lock()
			c.metrics.BatchesProcessed++
			c.metrics.EventsProcessed += int64(len(batch.Envelopes))
			c.metrics.TotalLatencyMs += duration.Milliseconds()
			c.metrics.LastProcessedTime = time.Now()
			c.metricsMu.Unlock()

			log.Printf("✅ Batch processed successfully: %d events in %v",
				len(batch.Envelopes), duration)
			return nil
		}

		// DB write failed - log detailed error
		log.Printf("❌ DB write error (attempt %d/%d): %v | Batch size: %d events",
			attempt+1, c.config.MaxRetries+1, err, len(batch.Envelopes))
		c.metricsMu.Lock()
		c.metrics.DBWriteErrors++
		c.metricsMu.Unlock()

		// Classify error
		if !isRetriableError(err) {
			log.Printf("❌ Non-retriable error: %v", err)
			lastErr = err
			break
		}

		lastErr = err
	}

	// All retries exhausted - send to DLQ (logging happens in sendToDLQ)
	c.sendToDLQ(ctx, batch, lastErr)

	// Mark offsets for commit to avoid re-processing forever
	// DLQ records are safely stored and can be replayed manually later
	if len(batch.Records) > 0 {
		c.client.MarkCommitRecords(batch.Records...)
		log.Printf("💀 Marked %d DLQ records for commit (offsets: %d-%d)",
			len(batch.Records),
			batch.Records[0].Offset,
			batch.Records[len(batch.Records)-1].Offset)
	}

	c.metricsMu.Lock()
	c.metrics.EventsFailed += int64(len(batch.Envelopes))
	c.metrics.EventsDLQ += int64(len(batch.Envelopes))
	c.metricsMu.Unlock()

	return lastErr
}

func (c *Consumer) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: 1s, 2s, 4s, 8s, ...
	backoff := c.config.RetryBackoffMin * time.Duration(1<<uint(attempt-1))
	if backoff > c.config.RetryBackoffMax {
		backoff = c.config.RetryBackoffMax
	}
	return backoff
}

func isRetriableError(err error) bool {
	// Check for PostgreSQL error codes (using pgx error types)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		// Permanent errors - don't retry
		case "23505": // unique_violation (duplicate key)
			return false
		case "23503": // foreign_key_violation
			return false
		case "23502": // not_null_violation
			return false
		case "23514": // check_violation
			return false
		case "22P02": // invalid_text_representation (bad data format)
			return false
		case "42P01": // undefined_table
			return false
		case "42703": // undefined_column
			return false

		// Transient errors - retry
		case "40001": // serialization_failure (deadlock)
			return true
		case "40P01": // deadlock_detected
			return true
		case "53300": // too_many_connections
			return true
		case "53400": // configuration_limit_exceeded
			return true

		// Default for other PostgreSQL errors: retry
		default:
			return true
		}
	}

	// Check for network errors (transient - should retry)
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return true
		}
	}

	// Check for context errors (don't retry - deliberate cancellation)
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Fallback to string matching for application-level errors
	errStr := strings.ToLower(err.Error())

	// Permanent application errors
	if containsAny(errStr, []string{
		"unknown event type",
		"validation failed",
		"invalid data",
	}) {
		return false
	}

	// Default: retry for unknown errors (safer to retry than lose data)
	return true
}

func containsAny(str string, substrs []string) bool {
	for _, substr := range substrs {
		if containsString(str, substr) {
			return true
		}
	}
	return false
}

func containsString(str, substr string) bool {
	// Case-insensitive contains check
	return strings.Contains(strings.ToLower(str), strings.ToLower(substr))
}

func (c *Consumer) sendToDLQ(ctx context.Context, batch *RecordBatch, err error) {
	if c.dlqWriter == nil {
		log.Printf("⚠️  DLQ writer not configured - events will be lost!")
		for _, envelope := range batch.Envelopes {
			log.Printf("💀 DLQ (not written): event_type=%s tenant=%s error=%v",
				envelope.EventType, envelope.TenantID, err)
		}
		return
	}

	// Write to DLQ
	if dlqErr := c.dlqWriter.WriteBatch(ctx, batch.Envelopes, err); dlqErr != nil {
		log.Printf("❌ Failed to write to DLQ: %v", dlqErr)
		for _, envelope := range batch.Envelopes {
			log.Printf("💀 DLQ write failed: event_type=%s tenant=%s original_error=%v dlq_error=%v",
				envelope.EventType, envelope.TenantID, err, dlqErr)
		}
	} else {
		log.Printf("💀 Sent %d events to DLQ after %d retries: %v",
			len(batch.Envelopes), c.config.MaxRetries, err)
	}
}

// GetMetrics returns current consumer metrics
func (c *Consumer) GetMetrics() ConsumerMetrics {
	c.metricsMu.RLock()
	defer c.metricsMu.RUnlock()
	// Return a copy to prevent race conditions
	return *c.metrics
}

// Close closes the consumer
func (c *Consumer) Close() {
	c.client.Close()
}
