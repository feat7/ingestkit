//go:build integration

package storage

import (
	"context"
	"encoding/json"
	"github.com/feat7/ingestkit/internal/event"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("INGESTKIT_TEST_DATABASE_URL")
	require.NotEmpty(t, url, "integration database is required")
	p, err := pgxpool.New(context.Background(), url)
	require.NoError(t, err)
	t.Cleanup(p.Close)
	require.NoError(t, p.Ping(context.Background()))
	return p
}
func testEvent(t *testing.T) event.Envelope {
	t.Helper()
	return event.Envelope{Input: event.Input{EventID: uuid.New(), EventType: "order_completed", OccurredAt: time.Now().UTC().Truncate(time.Microsecond), Payload: json.RawMessage("{\"amount\":25}")}, TenantID: uuid.NewString(), ReceivedAt: time.Now().UTC().Truncate(time.Microsecond)}
}
func TestWriteBatchPreservesEnvelope(t *testing.T) {
	p := testPool(t)
	e := testEvent(t)
	r, err := NewEventStore(p).WriteBatch(context.Background(), []event.Envelope{e})
	require.NoError(t, err)
	require.EqualValues(t, 1, r.Inserted)
	var id, typ string
	var payload []byte
	var receipt time.Time
	require.NoError(t, p.QueryRow(context.Background(), "SELECT event_id::text,event_type,payload,received_at FROM ingestkit.events WHERE tenant_id=$1", e.TenantID).Scan(&id, &typ, &payload, &receipt))
	require.Equal(t, e.EventID.String(), id)
	require.Equal(t, e.EventType, typ)
	require.JSONEq(t, string(e.Payload), string(payload))
	require.True(t, e.ReceivedAt.Equal(receipt))
}
func TestWriteBatchDeduplicatesWithinAndAcrossBatches(t *testing.T) {
	p := testPool(t)
	s := NewEventStore(p)
	e := testEvent(t)
	r, err := s.WriteBatch(context.Background(), []event.Envelope{e, e})
	require.NoError(t, err)
	require.EqualValues(t, 1, r.Inserted)
	require.EqualValues(t, 1, r.Duplicates)
	r, err = s.WriteBatch(context.Background(), []event.Envelope{e})
	require.NoError(t, err)
	require.EqualValues(t, 0, r.Inserted)
	require.EqualValues(t, 1, r.Duplicates)
}
func TestWriteBatchScopesIDsByTenant(t *testing.T) {
	p := testPool(t)
	e := testEvent(t)
	other := e
	other.TenantID = uuid.NewString()
	r, err := NewEventStore(p).WriteBatch(context.Background(), []event.Envelope{e, other})
	require.NoError(t, err)
	require.EqualValues(t, 2, r.Inserted)
}
func TestWriteBatchRollsBackOnFailure(t *testing.T) {
	p := testPool(t)
	e := testEvent(t)
	bad := testEvent(t)
	bad.Payload = json.RawMessage("[]")
	_, err := NewEventStore(p).WriteBatch(context.Background(), []event.Envelope{e, bad})
	require.Error(t, err)
	var n int
	require.NoError(t, p.QueryRow(context.Background(), "SELECT count(*) FROM ingestkit.events WHERE tenant_id=$1", e.TenantID).Scan(&n))
	require.Zero(t, n)
}
func TestWriteBatchCancellation(t *testing.T) {
	p := testPool(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := NewEventStore(p).WriteBatch(ctx, []event.Envelope{testEvent(t)})
	require.Error(t, err)
}
func TestMigrationPreservesLegacyTablesAndLedger(t *testing.T) {
	p := testPool(t)
	for _, table := range []string{"legacy_sentinel", "schema_migrations"} {
		var n int
		require.NoError(t, p.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&n))
		require.Equal(t, 1, n)
	}
	var n int
	require.NoError(t, p.QueryRow(context.Background(), "SELECT count(*) FROM ingestkit_schema_migrations WHERE version=1 AND NOT dirty").Scan(&n))
	require.Equal(t, 1, n)
}
