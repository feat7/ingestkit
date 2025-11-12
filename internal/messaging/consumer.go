package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// DLQWriter interface for writing failed events
type DLQWriter interface {
	WriteBatch(ctx context.Context, envelopes []*EventEnvelope, err error) error
}

// Consumer handles consuming events from Redpanda
type Consumer struct {
	client    *kgo.Client
	handler   BatchEventHandler
	config    *ConsumerConfig
	metrics   *ConsumerMetrics
	dlqWriter DLQWriter
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
		BatchSize:       100,
		BatchTimeout:    time.Second,
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
	BatchesProcessed  int64
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
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(groupID),
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()),
		kgo.DisableAutoCommit(), // Disable auto-commit for manual offset management
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka client: %w", err)
	}

	return &Consumer{
		client:    client,
		handler:   handler,
		config:    config,
		metrics:   &ConsumerMetrics{},
		dlqWriter: dlqWriter,
	}, nil
}

// Start begins consuming messages with batch processing
func (c *Consumer) Start(ctx context.Context) error {
	log.Printf("🚀 Starting consumer with batch_size=%d batch_timeout=%v workers=%d",
		c.config.BatchSize, c.config.BatchTimeout, c.config.Workers)

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

			// Collect records into batch
			batch := &RecordBatch{
				Records:   make([]*kgo.Record, 0, c.config.BatchSize),
				Envelopes: make([]*EventEnvelope, 0, c.config.BatchSize),
			}

			fetches.EachRecord(func(record *kgo.Record) {
				// Unmarshal envelope
				var envelope EventEnvelope
				if err := json.Unmarshal(record.Value, &envelope); err != nil {
					log.Printf("❌ Failed to unmarshal event: %v", err)
					return
				}

				batch.Records = append(batch.Records, record)
				batch.Envelopes = append(batch.Envelopes, &envelope)
			})

			// Process batch if we have records
			if len(batch.Records) > 0 {
				if err := c.processBatch(ctx, batch); err != nil {
					log.Printf("❌ Failed to process batch: %v", err)
					// Continue processing next batch
				}
			}
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

		// Call batch handler
		err := c.handler(ctx, batch.Envelopes)
		if err == nil {
			// Success - commit offsets
			if err := c.client.CommitUncommittedOffsets(ctx); err != nil {
				log.Printf("⚠️  Failed to commit offsets: %v", err)
				// Don't fail the batch - offsets will be re-consumed (at-least-once semantics)
			}

			// Update metrics
			c.metrics.BatchesProcessed++
			c.metrics.EventsProcessed += int64(len(batch.Envelopes))
			c.metrics.TotalLatencyMs += time.Since(startTime).Milliseconds()
			c.metrics.LastProcessedTime = time.Now()

			log.Printf("✅ Batch processed successfully: %d events in %v",
				len(batch.Envelopes), time.Since(startTime))
			return nil
		}

		// Classify error
		if !isRetriableError(err) {
			log.Printf("❌ Non-retriable error: %v", err)
			lastErr = err
			break
		}

		lastErr = err
	}

	// All retries exhausted - send to DLQ
	log.Printf("💀 Sending batch to DLQ after %d retries: %v", c.config.MaxRetries, lastErr)
	c.sendToDLQ(ctx, batch, lastErr)

	// Still commit offsets to avoid re-processing forever
	if err := c.client.CommitUncommittedOffsets(ctx); err != nil {
		log.Printf("⚠️  Failed to commit offsets after DLQ: %v", err)
	}

	c.metrics.EventsFailed += int64(len(batch.Envelopes))
	c.metrics.EventsDLQ += int64(len(batch.Envelopes))

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
	// Classify errors as retriable or permanent
	errStr := err.Error()

	// Transient errors - retry
	if containsAny(errStr, []string{
		"connection refused",
		"connection reset",
		"timeout",
		"temporary failure",
		"deadlock",
		"too many connections",
	}) {
		return true
	}

	// Permanent errors - don't retry
	if containsAny(errStr, []string{
		"unknown event type",
		"validation failed",
		"invalid data",
		"constraint violation",
		"duplicate key",
	}) {
		return false
	}

	// Default: retry for unknown errors
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
	// Simple case-insensitive contains check
	return len(str) >= len(substr) && (str == substr ||
		(len(str) > len(substr) &&
			(containsString(str[1:], substr) || str[:len(substr)] == substr)))
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
		log.Printf("💀 Wrote %d events to DLQ: %v", len(batch.Envelopes), err)
	}
}

// GetMetrics returns current consumer metrics
func (c *Consumer) GetMetrics() *ConsumerMetrics {
	return c.metrics
}

// Close closes the consumer
func (c *Consumer) Close() {
	c.client.Close()
}
