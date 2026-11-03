package benchmark

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PendingObserver owns one session and removes an ID only after seeing its
// committed target row. Timestamps never act as a commit-order cursor.
type PendingObserver struct {
	mu               sync.Mutex
	conn             *pgxpool.Conn
	table, tenant    string
	queries, queryNS int64
}

func NewPendingObserver(ctx context.Context, pool *pgxpool.Pool, tenant string, ids []uuid.UUID) (*PendingObserver, error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	o := &PendingObserver{conn: conn, tenant: tenant, table: "ingestkit_pending_" + uuid.NewString()}
	fail := func(err error) (*PendingObserver, error) {
		end, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		raw := conn.Hijack()
		return nil, errors.Join(err, raw.Close(end))
	}
	if _, err = conn.Exec(ctx, "CREATE TEMP TABLE "+pgx.Identifier{o.table}.Sanitize()+"(event_id uuid PRIMARY KEY)"); err != nil {
		return fail(err)
	}
	unique := make([]uuid.UUID, 0, len(ids))
	seen := map[uuid.UUID]bool{}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	if _, err = conn.CopyFrom(ctx, pgx.Identifier{o.table}, []string{"event_id"}, pgx.CopyFromSlice(len(unique), func(i int) ([]any, error) { return []any{unique[i].String()}, nil })); err != nil {
		return fail(err)
	}
	return o, nil
}
func (o *PendingObserver) Observe(ctx context.Context) (out []StoredObservation, result error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.conn == nil {
		return nil, errors.New("observer is closed")
	}
	start := time.Now()
	defer func() { o.queryNS += time.Since(start).Nanoseconds() }()
	o.queries++
	tx, err := o.conn.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		end, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = tx.Rollback(end)
	}()
	o.queries++
	rows, err := tx.Query(ctx, "DELETE FROM "+pgx.Identifier{o.table}.Sanitize()+" p USING ingestkit.events e WHERE e.tenant_id=$1 AND e.event_id=p.event_id RETURNING e.event_id,e.received_at", o.tenant)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v StoredObservation
		if err = rows.Scan(&v.ID, &v.ReceivedAt); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if len(out) > 0 {
		var clock time.Time
		o.queries++
		if err = tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&clock); err != nil {
			return nil, err
		}
		for i := range out {
			out[i].ObservedAt = clock
		}
	}
	o.queries++
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return out, nil
}
func (o *PendingObserver) Stats() (int64, int64) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.queries, o.queryNS
}
func (o *PendingObserver) Close(ctx context.Context) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.conn == nil {
		return nil
	}
	conn := o.conn
	o.conn = nil
	_, err := conn.Exec(ctx, "DROP TABLE "+pgx.Identifier{o.table}.Sanitize())
	if err != nil {
		raw := conn.Hijack()
		end, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		// Closing the session destroys its temp table even if cancellation made
		// DROP impossible. Only a failed disposal is a cleanup error.
		return raw.Close(end)
	}
	conn.Release()
	return nil
}
