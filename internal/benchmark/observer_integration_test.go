//go:build integration

package benchmark

import (
	"context"
	"github.com/feat7/ingestkit/internal/event"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

func observerPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("INGESTKIT_TEST_DATABASE_URL")
	require.NotEmpty(t, dsn, "owned integration database required")
	p, e := pgxpool.New(context.Background(), dsn)
	require.NoError(t, e)
	t.Cleanup(p.Close)
	return p
}
func TestObserverSeesOutOfOrderCommits(t *testing.T) {
	ctx := context.Background()
	p := observerPool(t)
	tenant := "observer-" + uuid.NewString()
	a, b := uuid.New(), uuid.New()
	defer p.Exec(ctx, "DELETE FROM ingestkit.events WHERE tenant_id=$1", tenant)
	tx, e := p.Begin(ctx)
	require.NoError(t, e)
	defer tx.Rollback(ctx)
	_, e = tx.Exec(ctx, "INSERT INTO ingestkit.events(tenant_id,event_id,event_type,occurred_at,received_at,payload) VALUES($1,$2,'observer',now(),now()-interval '1 second','{}')", tenant, a.String())
	require.NoError(t, e)
	_, e = p.Exec(ctx, "INSERT INTO ingestkit.events(tenant_id,event_id,event_type,occurred_at,received_at,payload) VALUES($1,$2,'observer',now(),now(),'{}')", tenant, b.String())
	require.NoError(t, e)
	observer, e := NewPendingObserver(ctx, p, tenant, []uuid.UUID{a, b})
	require.NoError(t, e)
	defer observer.Close(ctx)
	rows, e := observer.Observe(ctx)
	require.NoError(t, e)
	require.Len(t, rows, 1)
	require.Equal(t, b, rows[0].ID)
	require.False(t, rows[0].ObservedAt.IsZero())
	require.NoError(t, tx.Commit(ctx))
	rows, e = observer.Observe(ctx)
	require.NoError(t, e)
	require.Len(t, rows, 1)
	require.Equal(t, a, rows[0].ID)
	rows, e = observer.Observe(ctx)
	require.NoError(t, e)
	require.Empty(t, rows)
}
func TestObserverCancellationReleasesConnection(t *testing.T) {
	ctx := context.Background()
	p := observerPool(t)
	observer, e := NewPendingObserver(ctx, p, "none", []uuid.UUID{uuid.New()})
	require.NoError(t, e)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, e = observer.Observe(canceled)
	require.Error(t, e)
	end, stop := context.WithTimeout(ctx, 3*time.Second)
	defer stop()
	require.NoError(t, observer.Close(end))
	require.NoError(t, p.Ping(end))
	require.Zero(t, p.Stat().AcquiredConns())
}
func TestIndependentReconciliationFindsMissingAndUnexpected(t *testing.T) {
	ctx := context.Background()
	p := observerPool(t)
	tenant := "verify-" + uuid.NewString()
	defer p.Exec(ctx, "DELETE FROM ingestkit.events WHERE tenant_id=$1", tenant)
	expected, e := Workload(WorkloadConfig{Namespace: tenant, PayloadProfile: "varied", Count: 2, PayloadBytes: 256})
	require.NoError(t, e)
	for i := range expected {
		expected[i].TenantID = tenant
	}
	_, e = writeMethod(ctx, p, "batch-insert", expected[:1])
	require.NoError(t, e)
	stranger := expected[0]
	stranger.EventID = uuid.New()
	_, e = writeMethod(ctx, p, "batch-insert", []event.Envelope{stranger})
	require.NoError(t, e)
	required := []uuid.UUID{expected[0].EventID, expected[1].EventID}
	v, e := VerifyPipelineRows(ctx, p, tenant, expected, required)
	require.Error(t, e)
	require.Equal(t, 1, v.Missing)
	require.Equal(t, 1, v.Unexpected)
	_, e = p.Exec(ctx, "DELETE FROM ingestkit.events WHERE tenant_id=$1 AND event_id=$2", tenant, stranger.EventID.String())
	require.NoError(t, e)
	_, e = writeMethod(ctx, p, "batch-insert", expected[1:])
	require.NoError(t, e)
	v, e = VerifyPipelineRows(ctx, p, tenant, expected, required)
	require.NoError(t, e)
	require.Len(t, v.StoredIDs, 2)
	_, e = p.Exec(ctx, "UPDATE ingestkit.events SET payload='{\"changed\":true}' WHERE tenant_id=$1 AND event_id=$2", tenant, expected[0].EventID.String())
	require.NoError(t, e)
	v, e = VerifyPipelineRows(ctx, p, tenant, expected, required)
	require.Error(t, e)
	require.Equal(t, 1, v.ContentMismatch)
}
