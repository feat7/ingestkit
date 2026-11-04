package benchmark

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/feat7/ingestkit/internal/event"
	"github.com/feat7/ingestkit/internal/runtimeinfo"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"math"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
)

var errObserverUnavailable = errors.New("observer temporarily unavailable")

type PipelineConfig struct {
	ReconnectObserver                                       bool
	SampleStarted                                           func(int, time.Time)
	SampleFinished                                          func(int) error
	HTTPClient                                              *http.Client
	Namespace, PayloadProfile                               string
	Pool                                                    *pgxpool.Pool
	APIURL, APIKey, Tenant, Pattern                         string
	Seed                                                    int64
	Rate, Concurrency, BatchSize, PayloadBytes, Repetitions int
	Duration, Warmup, DrainDeadline, PollInterval           time.Duration
}

// No credentials or connection strings are included in the serializable config.
func PublicPipelineConfig(c PipelineConfig) map[string]any {
	return map[string]any{"observer_reconnect": c.ReconnectObserver, "namespace": c.Namespace, "payload_profile": c.PayloadProfile, "https": strings.HasPrefix(c.APIURL, "https://"), "tenant": c.Tenant, "pattern": c.Pattern, "seed": c.Seed, "target_events_per_second": c.Rate, "concurrency": c.Concurrency, "http_batch_size": c.BatchSize, "payload_bytes": c.PayloadBytes, "repetitions": c.Repetitions, "window_ns": c.Duration.Nanoseconds(), "warmup_ns": c.Warmup.Nanoseconds(), "drain_deadline_ns": c.DrainDeadline.Nanoseconds(), "poll_interval_ns": c.PollInterval.Nanoseconds()}
}

type Latency struct {
	P50 int64 `json:"p50_ns"`
	P95 int64 `json:"p95_ns"`
	P99 int64 `json:"p99_ns"`
}

// Nearest-rank percentiles, including every measured sample; no interpolation.
func Percentiles(samples []int64) Latency {
	a := slices.Clone(samples)
	slices.Sort(a)
	if len(a) == 0 {
		return Latency{}
	}
	at := func(p float64) int64 { return a[max(0, int(math.Ceil(float64(len(a))*p))-1)] }
	return Latency{at(.5), at(.95), at(.99)}
}

type RequestObservation struct {
	StartedNS, FinishedNS, ElapsedNS int64
	Events, Status                   int
	Accepted, Ambiguous              bool
}

type BacklogSample struct {
	PendingAcknowledged                     int
	ObservationAvailable, FinalVerification bool
	ElapsedNS                               int64 `json:"elapsed_ns"`
	Accepted, Visible                       int
}
type PipelineRun struct {
	Connections                       ConnectionSnapshot        `json:"connections"`
	GeneratorLimits                   runtimeinfo.ProcessLimits `json:"generator_limits"`
	DriverPoolBefore, DriverPoolAfter map[string]any

	LoadStatus                                                                                   string   `json:"load_status"`
	DeliveryStatus                                                                               string   `json:"delivery_status"`
	ObservationStatus                                                                            string   `json:"observation_status"`
	VisibilityMetric                                                                             *Latency `json:"receipt_to_observed_storage_upper_bound"`
	HTTPCompletedNS, AccountingCompletedNS                                                       int64
	ObservationCompletedNS                                                                       *int64 `json:"observation_completed_ns"`
	ObserverSlowQueries, ObserverMaxQueryNS                                                      int64
	ObservationErrorClass                                                                        string `json:"observation_error_class,omitempty"`
	started                                                                                      time.Time
	ObserverOutages                                                                              int
	RequestSamples                                                                               []RequestObservation `json:"request_samples"`
	AttemptedInWindow, AttemptedAfterWindow, AcceptedInWindow, AcceptedAfterWindow               int
	ObserverPolls, ObserverQueries, ObserverQueryNS                                              int64
	ObserverLatePolls, ObserverMaxGapNS                                                          int64
	VisibilitySampleCount                                                                        int
	VerificationQueries                                                                          int
	ObservedEvents                                                                               int
	VerificationNS                                                                               int64
	AmbiguousUnobserved, ContentMismatch                                                         int
	Outcome                                                                                      string   `json:"outcome"`
	LimitReasons                                                                                 []string `json:"limit_reasons,omitempty"`
	Repetition                                                                                   int      `json:"repetition"`
	Planned, Attempted, Accepted, Persisted, Ambiguous, Rejected, GeneratorMissed, LateSchedules int
	Missing, Unexpected, DuplicateRows, AmbiguousPersisted, BacklogAtWindowEnd                   int
	WindowNS, ActiveNS, TotalNS, DrainNS, PollIntervalNS                                         int64
	AcceptedEPS, VerifiedCompletionEPS                                                           float64
	Ack                                                                                          Latency           `json:"ack_latency"`
	Visibility                                                                                   Latency           `json:"-"`
	AckNS                                                                                        []int64           `json:"ack_ns"`
	VisibilityNS                                                                                 []int64           `json:"visibility_ns"`
	AttemptedIDs, AcceptedIDs, AmbiguousIDs, StoredIDs                                           []uuid.UUID       `json:"-"`
	IDSetSHA256                                                                                  map[string]string `json:"id_set_sha256"`
	Backlog                                                                                      []BacklogSample   `json:"backlog"`
	Correct                                                                                      bool              `json:"correct"`
}

func finalizePipelineVerification(r *PipelineRun, verified PipelineVerification, verifyErr error) error {
	r.StoredIDs = verified.StoredIDs
	r.ContentMismatch = verified.ContentMismatch
	reconcileErr := r.Reconcile()
	r.DeliveryStatus = "passed"
	if verifyErr != nil || reconcileErr != nil {
		r.DeliveryStatus = "failed"
		r.Correct = false
		r.Outcome = "failed"
	}
	return errors.Join(verifyErr, reconcileErr)
}

func uniqueIDs(ids []uuid.UUID) map[uuid.UUID]bool {
	out := map[uuid.UUID]bool{}
	for _, id := range ids {
		out[id] = true
	}
	return out
}
func (r *PipelineRun) Reconcile() error {
	accepted, attempted, ambiguous := uniqueIDs(r.AcceptedIDs), uniqueIDs(r.AttemptedIDs), uniqueIDs(r.AmbiguousIDs)
	stored := uniqueIDs(r.StoredIDs)
	r.IDSetSHA256 = map[string]string{}
	for name, ids := range map[string][]uuid.UUID{"attempted": r.AttemptedIDs, "accepted": r.AcceptedIDs, "ambiguous": r.AmbiguousIDs, "stored": r.StoredIDs} {
		values := make([]string, 0, len(ids))
		for id := range uniqueIDs(ids) {
			values = append(values, id.String())
		}
		slices.Sort(values)
		sum := sha256.Sum256([]byte(strings.Join(values, "\n")))
		r.IDSetSHA256[name] = hex.EncodeToString(sum[:])
	}
	r.Missing = 0
	r.Unexpected = 0
	r.AmbiguousPersisted = 0
	r.DuplicateRows = len(r.StoredIDs) - len(stored)
	for id := range accepted {
		if !stored[id] {
			r.Missing++
		}
	}
	for id := range stored {
		if !attempted[id] && !accepted[id] {
			r.Unexpected++
		}
		if ambiguous[id] {
			r.AmbiguousPersisted++
		}
	}
	r.Accepted = len(accepted)
	r.Persisted = len(stored)
	if r.TotalNS > 0 {
		r.VerifiedCompletionEPS = float64(r.Persisted) * 1e9 / float64(r.TotalNS)
	}
	if r.WindowNS > 0 {
		r.AcceptedEPS = float64(r.AcceptedInWindow) * 1e9 / float64(r.WindowNS)
	}
	r.VisibilitySampleCount = len(r.VisibilityNS)
	r.Correct = r.Missing == 0 && r.Unexpected == 0 && r.DuplicateRows == 0 && r.ContentMismatch == 0 && r.TotalNS > 0

	r.AmbiguousUnobserved = len(ambiguous) - r.AmbiguousPersisted
	r.LimitReasons = nil
	for _, limit := range []struct {
		count  int
		reason string
	}{{r.GeneratorMissed, "generator_missed_events"}, {r.LateSchedules, "late_schedules"}, {r.Rejected, "http_rejections"}, {r.Ambiguous, "ambiguous_delivery"}, {r.AttemptedAfterWindow, "request_queue_tail"}, {r.AcceptedAfterWindow, "late_acknowledgements"}, {int(r.ObserverLatePolls), "observer_poll_delays"}, {r.ObserverOutages, "observer_unavailable"}} {
		if limit.count > 0 {
			r.LimitReasons = append(r.LimitReasons, limit.reason)
		}
	}
	if r.LoadStatus != "aborted" {
		r.LoadStatus = "met"
		if r.GeneratorMissed+r.LateSchedules+r.Rejected+r.Ambiguous+r.AttemptedAfterWindow+r.AcceptedAfterWindow > 0 {
			r.LoadStatus = "limited"
		}
	}
	if r.DeliveryStatus == "" {
		r.DeliveryStatus = "not_run"
	}
	if r.ObservationStatus == "" {
		r.ObservationStatus = "not_run"
	}
	if r.ObservationStatus == "failed" {
		r.LimitReasons = append(r.LimitReasons, "observer_failed")
	}
	if r.ObservationStatus == "partial" {
		r.LimitReasons = append(r.LimitReasons, "partial_observation")
	}
	if r.ObserverSlowQueries > 0 {
		r.LimitReasons = append(r.LimitReasons, "observer_query_delays")
	}
	if r.ObservationStatus != "complete" {
		r.VisibilityMetric = nil
	}
	r.Outcome = "complete"
	if len(r.LimitReasons) > 0 {
		r.Outcome = "limited"
	}
	if r.LoadStatus == "aborted" {
		r.Outcome = "failed"
	}
	if !r.Correct {
		r.Outcome = "failed"
		return fmt.Errorf("reconciliation: %d missing, %d unexpected, %d duplicate stored rows", r.Missing, r.Unexpected, r.DuplicateRows)
	}
	return nil
}

type PipelineReport struct {
	Schema int            `json:"schema"`
	Kind   string         `json:"kind"`
	Config map[string]any `json:"config"`
	Runs   []PipelineRun  `json:"runs"`
	Notes  []string       `json:"notes"`
}
type StoredObservation struct {
	ID         uuid.UUID
	ReceivedAt time.Time
	ObservedAt time.Time // SQL clock after the committed-row result has been read.
}
type sendResult struct {
	Status              int
	IDs                 []uuid.UUID
	Accepted, Ambiguous bool
	ElapsedNS           int64
	Started, Finished   time.Time
}
type sender func(context.Context, []uuid.UUID) sendResult
type observer func(context.Context, []uuid.UUID) ([]StoredObservation, error)

func runPipelineSample(ctx context.Context, c PipelineConfig, rep int, send sender, observe observer) (r PipelineRun, result error) {
	active := c.Duration
	if c.Pattern == "burst" {
		active = c.Duration / 4
	}
	count := int(float64(c.Rate) * c.Duration.Seconds())
	es, e := Workload(WorkloadConfig{Namespace: c.Namespace, PayloadProfile: c.PayloadProfile, Seed: c.Seed + int64(rep), Count: count, PayloadBytes: c.PayloadBytes})
	if e != nil {
		return r, e
	}
	ids := make([]uuid.UUID, len(es))
	for i, e := range es {
		ids[i] = e.EventID
	}
	r = PipelineRun{Repetition: rep, Planned: len(ids), WindowNS: c.Duration.Nanoseconds(), ActiveNS: active.Nanoseconds(), PollIntervalNS: c.PollInterval.Nanoseconds()}
	jobs := make(chan []uuid.UUID, c.Concurrency*2)
	results := make(chan sendResult, (len(ids)+c.BatchSize-1)/c.BatchSize)
	var wg sync.WaitGroup
	for i := 0; i < c.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				at := time.Now()
				v := send(ctx, job)
				v.IDs = job
				v.Started = at
				v.Finished = time.Now()
				v.ElapsedNS = v.Finished.Sub(at).Nanoseconds()
				results <- v
			}
		}()
	}
	started := time.Now()
	r.started = started
	if c.SampleStarted != nil {
		c.SampleStarted(rep, started)
	}
	windowEnd := started.Add(c.Duration)
	scheduleDone := make(chan [2]int, 1)
	go func() {
		defer close(jobs)
		missed, late := 0, 0
		for i := 0; i < len(ids); i += c.BatchSize {
			due := started.Add(time.Duration(float64(i) / float64(len(ids)) * float64(active)))
			delay := time.Until(due)
			if delay > 0 {
				timer := time.NewTimer(delay)
				select {
				case <-ctx.Done():
					timer.Stop()
					missed += len(ids) - i
					scheduleDone <- [2]int{missed, late}
					return
				case <-timer.C:
				}
			}
			job := ids[i:min(i+c.BatchSize, len(ids))]
			interval := time.Duration(float64(c.BatchSize) / float64(len(ids)) * float64(active))
			if time.Since(due) > interval {
				late++
			}
			if time.Now().After(started.Add(active)) {
				missed += len(ids) - i
				break
			}
			select {
			case jobs <- job:
			default:
				missed += len(job)
			}
		}
		scheduleDone <- [2]int{missed, late}
	}()
	go func() { wg.Wait(); close(results) }()
	tick := time.NewTicker(c.PollInterval)
	defer tick.Stop()
	seen := map[uuid.UUID]bool{}
	acceptedAt := map[uuid.UUID]time.Time{}
	observedAt := map[uuid.UUID]time.Time{}
	observeCtx, cancelObservation := context.WithCancel(ctx)
	requests, observations, observerDone := observerWorker(observeCtx, ids, observe)
	defer func() { cancelObservation(); <-observerDone }()
	inFlight := false
	var observationError error
	var previousPoll time.Time
	needsObservation := func() bool {
		for id := range acceptedAt {
			if !seen[id] {
				return true
			}
		}
		for _, id := range r.AmbiguousIDs {
			if !seen[id] {
				return true
			}
		}
		return false
	}
	recordObservation := func(v observationResult, expectedCancel bool) {
		inFlight = false
		r.ObserverPolls++
		elapsed := v.finished.Sub(v.started).Nanoseconds()
		r.ObserverQueryNS += elapsed
		r.ObserverMaxQueryNS = max(r.ObserverMaxQueryNS, elapsed)
		if elapsed > int64(2*c.PollInterval) {
			r.ObserverSlowQueries++
		}
		if !previousPoll.IsZero() {
			gap := v.started.Sub(previousPoll).Nanoseconds()
			r.ObserverMaxGapNS = max(r.ObserverMaxGapNS, gap)
			if gap > int64(2*c.PollInterval) {
				r.ObserverLatePolls++
			}
		}
		previousPoll = v.started
		available := v.err == nil
		if v.err != nil {
			if c.ReconnectObserver && errors.Is(v.err, errObserverUnavailable) {
				r.ObserverOutages++
			} else if !(expectedCancel && errors.Is(v.err, context.Canceled)) {
				observationError = v.err
			}
		}
		for _, row := range v.rows {
			if seen[row.ID] {
				continue
			}
			seen[row.ID] = true
			observedAt[row.ID] = v.finished
			r.StoredIDs = append(r.StoredIDs, row.ID)
			clock := row.ObservedAt
			if clock.IsZero() {
				clock = v.finished
			}
			latency := clock.Sub(row.ReceivedAt).Nanoseconds()
			if latency < 0 {
				observationError = errors.New("negative visibility latency: clocks disagree")
				continue
			}
			r.VisibilityNS = append(r.VisibilityNS, latency)
		}
		r.Backlog = append(r.Backlog, BacklogSample{ElapsedNS: v.finished.Sub(started).Nanoseconds(), Visible: len(seen), ObservationAvailable: available})
	}
	requestObservation := func() {
		if !inFlight && observationError == nil {
			requests <- struct{}{}
			inFlight = true
		}
	}
	recordRequest := func(v sendResult) {
		point := RequestObservation{StartedNS: v.Started.Sub(started).Nanoseconds(), FinishedNS: v.Finished.Sub(started).Nanoseconds(), ElapsedNS: v.ElapsedNS, Events: len(v.IDs), Status: v.Status, Accepted: v.Accepted, Ambiguous: v.Ambiguous}
		r.RequestSamples = append(r.RequestSamples, point)
		r.HTTPCompletedNS = max(r.HTTPCompletedNS, point.FinishedNS)
		r.AttemptedIDs = append(r.AttemptedIDs, v.IDs...)
		r.Attempted += len(v.IDs)
		if v.Started.After(windowEnd) {
			r.AttemptedAfterWindow += len(v.IDs)
		} else {
			r.AttemptedInWindow += len(v.IDs)
		}
		if v.Accepted {
			if v.Finished.After(windowEnd) {
				r.AcceptedAfterWindow += len(v.IDs)
			} else {
				r.AcceptedInWindow += len(v.IDs)
			}
			r.AcceptedIDs = append(r.AcceptedIDs, v.IDs...)
			r.AckNS = append(r.AckNS, v.ElapsedNS)
			for _, id := range v.IDs {
				acceptedAt[id] = v.Finished
			}
		} else if v.Ambiguous {
			r.Ambiguous += len(v.IDs)
			r.AmbiguousIDs = append(r.AmbiguousIDs, v.IDs...)
		} else {
			r.Rejected += len(v.IDs)
		}
	}
	window := time.NewTimer(c.Duration)
	defer window.Stop()
	ctxDone := ctx.Done()
	windowElapsed, workersDone := false, false
	for !windowElapsed || !workersDone {
		select {
		case <-ctxDone:
			result = ctx.Err()
			windowElapsed = true
			ctxDone = nil
			r.LoadStatus = "aborted"
		case <-window.C:
			windowElapsed = true
		case v, ok := <-results:
			if !ok {
				workersDone = true
				results = nil
				continue
			}
			recordRequest(v)
		case v, ok := <-observations:
			if !ok {
				observations = nil
				continue
			}
			recordObservation(v, false)
		case <-tick.C:
			if len(seen) < len(ids) && (!workersDone || needsObservation()) {
				requestObservation()
			}
		}
	}
	r.AccountingCompletedNS = time.Since(started).Nanoseconds()
	sched := <-scheduleDone
	r.GeneratorMissed, r.LateSchedules = sched[0], sched[1]
	drain := time.NewTimer(c.DrainDeadline)
	defer drain.Stop()
	if needsObservation() {
		requestObservation()
	}
	for result == nil && observationError == nil && needsObservation() {
		select {
		case <-ctx.Done():
			result = ctx.Err()
			r.LoadStatus = "aborted"
		case <-drain.C:
			for id := range acceptedAt {
				if !seen[id] {
					observationError = errors.New("acknowledged events not observed before deadline")
					break
				}
			}
			goto drained
		case v, ok := <-observations:
			if !ok {
				observations = nil
				continue
			}
			recordObservation(v, false)
		case <-tick.C:
			requestObservation()
		}
	}
drained:
	expectedCancel := result == nil && observationError == nil
	for id := range acceptedAt {
		if !seen[id] {
			expectedCancel = false
			break
		}
	}
	cancelObservation()
	<-observerDone
	if observations != nil {
		for v := range observations {
			recordObservation(v, expectedCancel)
		}
	}
	r.ObservationStatus = "complete"
	if observationError != nil || result != nil {
		r.ObservationStatus = "failed"
		r.ObservationErrorClass = "query_failed"
		if errors.Is(observationError, context.DeadlineExceeded) {
			r.ObservationErrorClass = "timeout"
		}
	} else if needsObservation() {
		r.ObservationStatus = "partial"
	}
	if len(acceptedAt) > 0 {
		var last time.Time
		complete := true
		for id := range acceptedAt {
			at, ok := observedAt[id]
			if !ok {
				complete = false
				break
			}
			if at.After(last) {
				last = at
			}
		}
		if complete {
			elapsed := last.Sub(started).Nanoseconds()
			r.ObservationCompletedNS = &elapsed
		}
	}
	r.DeliveryStatus = "not_run"
	end := time.Now()
	r.ObservedEvents = len(seen)
	r.TotalNS = end.Sub(started).Nanoseconds()
	r.DrainNS = max(0, end.Sub(windowEnd).Nanoseconds())
	for id, at := range acceptedAt {
		visible, ok := observedAt[id]
		if !at.After(windowEnd) && (!ok || visible.After(windowEnd)) {
			r.BacklogAtWindowEnd++
		}
	}
	// Reconstruct acknowledgement counts from actual worker finish times. A
	// blocked SQL probe must not shift completed ACKs to when its result is read.
	finishes := make([]time.Time, 0, len(acceptedAt))
	type pendingChange struct {
		at    time.Time
		delta int
	}
	changes := []pendingChange{}
	for id, at := range acceptedAt {
		finishes = append(finishes, at)
		observed, ok := observedAt[id]
		if !ok || observed.After(at) {
			changes = append(changes, pendingChange{at, 1})
			if ok {
				changes = append(changes, pendingChange{observed, -1})
			}
		}
	}
	slices.SortFunc(finishes, func(a, b time.Time) int { return a.Compare(b) })
	slices.SortFunc(changes, func(a, b pendingChange) int { return a.at.Compare(b.at) })
	fi, ci, pendingCount := 0, 0, 0
	for i := range r.Backlog {
		at := started.Add(time.Duration(r.Backlog[i].ElapsedNS))
		for fi < len(finishes) && !finishes[fi].After(at) {
			fi++
		}
		for ci < len(changes) && !changes[ci].at.After(at) {
			pendingCount += changes[ci].delta
			ci++
		}
		r.Backlog[i].Accepted = fi
		r.Backlog[i].PendingAcknowledged = pendingCount
	}
	r.Ack = Percentiles(r.AckNS)
	r.Visibility = Percentiles(r.VisibilityNS)
	if r.ObservationStatus == "complete" && len(r.VisibilityNS) > 0 {
		r.VisibilityMetric = &r.Visibility
	}
	reconcileErr := r.Reconcile()
	if observationError != nil {
		return r, errors.Join(result, &ObservationError{Cause: observationError})
	}
	return r, errors.Join(result, reconcileErr)
}
func RunPipeline(ctx context.Context, c PipelineConfig) (PipelineReport, error) {
	if c.Namespace == "" {
		c.Namespace = uuid.NewString()
	}
	if c.PayloadProfile == "" {
		c.PayloadProfile = "varied"
	}
	r := PipelineReport{Schema: 2, Kind: "pipeline", Config: PublicPipelineConfig(c), Notes: []string{"Target load is offered events/sec, not a capacity estimate. Burst offers the same count within the first quarter of the window.", "HTTP acknowledgement is measured per successful request, including client encoding/network/broker acknowledgement. No automatic load-driver retries. Visibility percentiles use only first observations made by the pending observer (ObservedEvents), not rows first found by final verification. AmbiguousUnobserved means absent at final bounded verification.", "Storage latency is a per-event receipt-to-first-observed-committed-row upper bound. Receipt and SQL clock timestamps share the deployment VM clock; the SQL clock is queried after reading the committed-row result. Includes polling, query and clock-probe delay; persisted_at is not used. Host time only schedules load/backlog.", "VerifiedCompletionEPS is an audit-completion rate, not SQL capacity: independently verified unique rows / (measured window plus request tail and complete observation/verification drain); accepted EPS counts only acknowledgements completed inside the measured window. Late starts and acknowledgements are reported separately. Limited outcomes are not achieved-load/capacity claims. See raw durations.", "ID sets are reconciled in memory; sorted unique IDs joined by newline (no trailing newline) are SHA-256 hashed for bounded public evidence. Recreate IDs from seed plus repetition and event index.",
		"Generator misses mean its bounded queue could not keep up; late schedules count scheduler delays beyond one request interval. Shared-host generator/observer compete with services."}}
	if c.Pool == nil || c.Rate <= 0 || c.Concurrency <= 0 || c.Concurrency > 1000 || c.Duration <= 0 || c.DrainDeadline <= 0 || c.PollInterval <= 0 || c.BatchSize <= 0 || c.BatchSize > 100 || c.Repetitions < 3 || c.Tenant == "" || (c.Pattern != "steady" && c.Pattern != "burst") {
		return r, errors.New("invalid pipeline configuration")
	}
	count := int(float64(c.Rate) * c.Duration.Seconds())
	if count <= 0 || count > 1000000 {
		return r, errors.New("pipeline requires 1..1000000 events per sample")
	}
	// Clone once per case so repetitions retain normal keep-alive behavior.
	client := &http.Client{Timeout: 8 * time.Second}
	if c.HTTPClient != nil {
		*client = *c.HTTPClient
	}
	transport := http.DefaultTransport.(*http.Transport)
	if client.Transport != nil {
		var ok bool
		transport, ok = client.Transport.(*http.Transport)
		if !ok {
			return r, errors.New("pipeline instrumentation requires an HTTP transport")
		}
	}
	recorder := NewConnectionRecorder()
	client.Transport = recorder.WrapTransport(transport)
	defer client.CloseIdleConnections()
	run := func(cfg PipelineConfig, rep int) (runResult PipelineRun, runErr error) {
		es, e := Workload(WorkloadConfig{Namespace: cfg.Namespace, PayloadProfile: cfg.PayloadProfile, Seed: cfg.Seed + int64(rep), Count: int(float64(cfg.Rate) * cfg.Duration.Seconds()), PayloadBytes: cfg.PayloadBytes})
		if e != nil {
			return PipelineRun{}, e
		}
		byID := map[uuid.UUID]event.Input{}
		ids := make([]uuid.UUID, 0, len(es))
		for _, v := range es {
			byID[v.EventID] = v.Input
			ids = append(ids, v.EventID)
		}
		// Refuse collisions rather than overwriting/removing somebody else's dataset.
		existing, e := VerifyPipelineRows(ctx, c.Pool, cfg.Tenant, es, nil)
		if e != nil && len(existing.StoredIDs) == 0 {
			return PipelineRun{}, e
		}
		if len(existing.StoredIDs) > 0 {
			return PipelineRun{}, errors.New("case namespace or event IDs already exist; choose a new namespace or an isolated stack")
		}
		defer func() {
			end, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = c.Pool.Exec(end, "DELETE FROM ingestkit.events WHERE tenant_id=$1 AND event_id=ANY($2::uuid[]) AND event_type=$3", c.Tenant, ids, es[0].EventType)
		}()
		defer func() {
			if cfg.SampleFinished != nil {
				runErr = errors.Join(runErr, cfg.SampleFinished(rep))
			}
		}()
		send := func(ctx context.Context, ids []uuid.UUID) sendResult {
			v := sendResult{}
			inputs := make([]event.Input, len(ids))
			for i, id := range ids {
				inputs[i] = byID[id]
			}
			raw, e := json.Marshal(map[string]any{"events": inputs})
			if e != nil {
				return v
			}
			requestCtx, requestEnd := context.WithTimeout(ctx, 8*time.Second)
			defer requestEnd()
			req, e := http.NewRequestWithContext(requestCtx, "POST", cfg.APIURL+"/v1/events/batch", bytes.NewReader(raw))
			if e != nil {
				return v
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
			req, finish := recorder.TraceRequest(req)
			res, e := client.Do(req)
			defer func() { finish(res, e) }()
			if e != nil {
				v.Ambiguous = true
				return v
			}
			defer res.Body.Close()
			v.Status = res.StatusCode
			if res.StatusCode != 202 {
				v.Ambiguous = res.StatusCode >= 500
				return v
			}
			var ack struct {
				IDs []uuid.UUID `json:"event_ids"`
			}
			if e = json.NewDecoder(io.LimitReader(res.Body, 65536)).Decode(&ack); e != nil || !slices.Equal(ids, ack.IDs) {
				v.Ambiguous = true
				return v
			}
			v.Accepted = true
			return v
		}
		pending, e := NewPendingObserver(ctx, c.Pool, cfg.Tenant, ids)
		if e != nil {
			return PipelineRun{}, e
		}
		totalQueries := int64(0)
		closeObserver := func() error {
			if pending == nil {
				return nil
			}
			q, _ := pending.Stats()
			totalQueries += q
			end, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			err := pending.Close(end)
			pending = nil
			return err
		}
		defer closeObserver()
		alreadyObserved := map[uuid.UUID]bool{}
		observe := func(ctx context.Context, _ []uuid.UUID) ([]StoredObservation, error) {
			probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			if pending == nil {
				remaining := []uuid.UUID{}
				for _, id := range ids {
					if !alreadyObserved[id] {
						remaining = append(remaining, id)
					}
				}
				var err error
				pending, err = NewPendingObserver(probeCtx, c.Pool, cfg.Tenant, remaining)
				if err != nil {
					if cfg.ReconnectObserver && ctx.Err() == nil {
						return nil, errObserverUnavailable
					}
					return nil, err
				}
				totalQueries += 2
			}
			rows, err := pending.Observe(probeCtx)
			if err != nil && cfg.ReconnectObserver && ctx.Err() == nil {
				_ = closeObserver()
				return nil, errObserverUnavailable
			}
			for _, row := range rows {
				alreadyObserved[row.ID] = true
			}
			return rows, err
		}
		limits := runtimeinfo.CaptureProcessLimits()
		before := DriverPoolSnapshot(c.Pool)
		recorder.BeginWindow()
		sample, sampleErr := runPipelineSample(ctx, cfg, rep, send, observe)
		sample.Connections = recorder.Snapshot()
		sample.GeneratorLimits = limits
		sample.DriverPoolBefore = before
		sample.DriverPoolAfter = DriverPoolSnapshot(c.Pool)
		closeErr := closeObserver()
		sample.ObserverQueries = totalQueries
		verificationStarted := time.Now()
		verifyCtx, verifyEnd := context.WithTimeout(ctx, c.DrainDeadline)
		verified, verifyErr := VerifyPipelineRows(verifyCtx, c.Pool, cfg.Tenant, es, sample.AcceptedIDs)
		verifyEnd()
		sample.VerificationNS = time.Since(verificationStarted).Nanoseconds()
		sample.VerificationQueries = 1
		sample.TotalNS = time.Since(sample.started).Nanoseconds()
		sample.DrainNS = max(0, sample.TotalNS-sample.WindowNS)
		reconcileErr := finalizePipelineVerification(&sample, verified, verifyErr)
		sample.Backlog = append(sample.Backlog, BacklogSample{ElapsedNS: sample.TotalNS, Accepted: sample.Accepted, Visible: sample.Persisted, PendingAcknowledged: sample.Missing, ObservationAvailable: verifyErr == nil, FinalVerification: true})
		if closeErr != nil {
			closeErr = &ObservationError{Cause: closeErr}
			sample.ObservationStatus = "failed"
			sample.VisibilityMetric = nil
			sample.Outcome = "limited"
		}
		result := errors.Join(sampleErr, closeErr, reconcileErr)
		if result != nil && !observationOnly(result) {
			sample.Outcome = "failed"
		}
		return sample, result
	}
	if c.Warmup > 0 {
		warm := c
		warm.Duration = c.Warmup
		warm.Seed = c.Seed - 1000000
		if _, e := run(warm, 0); e != nil {
			if !observationOnly(e) {
				return r, fmt.Errorf("warm-up: %w", e)
			}
			r.Notes = append(r.Notes, "Warm-up SQL observation failed; warm-up acknowledged IDs and content were independently verified before proceeding.")
		}
	}
	var observationErrors error
	for i := 1; i <= c.Repetitions; i++ {
		v, e := run(c, i)
		r.Runs = append(r.Runs, v)
		if e != nil {
			if !observationOnly(e) {
				return r, e
			}
			observationErrors = errors.Join(observationErrors, e)
		}
	}
	return r, observationErrors
}
