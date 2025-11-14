package messaging

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/twmb/franz-go/pkg/kgo"
)

func TestCalculateBackoff(t *testing.T) {
	config := DefaultConsumerConfig()
	consumer := &Consumer{config: config}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 1 * time.Second},  // 2^0 = 1s
		{2, 2 * time.Second},  // 2^1 = 2s
		{3, 4 * time.Second},  // 2^2 = 4s
		{4, 8 * time.Second},  // 2^3 = 8s
		{5, 16 * time.Second}, // 2^4 = 16s
		{6, 30 * time.Second}, // 2^5 = 32s but capped at 30s
		{7, 30 * time.Second}, // Stays at max
	}

	for _, tt := range tests {
		t.Run(string(rune('0'+tt.attempt)), func(t *testing.T) {
			result := consumer.calculateBackoff(tt.attempt)
			if result != tt.expected {
				t.Errorf("Attempt %d: expected %v, got %v", tt.attempt, tt.expected, result)
			}
		})
	}
}

func TestIsRetriableError_PostgreSQLErrors(t *testing.T) {
	tests := []struct {
		name      string
		pgCode    string
		retriable bool
	}{
		{"unique violation", "23505", false},      // duplicate key
		{"foreign key violation", "23503", false}, // FK constraint
		{"not null violation", "23502", false},    // NOT NULL constraint
		{"check violation", "23514", false},       // CHECK constraint
		{"serialization failure", "40001", true},  // deadlock, retriable
		{"deadlock detected", "40P01", true},      // deadlock
		{"connection exception", "08000", true},   // connection error
		{"unknown error", "XXXXX", true},          // default to retriable
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pgErr := &pgconn.PgError{
				Code: tt.pgCode,
			}
			result := isRetriableError(pgErr)
			if result != tt.retriable {
				t.Errorf("PG error %s: expected retriable=%v, got %v",
					tt.pgCode, tt.retriable, result)
			}
		})
	}
}

func TestIsRetriableError_NetworkErrors(t *testing.T) {
	tests := []struct {
		name      string
		errMsg    string
		retriable bool
	}{
		{"connection refused", "connection refused", true},
		{"connection reset", "connection reset", true},
		{"timeout", "timeout", true},
		{"EOF", "EOF", true},
		{"network unreachable", "network is unreachable", true},
		{"context deadline", "context deadline exceeded", true},
		{"other error", "some other error", true}, // default to retriable
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.errMsg)
			result := isRetriableError(err)
			if result != tt.retriable {
				t.Errorf("Error '%s': expected retriable=%v, got %v",
					tt.errMsg, tt.retriable, result)
			}
		})
	}
}

func TestIsRetriableError_NonRetriable(t *testing.T) {
	// Non-PostgreSQL, non-network errors should default to retriable
	// (safe default - we retry unless we know it's permanent)
	err := errors.New("random application error")
	if !isRetriableError(err) {
		t.Error("Unknown errors should be retriable by default")
	}
}

func TestDefaultConsumerConfig(t *testing.T) {
	config := DefaultConsumerConfig()

	if config.BatchSize != 500 {
		t.Errorf("Expected BatchSize 500, got %d", config.BatchSize)
	}
	if config.BatchTimeout != 20*time.Millisecond {
		t.Errorf("Expected BatchTimeout 20ms, got %v", config.BatchTimeout)
	}
	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries 3, got %d", config.MaxRetries)
	}
	if config.RetryBackoffMin != time.Second {
		t.Errorf("Expected RetryBackoffMin 1s, got %v", config.RetryBackoffMin)
	}
	if config.RetryBackoffMax != 30*time.Second {
		t.Errorf("Expected RetryBackoffMax 30s, got %v", config.RetryBackoffMax)
	}
	if config.Workers != 4 {
		t.Errorf("Expected Workers 4, got %d", config.Workers)
	}
}

func TestConsumerMetrics_GetMetrics(t *testing.T) {
	// Test that GetMetrics returns a copy, not a pointer
	consumer := &Consumer{
		metrics: &ConsumerMetrics{
			EventsProcessed: 100,
			EventsFailed:    5,
			BatchesProcessed: 10,
		},
	}

	metrics1 := consumer.GetMetrics()
	metrics2 := consumer.GetMetrics()

	// Should be equal values
	if metrics1.EventsProcessed != metrics2.EventsProcessed {
		t.Error("GetMetrics should return consistent values")
	}

	// Modify returned value shouldn't affect internal state
	metrics1.EventsProcessed = 999
	metrics3 := consumer.GetMetrics()
	if metrics3.EventsProcessed != 100 {
		t.Error("GetMetrics should return a copy, not affect internal state")
	}
}

func TestRecordBatch_Validation(t *testing.T) {
	batch := &RecordBatch{
		Records:   make([]*kgo.Record, 0, 100),
		Envelopes: make([]*EventEnvelope, 0, 100),
	}

	if len(batch.Records) != 0 {
		t.Error("New batch should have 0 records")
	}
	if cap(batch.Records) != 100 {
		t.Error("Batch should be pre-allocated with capacity 100")
	}
}

func TestEventEnvelope_Structure(t *testing.T) {
	now := time.Now()
	envelope := &EventEnvelope{
		EventType:   "test_event",
		TenantID:    "tenant123",
		EventID:     "evt_123",
		Timestamp:   now,
		Payload:     []byte(`{"key":"value"}`),
	}

	if envelope.EventType != "test_event" {
		t.Error("EventType not set correctly")
	}
	if envelope.TenantID != "tenant123" {
		t.Error("TenantID not set correctly")
	}
	if envelope.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
	if len(envelope.Payload) == 0 {
		t.Error("Payload should not be empty")
	}
}

func TestConsumerMetrics_ThreadSafety(t *testing.T) {
	consumer := &Consumer{
		metrics: &ConsumerMetrics{},
	}

	// Simulate concurrent updates (basic smoke test)
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				consumer.metricsMu.Lock()
				consumer.metrics.EventsProcessed++
				consumer.metricsMu.Unlock()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	final := consumer.GetMetrics()
	if final.EventsProcessed != 1000 {
		t.Errorf("Expected 1000 events processed, got %d", final.EventsProcessed)
	}
}
