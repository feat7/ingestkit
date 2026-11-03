package benchmark

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestPipelineAcceptanceIsNotPersistenceRate(t *testing.T) {
	id := uuid.New()
	r := PipelineRun{AcceptedIDs: []uuid.UUID{id}, WindowNS: int64(time.Second), TotalNS: int64(2 * time.Second)}
	require.Error(t, r.Reconcile())
	require.Zero(t, r.PersistedEPS)
	r.StoredIDs = []uuid.UUID{id}
	require.NoError(t, r.Reconcile())
	require.Equal(t, .5, r.PersistedEPS)
}
func TestPipelineReconcilesSetsNotOnlyCounts(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	r := PipelineRun{AttemptedIDs: []uuid.UUID{a}, AcceptedIDs: []uuid.UUID{a}, StoredIDs: []uuid.UUID{b}, TotalNS: 1}
	require.Error(t, r.Reconcile())
	require.Equal(t, 1, r.Missing)
	require.Equal(t, 1, r.Unexpected)
	r.StoredIDs = []uuid.UUID{a, a}
	require.Error(t, r.Reconcile())
	require.Equal(t, 1, r.DuplicateRows)
}
func TestPipelineReportsLoadGeneratorLimits(t *testing.T) {
	c := PipelineConfig{Rate: 1000, Concurrency: 1, Duration: 100 * time.Millisecond, DrainDeadline: 100 * time.Millisecond, PollInterval: time.Millisecond, BatchSize: 1, PayloadBytes: 64, Pattern: "steady"}
	send := func(ctx context.Context, in []uuid.UUID) sendResult {
		select {
		case <-ctx.Done():
		case <-time.After(20 * time.Millisecond):
		}
		return sendResult{IDs: in, Accepted: true}
	}
	observe := func(context.Context, []uuid.UUID) ([]StoredObservation, error) { return nil, nil }
	r, e := runPipelineSample(context.Background(), c, 1, send, observe)
	require.Error(t, e)
	require.Positive(t, r.GeneratorMissed)
	require.Less(t, r.Attempted, r.Planned)
}
func TestLatencyPercentilesUseDefinedSamples(t *testing.T) {
	p := Percentiles([]int64{5, 1, 4, 3, 2})
	require.EqualValues(t, 3, p.P50)
	require.EqualValues(t, 5, p.P95)
	require.EqualValues(t, 5, p.P99)
	require.EqualValues(t, 0, Percentiles(nil).P50)
}
func TestPipelineDrainDeadlineFails(t *testing.T) {
	c := PipelineConfig{Rate: 10, Concurrency: 1, Duration: 100 * time.Millisecond, DrainDeadline: 20 * time.Millisecond, PollInterval: time.Millisecond, BatchSize: 1, PayloadBytes: 64, Pattern: "steady"}
	r, e := runPipelineSample(context.Background(), c, 1, func(_ context.Context, ids []uuid.UUID) sendResult { return sendResult{IDs: ids, Accepted: true} }, func(context.Context, []uuid.UUID) ([]StoredObservation, error) { return nil, nil })
	require.Error(t, e)
	require.Equal(t, 1, r.Missing)
	require.GreaterOrEqual(t, r.TotalNS, r.WindowNS)
}
