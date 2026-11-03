package persistence

import (
	"context"
	"fmt"
	"strings"

	"github.com/feat7/ingestkit/internal/event"
	"github.com/jackc/pgx/v5/pgxpool"
)

const WriteMethod = "batch-insert"

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
	// One statement is one atomic transaction. Splitting it into auto-committed
	// chunks would allow a failed batch to leave an unreported partial write.
	args := make([]any, 0, len(events)*6)
	values := make([]string, len(events))
	for i, e := range events {
		n := i * 6
		values[i] = fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d)", n+1, n+2, n+3, n+4, n+5, n+6)
		args = append(args, e.TenantID, e.EventID.String(), e.EventType, e.OccurredAt, e.ReceivedAt, []byte(e.Payload))
	}
	tag, err := s.pool.Exec(ctx, "INSERT INTO ingestkit.events(tenant_id,event_id,event_type,occurred_at,received_at,payload) VALUES "+strings.Join(values, ",")+" ON CONFLICT (tenant_id,event_id) DO NOTHING", args...)
	if err != nil {
		return WriteResult{}, err
	}
	return WriteResult{Inserted: tag.RowsAffected(), Duplicates: int64(len(events)) - tag.RowsAffected()}, nil
}
