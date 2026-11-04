package benchmark

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func observerConfig() PipelineConfig {
	return PipelineConfig{Rate: 20, Concurrency: 2, Duration: 100 * time.Millisecond, DrainDeadline: 30 * time.Millisecond, PollInterval: time.Millisecond, BatchSize: 1, PayloadBytes: 64, Pattern: "steady"}
}

func TestSlowObserverDoesNotStarveHTTPResults(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered := make(chan struct{}, 1)
	stopped := make(chan struct{}, 1)
	observe := func(ctx context.Context, ids []uuid.UUID) ([]StoredObservation, error) {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-ctx.Done()
		stopped <- struct{}{}
		return nil, ctx.Err()
	}
	done := make(chan PipelineRun, 1)
	go func() {
		r, _ := runPipelineSample(ctx, observerConfig(), 1, func(context.Context, []uuid.UUID) sendResult { return sendResult{Accepted: true} }, observe)
		done <- r
	}()
	<-entered
	select {
	case r := <-done:
		require.Equal(t, 2, r.Attempted)
		require.Len(t, r.RequestSamples, 2)
		require.Equal(t, "failed", r.ObservationStatus)
		select {
		case <-stopped:
		default:
			t.Fatal("observer was not joined")
		}
	case <-time.After(time.Second):
		t.Fatal("SQL observer held completed HTTP accounting past its window and drain budget")
	}
}

func TestObserverFailurePreservesACKsAndFinalVerification(t *testing.T) {
	r, e := runPipelineSample(context.Background(), observerConfig(), 1, func(context.Context, []uuid.UUID) sendResult { return sendResult{Accepted: true} }, func(context.Context, []uuid.UUID) ([]StoredObservation, error) {
		return nil, errors.New("probe unavailable")
	})
	require.Error(t, e)
	require.Equal(t, 2, r.Attempted)
	require.Len(t, r.AcceptedIDs, 2)
	require.NoError(t, finalizePipelineVerification(&r, PipelineVerification{StoredIDs: r.AcceptedIDs}, nil))
	require.Equal(t, "passed", r.DeliveryStatus)
	require.Equal(t, "failed", r.ObservationStatus)
	require.Nil(t, r.VisibilityMetric)
	require.Zero(t, r.Missing)
	require.True(t, r.Correct)
	require.Equal(t, "limited", r.Outcome)
}

func TestCancellationStopsAndJoinsObserver(t *testing.T) {
	for trial := 0; trial < 10; trial++ {
		ctx, cancel := context.WithCancel(context.Background())
		entered := make(chan struct{})
		stopped := make(chan struct{})
		done := make(chan struct{})
		go func() {
			defer close(done)
			_, _ = runPipelineSample(ctx, observerConfig(), 1, func(ctx context.Context, ids []uuid.UUID) sendResult {
				<-ctx.Done()
				return sendResult{Ambiguous: true}
			}, func(ctx context.Context, ids []uuid.UUID) ([]StoredObservation, error) {
				close(entered)
				<-ctx.Done()
				close(stopped)
				return nil, ctx.Err()
			})
		}()
		<-entered
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("cancelled accounting did not join and return")
		}
		select {
		case <-stopped:
		default:
			t.Fatal("observer still running after return")
		}
	}
}

func TestObserverHasOneInflightQuery(t *testing.T) {
	c := observerConfig()
	c.Rate = 10
	var active, peak atomic.Int32
	_, e := runPipelineSample(context.Background(), c, 1, func(context.Context, []uuid.UUID) sendResult { return sendResult{Accepted: true} }, func(ctx context.Context, ids []uuid.UUID) ([]StoredObservation, error) {
		n := active.Add(1)
		defer active.Add(-1)
		for old := peak.Load(); n > old; old = peak.Load() {
			if peak.CompareAndSwap(old, n) {
				break
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
		return []StoredObservation{{ID: ids[0], ReceivedAt: time.Now().Add(-time.Second)}}, nil
	})
	require.NoError(t, e)
	require.EqualValues(t, 1, peak.Load())
	require.Zero(t, active.Load())
}

func TestAmbiguousIDsKeepBoundedDeadlineWithoutFalseLoss(t *testing.T) {
	c := observerConfig()
	c.Rate = 10
	r, e := runPipelineSample(context.Background(), c, 1, func(context.Context, []uuid.UUID) sendResult { return sendResult{Ambiguous: true} }, func(ctx context.Context, ids []uuid.UUID) ([]StoredObservation, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	require.NoError(t, e, "ending an uncertainty budget is not an SQL observation failure")
	require.NoError(t, finalizePipelineVerification(&r, PipelineVerification{}, nil))
	require.Equal(t, "passed", r.DeliveryStatus)
	require.Equal(t, "partial", r.ObservationStatus)
	require.Equal(t, 1, r.AmbiguousUnobserved)
	require.Zero(t, r.Missing)
	require.GreaterOrEqual(t, r.DrainNS, c.DrainDeadline.Nanoseconds())
}

func TestPipelineReportsSchemaTwo(t *testing.T) {
	r, _ := RunPipeline(context.Background(), PipelineConfig{})
	raw, e := json.Marshal(r)
	require.NoError(t, e)
	var value map[string]any
	require.NoError(t, json.Unmarshal(raw, &value))
	require.Equal(t, float64(2), value["schema"])
}

func TestOldJournalIsNotResumedAsNewMeasurements(t *testing.T) {
	require.Error(t, (StudyJournal{Schema: 1}).Compatible(StudyIdentity{}))
}

func TestStudyRetainsObserverLimitedVerifiedRepetitions(t *testing.T) {
	report := PipelineReport{Runs: []PipelineRun{{Correct: true, DeliveryStatus: "passed"}, {Correct: true, DeliveryStatus: "passed"}, {Correct: true, DeliveryStatus: "passed"}}}
	err := &ObservationError{Cause: errors.New("SQL timeout")}
	require.True(t, verifiedObservationOnly(Case{Repetitions: 3}, report, err))
	require.False(t, verifiedObservationOnly(Case{Repetitions: 3}, report, errors.Join(err, errors.New("offset commit failed"))))
	report.Runs[2].DeliveryStatus = "failed"
	require.False(t, verifiedObservationOnly(Case{Repetitions: 3}, report, err))
}

func TestNoPollingAfterFinalRequiredObservation(t *testing.T) {
	c := observerConfig()
	c.Rate = 10
	var calls atomic.Int32
	r, e := runPipelineSample(context.Background(), c, 1, func(context.Context, []uuid.UUID) sendResult { return sendResult{Accepted: true} }, func(_ context.Context, ids []uuid.UUID) ([]StoredObservation, error) {
		calls.Add(1)
		return []StoredObservation{{ID: ids[0], ReceivedAt: time.Now().Add(-time.Second)}}, nil
	})
	require.NoError(t, e)
	require.Equal(t, 1, r.Attempted)
	require.EqualValues(t, 1, calls.Load(), "queries after the only required row was observed are redundant")
}

func TestVisibilityFailureSerializesUnavailableAndNoSQLCapacityRate(t *testing.T) {
	r := PipelineRun{ObservationStatus: "failed"}
	data, e := json.Marshal(r)
	require.NoError(t, e)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(data, &fields))
	require.Nil(t, fields["receipt_to_observed_storage_upper_bound"])
	require.NotContains(t, fields, "PersistedEPS")
}
