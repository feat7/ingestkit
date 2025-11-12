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
	SchemaVersion string                 `json:"schema_version"`
	EventType     string                 `json:"event_type"`
	TenantID      string                 `json:"tenant_id"`
	EventID       string                 `json:"event_id"`
	Timestamp     time.Time              `json:"timestamp"`
	Payload       map[string]interface{} `json:"payload"`
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

// Publish sends an event to Redpanda
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

	// Publish synchronously for POC (async with callbacks for production)
	results := p.client.ProduceSync(ctx, record)
	if err := results.FirstErr(); err != nil {
		return fmt.Errorf("failed to produce message: %w", err)
	}

	return nil
}

// Close closes the producer
func (p *Producer) Close() {
	p.client.Close()
}
