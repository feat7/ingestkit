package persistence

import (
	"context"
	"fmt"
	"github.com/feat7/ingestkit/internal/event"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EventStore struct{ pool *pgxpool.Pool }
type WriteResult struct {
	Inserted   int64
	Duplicates int64
}

func NewEventStore(pool *pgxpool.Pool) *EventStore   { return &EventStore{pool: pool} }
func (s *EventStore) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }
func (s *EventStore) WriteBatch(ctx context.Context, events []event.Envelope) (WriteResult, error) {
	if len(events) == 0 {
		return WriteResult{}, nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return WriteResult{}, err
	}
	defer tx.Rollback(context.Background())
	_, err = tx.Exec(ctx, "CREATE TEMP TABLE ingestkit_stage (LIKE ingestkit.events INCLUDING DEFAULTS) ON COMMIT DROP")
	if err != nil {
		return WriteResult{}, err
	}
	columns := []string{"tenant_id", "event_id", "event_type", "occurred_at", "received_at", "payload"}
	_, err = tx.CopyFrom(ctx, pgx.Identifier{"ingestkit_stage"}, columns, pgx.CopyFromSlice(len(events), func(i int) ([]any, error) {
		e := events[i]
		return []any{e.TenantID, e.EventID.String(), e.EventType, e.OccurredAt, e.ReceivedAt, []byte(e.Payload)}, nil
	}))
	if err != nil {
		return WriteResult{}, fmt.Errorf("stage events: %w", err)
	}
	tag, err := tx.Exec(ctx, "INSERT INTO ingestkit.events (tenant_id,event_id,event_type,occurred_at,received_at,payload) SELECT tenant_id,event_id,event_type,occurred_at,received_at,payload FROM ingestkit_stage ON CONFLICT (tenant_id,event_id) DO NOTHING")
	if err != nil {
		return WriteResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return WriteResult{}, err
	}
	return WriteResult{Inserted: tag.RowsAffected(), Duplicates: int64(len(events)) - tag.RowsAffected()}, nil
}
