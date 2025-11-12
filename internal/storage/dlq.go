package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/yourusername/ingestkit/internal/messaging"
)

// DLQWriter handles writing failed events to the dead letter queue
type DLQWriter struct {
	db *sql.DB
}

// NewDLQWriter creates a new DLQ writer
func NewDLQWriter(connStr string) (*DLQWriter, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DLQWriter{db: db}, nil
}

// WriteBatch writes a batch of failed events to the DLQ
func (d *DLQWriter) WriteBatch(ctx context.Context, envelopes []*messaging.EventEnvelope, err error) error {
	if len(envelopes) == 0 {
		return nil
	}

	query := `
		INSERT INTO ingestkit_meta.dead_letter_queue
		(event_type, tenant_id, event_payload, error_message, retry_count)
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
		_, execErr := d.db.ExecContext(ctx, query,
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

// Close closes the DLQ writer
func (d *DLQWriter) Close() error {
	return d.db.Close()
}
