package benchmark

// Preserve the original staging/COPY/merge transaction as an explicit comparison.
// This does not delegate to the deployed writer: changing that default must not
// silently relabel a different algorithm as COPY in recorded experiments.
import (
	"context"
	"fmt"
	"github.com/feat7/ingestkit/internal/event"
	"github.com/feat7/ingestkit/internal/persistence"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func writeCopyMerge(ctx context.Context, pool *pgxpool.Pool, events []event.Envelope) (persistence.WriteResult, error) {
	if len(events) == 0 {
		return persistence.WriteResult{}, nil
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return persistence.WriteResult{}, err
	}
	defer tx.Rollback(context.Background())
	_, err = tx.Exec(ctx, "CREATE TEMP TABLE ingestkit_stage (LIKE ingestkit.events INCLUDING DEFAULTS) ON COMMIT DROP")
	if err != nil {
		return persistence.WriteResult{}, err
	}
	columns := []string{"tenant_id", "event_id", "event_type", "occurred_at", "received_at", "payload"}
	_, err = tx.CopyFrom(ctx, pgx.Identifier{"ingestkit_stage"}, columns, pgx.CopyFromSlice(len(events), func(i int) ([]any, error) {
		e := events[i]
		return []any{e.TenantID, e.EventID.String(), e.EventType, e.OccurredAt, e.ReceivedAt, []byte(e.Payload)}, nil
	}))
	if err != nil {
		return persistence.WriteResult{}, fmt.Errorf("stage events: %w", err)
	}
	tag, err := tx.Exec(ctx, "INSERT INTO ingestkit.events (tenant_id,event_id,event_type,occurred_at,received_at,payload) SELECT tenant_id,event_id,event_type,occurred_at,received_at,payload FROM ingestkit_stage ON CONFLICT (tenant_id,event_id) DO NOTHING")
	if err != nil {
		return persistence.WriteResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return persistence.WriteResult{}, err
	}
	return persistence.WriteResult{Inserted: tag.RowsAffected(), Duplicates: int64(len(events)) - tag.RowsAffected()}, nil
}
