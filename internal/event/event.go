// Package event defines the HTTP and broker event contract.
package event

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"time"
)

const (
	MaxEventBytes    = 256 << 10
	MaxEnvelopeBytes = MaxEventBytes + 4096
	MaxRequestBytes  = 1 << 20
	MaxBatchEvents   = 100
)

type Input struct {
	EventID    uuid.UUID       `json:"event_id"`
	EventType  string          `json:"event_type"`
	OccurredAt time.Time       `json:"occurred_at"`
	Payload    json.RawMessage `json:"payload"`
}
type Envelope struct {
	Input
	TenantID   string    `json:"tenant_id"`
	ReceivedAt time.Time `json:"received_at"`
}
type ValidationError struct {
	Message  string
	TooLarge bool
}

func (e *ValidationError) Error() string { return e.Message }
func IsTooLarge(err error) bool          { e, ok := err.(*ValidationError); return ok && e.TooLarge }
func invalid(message string) error       { return &ValidationError{Message: message} }
func validate(in Input) error {
	if in.EventID == uuid.Nil {
		return invalid("event_id must be a nonzero UUID")
	}
	if len(in.EventType) == 0 || len(in.EventType) > 128 || !validText(in.EventType) {
		return invalid("event_type must contain 1–128 bytes")
	}
	if in.OccurredAt.IsZero() {
		return invalid("occurred_at must be an RFC 3339 timestamp")
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(in.Payload, &obj) != nil || obj == nil {
		return invalid("payload must be a JSON object")
	}
	if !validJSONB(in.Payload) {
		return invalid("payload must be PostgreSQL-compatible JSON with nesting at most 64")
	}
	return nil
}
func Accept(in Input, tenant string, received time.Time) (Envelope, error) {
	if err := validate(in); err != nil {
		return Envelope{}, err
	}
	if !ValidTenant(tenant) || received.IsZero() {
		return Envelope{}, fmt.Errorf("invalid server event identity")
	}
	return Envelope{Input: in, TenantID: tenant, ReceivedAt: received.UTC()}, nil
}
