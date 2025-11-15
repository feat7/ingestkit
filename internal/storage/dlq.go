// Package storage provides dead letter queue (DLQ) implementation.
//
// The DLQ stores events that failed processing after maximum retry attempts.
// Failed events are written to a PostgreSQL table with error details for
// later analysis and manual recovery.
//
// DLQ schema includes:
//   - Original event envelope (JSON)
//   - Error message and type
//   - Timestamp of failure
//   - Tenant ID for isolation
package storage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/feat7/ingestkit/internal/messaging"
)

// DLQWriter handles writing failed events to the dead letter queue
type DLQWriter struct {
	pool *pgxpool.Pool
}

// NewDLQWriter creates a new DLQ writer with connection pooling
func NewDLQWriter(connStr string) (*DLQWriter, error) {
	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DLQWriter{pool: pool}, nil
}

// WriteBatch writes a batch of failed events to the DLQ
func (d *DLQWriter) WriteBatch(ctx context.Context, envelopes []*messaging.EventEnvelope, err error) error {
	if len(envelopes) == 0 {
		return nil
	}

	query := `
		INSERT INTO ingestkit_meta.dead_letter_queue
		(event_type, tenant_id, event_data, error_message, retry_count)
		VALUES ($1, $2, $3, $4, $5)
	`

	// Write each envelope to DLQ
	for _, envelope := range envelopes {
		// Marshal payload to JSON
		payloadJSON, marshalErr := json.Marshal(envelope.Payload)
		if marshalErr != nil {
			// If we can't marshal, store error message
			payloadJSON = []byte(fmt.Sprintf("{\"error\": \"failed to marshal: %v\"}", marshalErr))
		}

		// Insert into DLQ
		_, execErr := d.pool.Exec(ctx, query,
			envelope.EventType,
			envelope.TenantID,
			payloadJSON,
			err.Error(),
			0, // Initial retry_count
		)

		if execErr != nil {
			return fmt.Errorf("failed to write to DLQ: %w", execErr)
		}
	}

	return nil
}

// Write writes a single failed event to the DLQ
func (d *DLQWriter) Write(ctx context.Context, envelope *messaging.EventEnvelope, err error) error {
	return d.WriteBatch(ctx, []*messaging.EventEnvelope{envelope}, err)
}

// Close closes the DLQ writer connection pool
func (d *DLQWriter) Close() {
	d.pool.Close()
}
