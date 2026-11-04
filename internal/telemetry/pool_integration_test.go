//go:build integration

package telemetry

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestPoolMetricsReportRealAcquisitionWaits(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	config, e := pgxpool.ParseConfig(os.Getenv("INGESTKIT_TEST_DATABASE_URL"))
	require.NoError(t, e)
	config.MaxConns = 1
	config.MinConns = 0
	p, e := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, e)
	defer p.Close()
	m := New()
	m.RegisterPool(p)
	hold, e := p.Acquire(ctx)
	require.NoError(t, e)
	defer hold.Release()
	value := func(name string) float64 {
		t.Helper()
		families, e := m.registry.Gather()
		require.NoError(t, e)
		for _, f := range families {
			if f.GetName() == name {
				metric := f.Metric[0]
				if metric.Gauge != nil {
					return metric.Gauge.GetValue()
				}
				return metric.Counter.GetValue()
			}
		}
		t.Fatalf("missing metric %s", name)
		return 0
	}
	require.EqualValues(t, 1, value("ingestkit_db_pool_max_conns"))
	require.EqualValues(t, 1, value("ingestkit_db_pool_acquired_conns"))
	waitCtx, stop := context.WithTimeout(ctx, 30*time.Millisecond)
	_, e = p.Acquire(waitCtx)
	stop()
	require.Error(t, e)
	require.Positive(t, value("ingestkit_db_pool_cancelled_acquires_total"))
	done := make(chan error, 1)
	began := make(chan struct{})
	go func() {
		close(began)
		second, e := p.Acquire(ctx)
		if e == nil {
			second.Release()
		}
		done <- e
	}()
	<-began
	time.Sleep(30 * time.Millisecond)
	hold.Release()
	require.NoError(t, <-done)
	require.Positive(t, value("ingestkit_db_pool_empty_acquires_total"))
	require.Positive(t, value("ingestkit_db_pool_empty_acquire_wait_seconds_total"))
	require.Zero(t, value("ingestkit_db_pool_acquired_conns"))
}
