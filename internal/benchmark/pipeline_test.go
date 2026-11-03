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

func TestVisibilityUsesObserverClockDomain(t *testing.T) {
	c := PipelineConfig{Rate: 10, Concurrency: 1, Duration: 100 * time.Millisecond, DrainDeadline: 20 * time.Millisecond, PollInterval: time.Millisecond, BatchSize: 1, PayloadBytes: 64, Pattern: "steady"}
	// The Docker VM clock may be ahead of the host. Receipt and SQL observation
	// use that VM clock; the host clock still schedules load and backlog samples.
	receipt := time.Now().Add(time.Hour)
	r, e := runPipelineSample(context.Background(), c, 1, func(_ context.Context, ids []uuid.UUID) sendResult { return sendResult{Accepted: true} }, func(_ context.Context, ids []uuid.UUID) ([]StoredObservation, error) {
		return []StoredObservation{{ID: ids[0], ReceivedAt: receipt, ObservedAt: receipt.Add(7 * time.Millisecond)}}, nil
	})
	require.NoError(t, e)
	require.True(t, r.Correct)
	require.Equal(t, []int64{int64(7 * time.Millisecond)}, r.VisibilityNS)
}

func TestLateAckOutsideMeasuredWindow(t *testing.T) {
	c := PipelineConfig{Rate: 10, Concurrency: 1, Duration: 100 * time.Millisecond, DrainDeadline: 20 * time.Millisecond, PollInterval: time.Millisecond, BatchSize: 1, PayloadBytes: 64, Pattern: "steady"}
	send := func(context.Context, []uuid.UUID) sendResult {
		time.Sleep(120 * time.Millisecond)
		return sendResult{Accepted: true}
	}
	observe := func(_ context.Context, ids []uuid.UUID) ([]StoredObservation, error) {
		return []StoredObservation{{ID: ids[0], ReceivedAt: time.Now().Add(-time.Second)}}, nil
	}
	r, e := runPipelineSample(context.Background(), c, 1, send, observe)
	require.NoError(t, e)
	require.Equal(t, 0, r.AcceptedInWindow)
	require.Equal(t, 1, r.AcceptedAfterWindow)
	require.Zero(t, r.AcceptedEPS)
	require.Equal(t, "limited", r.Outcome)
}
func TestClientQueueTailClassified(t *testing.T) {
	c := PipelineConfig{Rate: 100, Concurrency: 1, Duration: 30 * time.Millisecond, DrainDeadline: 20 * time.Millisecond, PollInterval: time.Millisecond, BatchSize: 1, PayloadBytes: 64, Pattern: "steady"}
	send := func(context.Context, []uuid.UUID) sendResult {
		time.Sleep(50 * time.Millisecond)
		return sendResult{Accepted: true}
	}
	observe := func(_ context.Context, ids []uuid.UUID) ([]StoredObservation, error) {
		out := []StoredObservation{}
		for _, id := range ids {
			out = append(out, StoredObservation{ID: id, ReceivedAt: time.Now().Add(-time.Second)})
		}
		return out, nil
	}
	r, e := runPipelineSample(context.Background(), c, 1, send, observe)
	require.NoError(t, e)
	require.Equal(t, 1, r.AttemptedInWindow)
	require.Equal(t, 2, r.AttemptedAfterWindow)
	require.Equal(t, 3, r.AcceptedAfterWindow)
	require.Equal(t, "limited", r.Outcome)
}

func TestAmbiguousAbsenceIsUnknownAtDeadline(t *testing.T) {
	c := PipelineConfig{Rate: 10, Concurrency: 1, Duration: 100 * time.Millisecond, DrainDeadline: 20 * time.Millisecond, PollInterval: time.Millisecond, BatchSize: 1, PayloadBytes: 64, Pattern: "steady"}
	r, e := runPipelineSample(context.Background(), c, 1, func(context.Context, []uuid.UUID) sendResult { return sendResult{Ambiguous: true} }, func(context.Context, []uuid.UUID) ([]StoredObservation, error) { return nil, nil })
	require.NoError(t, e)
	require.True(t, r.Correct)
	require.Equal(t, "limited", r.Outcome)
	require.Equal(t, 1, r.AmbiguousUnobserved)
	require.GreaterOrEqual(t, r.DrainNS, int64(c.DrainDeadline))
}

func TestSlowObserverReportsLatePolls(t *testing.T) {
	c := PipelineConfig{Rate: 10, Concurrency: 1, Duration: 100 * time.Millisecond, DrainDeadline: 20 * time.Millisecond, PollInterval: time.Millisecond, BatchSize: 1, PayloadBytes: 64, Pattern: "steady"}
	r, e := runPipelineSample(context.Background(), c, 1, func(context.Context, []uuid.UUID) sendResult { return sendResult{Accepted: true} }, func(_ context.Context, ids []uuid.UUID) ([]StoredObservation, error) {
		time.Sleep(5 * time.Millisecond)
		return []StoredObservation{{ID: ids[0], ReceivedAt: time.Now().Add(-time.Second)}}, nil
	})
	require.NoError(t, e)
	require.Positive(t, r.ObserverLatePolls)
	require.GreaterOrEqual(t, r.ObserverMaxGapNS, int64(5*time.Millisecond))
	require.Equal(t, "limited", r.Outcome)
}

func TestExpectedObserverOutageDoesNotEraseSuccessfulRecovery(t *testing.T) {
	c := PipelineConfig{ReconnectObserver: true, Rate: 10, Concurrency: 1, Duration: 100 * time.Millisecond, DrainDeadline: 20 * time.Millisecond, PollInterval: time.Millisecond, BatchSize: 1, PayloadBytes: 64, Pattern: "steady"}
	calls := 0
	r, e := runPipelineSample(context.Background(), c, 1, func(context.Context, []uuid.UUID) sendResult { return sendResult{Accepted: true} }, func(_ context.Context, ids []uuid.UUID) ([]StoredObservation, error) {
		calls++
		if calls <= 2 {
			return nil, errObserverUnavailable
		}
		return []StoredObservation{{ID: ids[0], ReceivedAt: time.Now().Add(-time.Second)}}, nil
	})
	require.NoError(t, e)
	require.True(t, r.Correct)
	require.Equal(t, 2, r.ObserverOutages)
	require.Equal(t, "limited", r.Outcome)
	require.False(t, r.Backlog[0].ObservationAvailable)
}
func TestBacklogUsesActualAckTimesAfterSlowObservation(t *testing.T) {
	c := PipelineConfig{Rate: 10, Concurrency: 2, Duration: 200 * time.Millisecond, DrainDeadline: 20 * time.Millisecond, PollInterval: time.Millisecond, BatchSize: 1, PayloadBytes: 64, Pattern: "steady"}
	calls := 0
	r, e := runPipelineSample(context.Background(), c, 1, func(context.Context, []uuid.UUID) sendResult { return sendResult{Accepted: true} }, func(_ context.Context, ids []uuid.UUID) ([]StoredObservation, error) {
		calls++
		if calls == 1 {
			time.Sleep(120 * time.Millisecond)
			return nil, nil
		}
		out := []StoredObservation{}
		for _, id := range ids {
			out = append(out, StoredObservation{ID: id, ReceivedAt: time.Now().Add(-time.Second)})
		}
		return out, nil
	})
	require.NoError(t, e)
	require.Equal(t, 2, r.Backlog[0].Accepted)
	require.Equal(t, 2, r.Backlog[0].PendingAcknowledged)
	require.Len(t, r.RequestSamples, 2)
	accepted := 0
	for _, sample := range r.RequestSamples {
		if sample.Accepted && sample.FinishedNS <= r.WindowNS {
			accepted += sample.Events
		}
		require.Equal(t, sample.FinishedNS-sample.StartedNS, sample.ElapsedNS)
	}
	require.Equal(t, r.AcceptedInWindow, accepted)
}
