// Package messaging provides Redpanda/Kafka producer and consumer implementations.
//
// Producer: Async publishing with error channels for low-latency event ingestion
// Consumer: Batch processing with smart batching (500 events OR 20ms timeout)
//
// Key features:
//   - At-least-once delivery guarantees
//   - Automatic retry with exponential backoff
//   - Dead letter queue for failed events
//   - Thread-safe metrics with RWMutex
package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// Producer handles publishing events to Redpanda
type Producer struct {
	client *kgo.Client
	topic  string
}

// EventEnvelope wraps an event for Redpanda
type EventEnvelope struct {
	SchemaVersion string          `json:"schema_version"`
	EventType     string          `json:"event_type"`
	TenantID      string          `json:"tenant_id"`
	EventID       string          `json:"event_id"`
	Timestamp     time.Time       `json:"timestamp"`
	Payload       json.RawMessage `json:"payload"` // Raw JSON for efficient marshaling
}

// NewProducer creates a new Redpanda producer
func NewProducer(brokers []string, topic string) (*Producer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.AllowAutoTopicCreation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka client: %w", err)
	}

	return &Producer{
		client: client,
		topic:  topic,
	}, nil
}

// Publish sends an event to Redpanda synchronously
func (p *Producer) Publish(ctx context.Context, envelope *EventEnvelope) error {
	// Marshal envelope to JSON
	data, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// Create Kafka record
	record := &kgo.Record{
		Topic: p.topic,
		Key:   []byte(envelope.TenantID), // Partition by tenant
		Value: data,
	}

	// Publish synchronously
	results := p.client.ProduceSync(ctx, record)
	if err := results.FirstErr(); err != nil {
		return fmt.Errorf("failed to produce message: %w", err)
	}

	return nil
}

// PublishAsync sends an event to Redpanda asynchronously
// Returns immediately, errors are sent to the provided error channel
func (p *Producer) PublishAsync(ctx context.Context, envelope *EventEnvelope, errChan chan<- error) {
	// Marshal envelope to JSON
	data, err := json.Marshal(envelope)
	if err != nil {
		if errChan != nil {
			errChan <- fmt.Errorf("failed to marshal event: %w", err)
		}
		return
	}

	// Create Kafka record
	record := &kgo.Record{
		Topic: p.topic,
		Key:   []byte(envelope.TenantID), // Partition by tenant
		Value: data,
	}

	// Publish asynchronously with callback
	p.client.Produce(ctx, record, func(r *kgo.Record, err error) {
		if err != nil && errChan != nil {
			errChan <- fmt.Errorf("failed to produce message: %w", err)
		}
	})
}

// PublishAsyncBatch sends multiple events to Redpanda asynchronously
// Returns immediately, errors are sent to the provided error channel
func (p *Producer) PublishAsyncBatch(ctx context.Context, envelopes []*EventEnvelope, errChan chan<- error) {
	for _, envelope := range envelopes {
		p.PublishAsync(ctx, envelope, errChan)
	}
}

// Flush waits for all async messages to be sent
func (p *Producer) Flush(ctx context.Context) error {
	return p.client.Flush(ctx)
}

// Close closes the producer
func (p *Producer) Close() {
	p.client.Close()
}
