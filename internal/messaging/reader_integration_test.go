//go:build integration

package messaging

import (
	"context"
	"github.com/feat7/ingestkit/internal/event"
	"github.com/feat7/ingestkit/internal/persistence"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"os"
	"testing"
	"time"
)

func TestReaderStoreRoundTrip(t *testing.T) {
	url := os.Getenv("INGESTKIT_TEST_DATABASE_URL")
	require.NotEmpty(t, url)
	pool, err := pgxpool.New(context.Background(), url)
	require.NoError(t, err)
	defer pool.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	addr := brokerAddr(t)
	e := publisherEvent()
	p, err := NewPublisher(PublisherConfig{Brokers: []string{addr}, Topic: "ingestkit.events"}, nil)
	require.NoError(t, err)
	defer p.Close()
	require.NoError(t, p.Publish(ctx, []event.Envelope{e}))
	r, err := NewReader(ReaderConfig{Brokers: []string{addr}, Topic: "ingestkit.events", DLQTopic: "ingestkit.dlq", GroupID: "test-" + uuid.NewString()}, persistence.NewEventStore(pool), nil)
	require.NoError(t, err)
	defer r.Close()
	committed := make(chan error, 1)
	original := r.commit
	r.commit = func(c context.Context, records ...*kgo.Record) error {
		for _, rec := range records {
			v, er := event.DecodeEnvelope(rec.Value)
			if er == nil && v.EventID == e.EventID {
				var n int
				er = pool.QueryRow(c, "SELECT count(*) FROM ingestkit.events WHERE tenant_id=$1 AND event_id=$2", e.TenantID, e.EventID.String()).Scan(&n)
				if er != nil || n != 1 {
					committed <- fmtError("storage not visible before commit")
					return fmtError("storage missing")
				}
				er = original(c, records...)
				committed <- er
				return er
			}
		}
		return original(c, records...)
	}
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	select {
	case err := <-committed:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("reader did not commit")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("reader failed to cancel")
	}
}

type fmtError string

func (e fmtError) Error() string { return string(e) }
