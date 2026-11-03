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

type PipelineConfig struct {
	Pool                                                    *pgxpool.Pool
	APIURL, APIKey, Tenant, Pattern                         string
	Seed                                                    int64
	Rate, Concurrency, BatchSize, PayloadBytes, Repetitions int
	Duration, Warmup, DrainDeadline, PollInterval           time.Duration
}

// No credentials or connection strings are included in the serializable config.
func PublicPipelineConfig(c PipelineConfig) map[string]any {
	return map[string]any{"tenant": c.Tenant, "pattern": c.Pattern, "seed": c.Seed, "target_events_per_second": c.Rate, "concurrency": c.Concurrency, "http_batch_size": c.BatchSize, "payload_bytes": c.PayloadBytes, "repetitions": c.Repetitions, "window_ns": c.Duration.Nanoseconds(), "warmup_ns": c.Warmup.Nanoseconds(), "drain_deadline_ns": c.DrainDeadline.Nanoseconds(), "poll_interval_ns": c.PollInterval.Nanoseconds()}
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

type BacklogSample struct {
	ElapsedNS         int64 `json:"elapsed_ns"`
	Accepted, Visible int
}
type PipelineRun struct {
	Repetition                                                                                   int `json:"repetition"`
	Planned, Attempted, Accepted, Persisted, Ambiguous, Rejected, GeneratorMissed, LateSchedules int
	Missing, Unexpected, DuplicateRows, AmbiguousPersisted, BacklogAtWindowEnd                   int
	WindowNS, ActiveNS, TotalNS, DrainNS, PollIntervalNS                                         int64
	AcceptedEPS, PersistedEPS                                                                    float64
	Ack                                                                                          Latency           `json:"ack_latency"`
	Visibility                                                                                   Latency           `json:"receipt_to_observed_storage_upper_bound"`
	AckNS                                                                                        []int64           `json:"ack_ns"`
	VisibilityNS                                                                                 []int64           `json:"visibility_ns"`
	AttemptedIDs, AcceptedIDs, AmbiguousIDs, StoredIDs                                           []uuid.UUID       `json:"-"`
	IDSetSHA256                                                                                  map[string]string `json:"id_set_sha256"`
	Backlog                                                                                      []BacklogSample   `json:"backlog"`
	Correct                                                                                      bool              `json:"correct"`
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
		r.PersistedEPS = float64(r.Persisted) * 1e9 / float64(r.TotalNS)
	}
	if r.WindowNS > 0 {
		r.AcceptedEPS = float64(r.Accepted) * 1e9 / float64(r.WindowNS)
	}
	r.Correct = r.Missing == 0 && r.Unexpected == 0 && r.DuplicateRows == 0 && r.TotalNS > 0
	if !r.Correct {
		return fmt.Errorf("reconciliation: %d missing, %d unexpected, %d duplicate stored rows", r.Missing, r.Unexpected, r.DuplicateRows)
	}
	return nil
}

type PipelineReport struct {
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
	IDs                 []uuid.UUID
	Accepted, Ambiguous bool
	ElapsedNS           int64
	Finished            time.Time
}
type sender func(context.Context, []uuid.UUID) sendResult
type observer func(context.Context, []uuid.UUID) ([]StoredObservation, error)

func runPipelineSample(ctx context.Context, c PipelineConfig, rep int, send sender, observe observer) (r PipelineRun, result error) {
	active := c.Duration
	if c.Pattern == "burst" {
		active = c.Duration / 4
	}
	count := int(float64(c.Rate) * c.Duration.Seconds())
	es, e := Workload(WorkloadConfig{Seed: c.Seed + int64(rep), Count: count, PayloadBytes: c.PayloadBytes})
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
				v.ElapsedNS = time.Since(at).Nanoseconds()
				v.Finished = time.Now()
				results <- v
			}
		}()
	}
	started := time.Now()
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
	// Every observation follows a completed SQL query on committed rows. The query
	// end minus received_at is an upper bound, including polling and query delay.
	sample := func() error {
		found, e := observe(ctx, ids)
		if e != nil {
			return e
		}
		now := time.Now()
		for _, v := range found {
			if !seen[v.ID] {
				seen[v.ID] = true
				observedAt[v.ID] = now
				r.StoredIDs = append(r.StoredIDs, v.ID)
				observedClock := v.ObservedAt
				if observedClock.IsZero() {
					observedClock = now
				} // Injected observers may share the host clock.
				lat := observedClock.Sub(v.ReceivedAt).Nanoseconds()
				if lat < 0 {
					return errors.New("negative visibility latency: clocks disagree")
				}
				r.VisibilityNS = append(r.VisibilityNS, lat)
			}
		}
		r.Backlog = append(r.Backlog, BacklogSample{ElapsedNS: now.Sub(started).Nanoseconds(), Accepted: len(acceptedAt), Visible: len(seen)})
		return nil
	}
	window := time.NewTimer(c.Duration)
	defer window.Stop()
	ctxDone := ctx.Done()
	windowElapsed := false
	workersDone := false
	var observationError error
	for !windowElapsed || !workersDone {
		select {
		case <-ctxDone:
			result = ctx.Err()
			windowElapsed = true
			ctxDone = nil
		case <-window.C:
			windowElapsed = true
		case v, ok := <-results:
			if !ok {
				workersDone = true
				results = nil
				continue
			}
			r.AttemptedIDs = append(r.AttemptedIDs, v.IDs...)
			r.Attempted += len(v.IDs)
			if v.Accepted {
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
		case <-tick.C:
			if observationError == nil {
				observationError = sample()
			}
		}
	}
	sched := <-scheduleDone
	r.GeneratorMissed = sched[0]
	r.LateSchedules = sched[1]
	drainStart := time.Now()
	deadline := drainStart.Add(c.DrainDeadline)
	for result == nil && observationError == nil {
		if e = sample(); e != nil {
			observationError = e
			break
		}
		missing := 0
		for id := range acceptedAt {
			if !seen[id] {
				missing++
			}
		}
		if missing == 0 {
			break
		}
		if time.Now().After(deadline) {
			result = errors.New("accepted events did not drain before deadline")
			break
		}
		select {
		case <-ctx.Done():
			result = ctx.Err()
		case <-tick.C:
		}
	}
	end := time.Now()
	r.TotalNS = end.Sub(started).Nanoseconds()
	r.DrainNS = max(0, end.Sub(windowEnd).Nanoseconds())
	for id, at := range acceptedAt {
		visible, ok := observedAt[id]
		if !at.After(windowEnd) && (!ok || visible.After(windowEnd)) {
			r.BacklogAtWindowEnd++
		}
	}
	r.Ack = Percentiles(r.AckNS)
	r.Visibility = Percentiles(r.VisibilityNS)
	reconcileErr := r.Reconcile()
	return r, errors.Join(result, observationError, reconcileErr)
}
func RunPipeline(ctx context.Context, c PipelineConfig) (PipelineReport, error) {
	r := PipelineReport{Kind: "pipeline", Config: PublicPipelineConfig(c), Notes: []string{"Target load is offered events/sec, not a capacity estimate. Burst offers the same count within the first quarter of the window.", "HTTP acknowledgement is measured per successful request, including client encoding/network/broker acknowledgement. No automatic load-driver retries.", "Storage latency is a per-event receipt-to-first-observed-committed-row upper bound. Receipt and SQL clock timestamps share the deployment VM clock; the SQL clock is queried after reading the committed-row result. Includes polling, query and clock-probe delay; persisted_at is not used. Host time only schedules load/backlog.", "PersistedEPS = all observed unique rows / (measured window plus request tail and drain); accepted EPS uses the measured window. See raw durations.", "ID sets are reconciled in memory; sorted unique IDs joined by newline (no trailing newline) are SHA-256 hashed for bounded public evidence. Recreate IDs from seed plus repetition and event index.",
		"Generator misses mean its bounded queue could not keep up; late schedules count scheduler delays beyond one request interval. Shared-host generator/observer compete with services."}}
	if c.Pool == nil || c.Rate <= 0 || c.Concurrency <= 0 || c.Concurrency > 1000 || c.Duration <= 0 || c.DrainDeadline <= 0 || c.PollInterval <= 0 || c.BatchSize <= 0 || c.BatchSize > 100 || c.Repetitions < 3 || c.Tenant == "" || (c.Pattern != "steady" && c.Pattern != "burst") {
		return r, errors.New("invalid pipeline configuration")
	}
	count := int(float64(c.Rate) * c.Duration.Seconds())
	if count <= 0 || count > 1000000 {
		return r, errors.New("pipeline requires 1..1000000 events per sample")
	}
	observe := func(ctx context.Context, ids []uuid.UUID) ([]StoredObservation, error) {
		rows, e := c.Pool.Query(ctx, "SELECT event_id,received_at FROM ingestkit.events WHERE tenant_id=$1 AND event_id=ANY($2::uuid[])", c.Tenant, ids)
		if e != nil {
			return nil, e
		}
		defer rows.Close()
		out := []StoredObservation{}
		for rows.Next() {
			var v StoredObservation
			if e = rows.Scan(&v.ID, &v.ReceivedAt); e != nil {
				return nil, e
			}
			out = append(out, v)
		}

		if e = rows.Err(); e != nil {
			return nil, e
		}
		rows.Close()
		if len(out) > 0 {
			var observedClock time.Time
			if e = c.Pool.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&observedClock); e != nil {
				return nil, e
			}
			for i := range out {
				out[i].ObservedAt = observedClock
			}
		}
		return out, nil
	}
	run := func(cfg PipelineConfig, rep int) (PipelineRun, error) {
		es, e := Workload(WorkloadConfig{Seed: cfg.Seed + int64(rep), Count: int(float64(cfg.Rate) * cfg.Duration.Seconds()), PayloadBytes: cfg.PayloadBytes})
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
		existing, e := observe(ctx, ids)
		if e != nil {
			return PipelineRun{}, e
		}
		if len(existing) > 0 {
			return PipelineRun{}, errors.New("seed IDs already exist; choose another seed or an isolated stack")
		}
		defer func() {
			end, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = c.Pool.Exec(end, "DELETE FROM ingestkit.events WHERE tenant_id=$1 AND event_id=ANY($2::uuid[])", c.Tenant, ids)
		}()
		client := &http.Client{Timeout: 8 * time.Second}
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
			req, e := http.NewRequestWithContext(ctx, "POST", cfg.APIURL+"/v1/events/batch", bytes.NewReader(raw))
			if e != nil {
				return v
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
			res, e := client.Do(req)
			if e != nil {
				v.Ambiguous = true
				return v
			}
			defer res.Body.Close()
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
		return runPipelineSample(ctx, cfg, rep, send, observe)
	}
	if c.Warmup > 0 {
		warm := c
		warm.Duration = c.Warmup
		warm.Seed = c.Seed - 1000000
		if _, e := run(warm, 0); e != nil {
			return r, fmt.Errorf("warm-up: %w", e)
		}
	}
	for i := 1; i <= c.Repetitions; i++ {
		v, e := run(c, i)
		r.Runs = append(r.Runs, v)
		if e != nil {
			return r, e
		}
		if v.Accepted == 0 {
			return r, errors.New("no accepted events; no useful pipeline measurement")
		}
	}
	return r, nil
}
