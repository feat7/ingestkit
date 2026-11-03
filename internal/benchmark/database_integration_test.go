//go:build integration

package benchmark

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestAllWriteMethodsProduceSameIDSet(t *testing.T) {
	dsn := os.Getenv("INGESTKIT_TEST_DATABASE_URL")
	require.NotEmpty(t, dsn, "use scripts/test-integration.sh")
	p, e := pgxpool.New(context.Background(), dsn)
	require.NoError(t, e)
	defer p.Close()
	r, e := RunDatabase(context.Background(), DatabaseConfig{Pool: p, Workload: WorkloadConfig{Seed: 123, Count: 30, PayloadBytes: 128, DuplicateFraction: .2}, BatchSize: 7, Repetitions: 3})
	require.NoError(t, e)
	require.NoError(t, r.Validate())
	require.Len(t, r.Runs, 9)
	for _, v := range r.Runs {
		require.EqualValues(t, 24, v.Inserted)
		require.EqualValues(t, 6, v.Duplicates)
		require.True(t, v.Correct)
	}
	for _, n := range []int{0, -1} {
		_, e = RunDatabase(context.Background(), DatabaseConfig{Pool: p, BatchSize: n, Repetitions: 3})
		require.Error(t, e)
	}
}
