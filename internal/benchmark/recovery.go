package benchmark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/feat7/ingestkit/internal/event"
	"github.com/feat7/ingestkit/internal/runenv"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FaultEvent struct {
	Action, Service, ErrorClass string
	Repetition                  int
	ElapsedNS, PlannedNS        int64
	At                          time.Time
}
type OutboxPoint struct {
	ElapsedNS             int64
	Pending, Acknowledged int
}
type RecoveryRun struct {
	FirstAttemptStatus     string      `json:",omitempty"`
	FirstAttemptNS         int64       `json:",omitempty"`
	AmbiguousIDs           []uuid.UUID `json:"-"`
	Repetition             int
	Correct                bool
	Faults                 []FaultEvent
	Pipeline               *PipelineRun  `json:",omitempty"`
	Outbox                 []OutboxPoint `json:",omitempty"`
	AcceptedIDs, StoredIDs []uuid.UUID   `json:"-"`
}
type RecoveryReport struct {
	Kind  string
	Runs  []RecoveryRun
	Notes []string
}

func waitUntil(ctx context.Context, at time.Time) error {
	timer := time.NewTimer(max(0, time.Until(at)))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func restoreOwned(s *runenv.Stack, service string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return s.SetServiceRunning(ctx, service, true)
}
func RunRecoveryCase(ctx context.Context, s *runenv.Stack, p *pgxpool.Pool, c Case, seed int64, namespace string) (report RecoveryReport, result error) {
	report = RecoveryReport{Kind: c.Recovery, Notes: []string{"Availability experiment; not a capacity estimate. Actual control timestamps and reconciled ID sets are retained.", "A forced consumer kill does not claim to land in every database/offset commit window; the deterministic crash regression remains separate."}}
	if c.Kind != "recovery" || c.Repetitions != 3 {
		return report, errors.New("invalid recovery case")
	}
	if c.Recovery == "outbox" {
		return runOutboxRecovery(ctx, s, p, c, seed, namespace)
	}
	service := "postgres"
	if c.Recovery == "consumer" {
		service = "consumer"
	} else if c.Recovery != "database" {
		return report, errors.New("unknown recovery case")
	}
	defer func() { result = errors.Join(result, restoreOwned(s, service)) }()
	client, err := s.HTTPClient()
	if err != nil {
		return report, err
	}
	config := pipelineCaseConfig(ctx, s, p, client, c, seed, namespace)
	var mu sync.Mutex
	traces := map[int][]FaultEvent{}
	completed := map[int]chan error{}
	failures := map[int]error{}
	record := func(rep int, start time.Time, action string, planned time.Duration, err error) {
		mu.Lock()
		defer mu.Unlock()
		entry := FaultEvent{Action: action, Service: service, Repetition: rep, ElapsedNS: time.Since(start).Nanoseconds(), PlannedNS: int64(planned), At: time.Now().UTC()}
		if err != nil {
			entry.ErrorClass = "control_failed"
		}
		traces[rep] = append(traces[rep], entry)
	}
	config.SampleStarted = func(rep int, start time.Time) {
		if rep == 0 {
			return
		}
		done := make(chan error, 1)
		mu.Lock()
		completed[rep] = done
		mu.Unlock()
		go func() {
			var err error
			defer func() { done <- err }()
			if err = waitUntil(ctx, start.Add(c.FaultAt)); err != nil {
				return
			}
			record(rep, start, "fault_requested", c.FaultAt, nil)
			if service == "consumer" {
				err = s.KillConsumer(ctx)
			} else {
				err = s.SetServiceRunning(ctx, service, false)
			}
			record(rep, start, "stopped", c.FaultAt, err)
			if err != nil {
				return
			}
			if err = waitUntil(ctx, start.Add(c.RestoreAt)); err != nil {
				return
			}
			record(rep, start, "restore_requested", c.RestoreAt, nil)
			err = s.SetServiceRunning(ctx, service, true)
			record(rep, start, "started", c.RestoreAt, err)
		}()
	}
	config.SampleFinished = func(rep int) error {
		if rep == 0 {
			return WaitConsumerCaughtUp(ctx, s)
		}
		mu.Lock()
		done := completed[rep]
		mu.Unlock()
		if done == nil {
			return nil
		}
		err := <-done
		if err == nil {
			err = WaitConsumerCaughtUp(ctx, s)
		}
		mu.Lock()
		failures[rep] = err
		mu.Unlock()
		return err
	}
	pipeline, err := RunPipeline(ctx, config)
	for i := range pipeline.Runs {
		sample := pipeline.Runs[i]
		mu.Lock()
		faults := append([]FaultEvent(nil), traces[sample.Repetition]...)
		controlErr := failures[sample.Repetition]
		mu.Unlock()
		report.Runs = append(report.Runs, RecoveryRun{Repetition: sample.Repetition, Correct: sample.Correct && controlErr == nil && len(faults) >= 4, Faults: faults, Pipeline: &sample, AcceptedIDs: sample.AcceptedIDs, StoredIDs: sample.StoredIDs})
		err = errors.Join(err, controlErr)
	}
	return report, err
}
func outboxProcess(ctx context.Context, s *runenv.Stack, args ...string) ([]byte, error) {
	root, err := runenv.Root()
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, "python3", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "API_URL="+s.APIURL, "INGESTKIT_API_KEY="+s.APIKey(), "INGESTKIT_CA_FILE="+filepath.Join(s.Directory, "root.crt"))
	return cmd.CombinedOutput()
}
func makeOutbox(ctx context.Context, s *runenv.Stack, database, prefix string, count int) ([]event.Envelope, error) {
	script := `import json,sqlite3,sys; from examples.outbox.app import create_order; db,prefix,count=sys.argv[1],sys.argv[2],int(sys.argv[3]); [create_order(db,prefix+'/'+str(i),1000+i) for i in range(count)]; connection=sqlite3.connect(db); print('['+','.join(row[0] for row in connection.execute('SELECT body FROM outbox ORDER BY rowid'))+']'); connection.close()`
	raw, err := outboxProcess(ctx, s, "-c", script, database, prefix, fmt.Sprint(count))
	if err != nil {
		return nil, errors.New("outbox setup failed")
	}
	var inputs []event.Input
	if json.Unmarshal(raw, &inputs) != nil || len(inputs) != count {
		return nil, errors.New("invalid outbox setup result")
	}
	events := make([]event.Envelope, len(inputs))
	for i, input := range inputs {
		events[i] = event.Envelope{Input: input, TenantID: "default"}
	}
	return events, nil
}
func dispatchOutbox(ctx context.Context, s *runenv.Stack, database string) (map[string]any, error) {
	raw, err := outboxProcess(ctx, s, "-m", "examples.outbox", "dispatch", "--database", database, "--max-attempts", "1")
	var summary map[string]any
	if json.Unmarshal(raw, &summary) != nil {
		return nil, errors.New("invalid outbox dispatch result")
	}
	return summary, err
}
func eventIDs(events []event.Envelope) []uuid.UUID {
	ids := make([]uuid.UUID, len(events))
	for i, e := range events {
		ids[i] = e.EventID
	}
	return ids
}
func waitStoredInputs(ctx context.Context, p *pgxpool.Pool, events []event.Envelope, deadline time.Duration) ([]uuid.UUID, error) {
	ids := eventIDs(events)
	observer, err := NewPendingObserver(ctx, p, "default", ids)
	if err != nil {
		return nil, err
	}
	closeObserver := func() error {
		end, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return observer.Close(end)
	}
	defer closeObserver()
	end := time.Now().Add(deadline)
	found := map[uuid.UUID]bool{}
	for len(found) < len(ids) {
		rows, e := observer.Observe(ctx)
		if e != nil {
			return nil, e
		}
		for _, row := range rows {
			found[row.ID] = true
		}
		if len(found) == len(ids) {
			break
		}
		if time.Now().After(end) {
			return nil, errors.New("acknowledged outbox events did not drain")
		}
		if e = waitUntil(ctx, time.Now().Add(50*time.Millisecond)); e != nil {
			return nil, e
		}
	}
	if err = closeObserver(); err != nil {
		return nil, err
	}
	var business struct {
		OrderID string `json:"order_id"`
	}
	if json.Unmarshal(events[0].Payload, &business) != nil {
		return nil, errors.New("invalid outbox verification payload")
	}
	slash := strings.LastIndex(business.OrderID, "/")
	if slash < 0 {
		return nil, errors.New("outbox verification needs a business namespace")
	}
	v, err := VerifyOutboxRows(ctx, p, "default", business.OrderID[:slash+1], events, ids)
	return v.StoredIDs, err
}
func deleteOutboxRows(p *pgxpool.Pool, events []event.Envelope) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = p.Exec(ctx, "DELETE FROM ingestkit.events WHERE tenant_id='default' AND event_id=ANY($1::uuid[])", eventIDs(events))
}
func runOutboxRecovery(ctx context.Context, s *runenv.Stack, p *pgxpool.Pool, c Case, seed int64, namespace string) (report RecoveryReport, result error) {
	report = RecoveryReport{Kind: "outbox", Notes: []string{"One untimed warm-up order before three measured broker outages.100 pending local events per repetition; the first one-attempt dispatch exceeds the broker publish timeout.", "Pending-local samples are observations after commands, not exact transition times. HTTP ACK still precedes PostgreSQL persistence."}}
	defer func() { result = errors.Join(result, restoreOwned(s, "redpanda")) }()
	warmDB := filepath.Join(s.Directory, "private", "outbox-warm.db")
	warm, e := makeOutbox(ctx, s, warmDB, namespace+"/warm", 1)
	if e != nil {
		return report, e
	}
	if _, e = dispatchOutbox(ctx, s, warmDB); e != nil {
		return report, errors.New("outbox warm-up dispatch failed")
	}
	if _, e = waitStoredInputs(ctx, p, warm, c.Drain); e != nil {
		return report, e
	}
	if e = WaitConsumerCaughtUp(ctx, s); e != nil {
		return report, e
	}
	deleteOutboxRows(p, warm)
	for rep := 1; rep <= c.Repetitions; rep++ {
		run, err := outboxRecoverySample(ctx, s, p, c, namespace, rep)
		report.Runs = append(report.Runs, run)
		if err != nil {
			return report, err
		}
	}
	return report, nil
}
func outboxRecoverySample(ctx context.Context, s *runenv.Stack, p *pgxpool.Pool, c Case, namespace string, rep int) (run RecoveryRun, result error) {
	run.Repetition = rep
	database := filepath.Join(s.Directory, "private", fmt.Sprintf("outbox-%d.db", rep))
	events, err := makeOutbox(ctx, s, database, fmt.Sprintf("%s/rep%d", namespace, rep), c.Count)
	if err != nil {
		return run, err
	}
	defer deleteOutboxRows(p, events)
	start := time.Now()
	record := func(action string, planned time.Duration, err error) {
		entry := FaultEvent{Action: action, Service: "redpanda", Repetition: rep, ElapsedNS: time.Since(start).Nanoseconds(), PlannedNS: int64(planned), At: time.Now().UTC()}
		if err != nil {
			entry.ErrorClass = "control_failed"
		}
		run.Faults = append(run.Faults, entry)
	}
	record("fault_requested", 0, nil)
	err = s.SetServiceRunning(ctx, "redpanda", false)
	record("stopped", 0, err)
	if err != nil {
		return run, err
	}
	run.Outbox = append(run.Outbox, OutboxPoint{ElapsedNS: time.Since(start).Nanoseconds(), Pending: c.Count})
	down := time.Now()
	restoreDone := make(chan error, 1)
	// Only the restore goroutine mutates its separate trace; merge after joining.
	var restoration []FaultEvent
	go func() {
		e := waitUntil(ctx, down.Add(c.RestoreAt))
		if e == nil {
			restoration = append(restoration, FaultEvent{Action: "restore_requested", Service: "redpanda", Repetition: rep, ElapsedNS: time.Since(start).Nanoseconds(), PlannedNS: down.Add(c.RestoreAt).Sub(start).Nanoseconds(), At: time.Now().UTC()})
			e = s.SetServiceRunning(ctx, "redpanda", true)
			entry := FaultEvent{Action: "started", Service: "redpanda", Repetition: rep, ElapsedNS: time.Since(start).Nanoseconds(), At: time.Now().UTC()}
			if e != nil {
				entry.ErrorClass = "control_failed"
			}
			restoration = append(restoration, entry)
		}
		restoreDone <- e
	}()
	firstStarted := time.Now()
	summary, firstErr := dispatchOutbox(ctx, s, database)
	run.FirstAttemptNS = time.Since(firstStarted).Nanoseconds()
	if code, ok := summary["error"].(string); ok && (code == "http_503" || code == "delivery_uncertain") {
		run.FirstAttemptStatus = code
		run.AmbiguousIDs = eventIDs(events)
	}
	run.Outbox = append(run.Outbox, OutboxPoint{ElapsedNS: time.Since(start).Nanoseconds(), Pending: c.Count})
	restoreErr := <-restoreDone
	run.Faults = append(run.Faults, restoration...)
	if restoreErr != nil {
		return run, restoreErr
	}
	if firstErr == nil || run.FirstAttemptStatus == "" || run.FirstAttemptNS < int64(5*time.Second) || summary["acknowledged"] != float64(0) || summary["pending"] != float64(c.Count) {
		return run, errors.New("broker fault did not retain all pending outbox events")
	}
	client, e := s.HTTPClient()
	if e != nil {
		return run, e
	}
	ready := time.Now().Add(c.Drain)
	for {
		response, e := client.Get(s.APIURL + "/ready")
		if e == nil {
			response.Body.Close()
			if response.StatusCode == 200 {
				break
			}
		}
		if time.Now().After(ready) {
			return run, errors.New("broker did not restore API readiness")
		}
		if e = waitUntil(ctx, time.Now().Add(100*time.Millisecond)); e != nil {
			return run, e
		}
	}
	summary, err = dispatchOutbox(ctx, s, database)
	if err != nil || summary["acknowledged"] != float64(c.Count) {
		return run, errors.New("restored outbox dispatch was not acknowledged")
	}
	run.AcceptedIDs = eventIDs(events)
	run.Outbox = append(run.Outbox, OutboxPoint{ElapsedNS: time.Since(start).Nanoseconds(), Acknowledged: c.Count})
	run.StoredIDs, err = waitStoredInputs(ctx, p, events, c.Drain)
	if err == nil {
		err = WaitConsumerCaughtUp(ctx, s)
	}
	run.Correct = err == nil && len(run.StoredIDs) == c.Count
	return run, err
}
