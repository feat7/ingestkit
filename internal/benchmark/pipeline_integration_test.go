//go:build integration

package benchmark

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

func TestBenchmarkCleanupPreservesUnownedData(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("INGESTKIT_TEST_DATABASE_URL")
	url := os.Getenv("INGESTKIT_TEST_API_URL")
	require.NotEmpty(t, dsn)
	require.NotEmpty(t, url)
	p, e := pgxpool.New(ctx, dsn)
	require.NoError(t, e)
	defer p.Close()
	id := uuid.New()
	_, e = p.Exec(ctx, "INSERT INTO ingestkit.events(tenant_id,event_id,event_type,occurred_at,received_at,payload) VALUES('demo',$1,'sentinel',now(),now(),'{}')", id.String())
	require.NoError(t, e)
	defer p.Exec(ctx, "DELETE FROM ingestkit.events WHERE tenant_id='demo' AND event_id=$1", id.String())
	r, e := RunPipeline(ctx, PipelineConfig{Pool: p, APIURL: url, APIKey: "local-demo-key", Tenant: "demo", Rate: 20, Concurrency: 2, Duration: time.Second, Warmup: 100 * time.Millisecond, DrainDeadline: 15 * time.Second, PollInterval: 25 * time.Millisecond, BatchSize: 2, PayloadBytes: 128, Pattern: "steady", Seed: 987, Repetitions: 3})
	require.NoError(t, e)
	require.Len(t, r.Runs, 3)
	raw, e := json.Marshal(r)
	require.NoError(t, e)
	require.NotContains(t, string(raw), "local-demo-key")
	require.NotContains(t, string(raw), dsn)
	var n int
	require.NoError(t, p.QueryRow(ctx, "SELECT count(*) FROM ingestkit.events WHERE event_id=$1", id.String()).Scan(&n))
	require.Equal(t, 1, n)
	for _, v := range r.Runs {
		require.Equal(t, 20, v.Accepted)
		require.Equal(t, 20, v.Persisted)
		require.Zero(t, v.Missing)
		require.Positive(t, v.Visibility.P50)
	}
}

func TestPipelineRefusesPreexistingCaseBeforeCleanup(t *testing.T) {
	ctx := context.Background()
	p := observerPool(t)
	ns := "collision-" + uuid.NewString()
	expected, e := Workload(WorkloadConfig{Namespace: ns, PayloadProfile: "varied", Seed: 43, Count: 2, PayloadBytes: 128})
	require.NoError(t, e)
	expected[0].TenantID = "demo"
	_, e = writeMethod(ctx, p, "batch-insert", expected[:1])
	require.NoError(t, e)
	defer p.Exec(ctx, "DELETE FROM ingestkit.events WHERE tenant_id='demo' AND event_id=$1", expected[0].EventID.String())
	_, e = RunPipeline(ctx, PipelineConfig{Pool: p, Namespace: ns, APIURL: os.Getenv("INGESTKIT_TEST_API_URL"), APIKey: "local-demo-key", Tenant: "demo", Rate: 20, Concurrency: 1, Duration: 100 * time.Millisecond, DrainDeadline: time.Second, PollInterval: 10 * time.Millisecond, BatchSize: 1, PayloadBytes: 128, Pattern: "steady", Seed: 42, Repetitions: 3})
	require.ErrorContains(t, e, "already exist")
	var n int
	require.NoError(t, p.QueryRow(ctx, "SELECT count(*) FROM ingestkit.events WHERE tenant_id='demo' AND event_id=$1", expected[0].EventID.String()).Scan(&n))
	require.Equal(t, 1, n, "pre-existing data must survive a refused run")
}
