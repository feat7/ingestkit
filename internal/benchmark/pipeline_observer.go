package benchmark

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ObservationError invalidates visibility measurements, not independently
// verified delivery. It is distinct from request cancellation and SQL verification.
type ObservationError struct{ Cause error }

func (e *ObservationError) Error() string { return "SQL observation failed: " + e.Cause.Error() }
func (e *ObservationError) Unwrap() error { return e.Cause }

type observationResult struct {
	started, finished time.Time
	rows              []StoredObservation
	err               error
}

// Only the accounting loop requests work, and it permits at most one outstanding
// request. The buffered result therefore cannot block a cancelled worker's exit.
func observerWorker(ctx context.Context, ids []uuid.UUID, observe observer) (chan<- struct{}, <-chan observationResult, <-chan struct{}) {
	requests := make(chan struct{}, 1)
	results := make(chan observationResult, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer close(results)
		for {
			select {
			case <-ctx.Done():
				return
			case <-requests:
			}
			if ctx.Err() != nil {
				return
			}
			start := time.Now()
			rows, err := observe(ctx, ids)
			results <- observationResult{started: start, finished: time.Now(), rows: rows, err: err}
		}
	}()
	return requests, results, done
}

func observationOnly(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(*ObservationError); ok {
		return true
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		children := joined.Unwrap()
		if len(children) == 0 {
			return false
		}
		for _, child := range children {
			if !observationOnly(child) {
				return false
			}
		}
		return true
	}
	return false
}

func verifiedObservationOnly(c Case, report any, err error) bool {
	pipeline, ok := report.(PipelineReport)
	if !ok || !observationOnly(err) || len(pipeline.Runs) != c.Repetitions {
		return false
	}
	for _, run := range pipeline.Runs {
		if !run.Correct || run.DeliveryStatus != "passed" {
			return false
		}
	}
	return true
}
