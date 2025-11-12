package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/twmb/franz-go/pkg/kgo"
)

// Consumer handles consuming events from Redpanda
type Consumer struct {
	client  *kgo.Client
	handler EventHandler
}

// EventHandler processes consumed events
type EventHandler func(ctx context.Context, envelope *EventEnvelope) error

// NewConsumer creates a new Redpanda consumer
func NewConsumer(brokers []string, topic string, groupID string, handler EventHandler) (*Consumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(groupID),
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()), // Start from latest for POC
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka client: %w", err)
	}

	return &Consumer{
		client:  client,
		handler: handler,
	}, nil
}

// Start begins consuming messages
func (c *Consumer) Start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Fetch records
			fetches := c.client.PollFetches(ctx)
			if fetches.IsClientClosed() {
				return fmt.Errorf("client closed")
			}

			// Handle errors
			if errs := fetches.Errors(); len(errs) > 0 {
				for _, err := range errs {
					log.Printf("Error fetching: %v", err.Err)
				}
				continue
			}

			// Process records
			fetches.EachRecord(func(record *kgo.Record) {
				if err := c.processRecord(ctx, record); err != nil {
					log.Printf("Error processing record: %v", err)
				}
			})
		}
	}
}

func (c *Consumer) processRecord(ctx context.Context, record *kgo.Record) error {
	// Unmarshal envelope
	var envelope EventEnvelope
	if err := json.Unmarshal(record.Value, &envelope); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	// Call handler
	if err := c.handler(ctx, &envelope); err != nil {
		return fmt.Errorf("handler failed: %w", err)
	}

	return nil
}

// Close closes the consumer
func (c *Consumer) Close() {
	c.client.Close()
}
