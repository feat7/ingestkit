package benchmark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/feat7/ingestkit/internal/event"
	"github.com/feat7/ingestkit/internal/persistence"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"reflect"
	"strings"
	"time"
)

var methods = []string{"insert", "batch-insert", "copy-merge"}

type DatabaseConfig struct {
	Pool                   *pgxpool.Pool
	Workload               WorkloadConfig
	BatchSize, Repetitions int
}
type DatabaseRun struct {
	Method     string  `json:"method"`
	Repetition int     `json:"repetition"`
	Submitted  int     `json:"submitted"`
	Inserted   int64   `json:"inserted"`
	Duplicates int64   `json:"duplicates"`
	ElapsedNS  int64   `json:"elapsed_ns"`
	BatchNS    []int64 `json:"batch_ns"`
	Correct    bool    `json:"correct"`
}
type DatabaseReport struct {
	Kind        string         `json:"kind"`
	Workload    WorkloadConfig `json:"workload"`
	BatchSize   int            `json:"batch_size"`
	Repetitions int            `json:"repetitions"`
	Runs        []DatabaseRun  `json:"runs"`
	Notes       []string       `json:"notes"`
}

func measure(setup func() error, work func() (int64, error), now func() time.Time) (time.Duration, int64, error) {
	if e := setup(); e != nil {
		return 0, 0, e
	}
	start := now()
	n, e := work()
	return now().Sub(start), n, e
}

const insertPrefix = "INSERT INTO ingestkit.events(tenant_id,event_id,event_type,occurred_at,received_at,payload) VALUES "

func writeMethod(ctx context.Context, p *pgxpool.Pool, method string, es []event.Envelope) (int64, error) {
	if method == "copy-merge" {
		r, e := persistence.NewEventStore(p).WriteBatch(ctx, es)
		return r.Inserted, e
	}
	if method == "insert" {
		var total int64
		for _, v := range es {
			n, e := writeMethod(ctx, p, "batch-insert", []event.Envelope{v})
			if e != nil {
				return total, e
			}
			total += n
		}
		return total, nil
	}
	if method != "batch-insert" {
		return 0, errors.New("unknown database method")
	}
	args := make([]any, 0, len(es)*6)
	values := make([]string, len(es))
	for i, e := range es {
		n := i * 6
		values[i] = fmt.Sprintf("($%d,$%d,$%d,$%d,$%d,$%d)", n+1, n+2, n+3, n+4, n+5, n+6)
		args = append(args, e.TenantID, e.EventID.String(), e.EventType, e.OccurredAt, e.ReceivedAt, []byte(e.Payload))
	}
	tag, e := p.Exec(ctx, insertPrefix+strings.Join(values, ",")+" ON CONFLICT (tenant_id,event_id) DO NOTHING", args...)
	return tag.RowsAffected(), e
}
func verifyDatabase(ctx context.Context, p *pgxpool.Pool, tenant string, es []event.Envelope) error {
	want := expectedEvents(es)
	rows, e := p.Query(ctx, "SELECT event_id,event_type,occurred_at,received_at,payload FROM ingestkit.events WHERE tenant_id=$1", tenant)
	if e != nil {
		return e
	}
	defer rows.Close()
	seen := map[uuid.UUID]bool{}
	for rows.Next() {
		var id uuid.UUID
		var typ string
		var occurred, received time.Time
		var payload []byte
		if e = rows.Scan(&id, &typ, &occurred, &received, &payload); e != nil {
			return e
		}
		w, ok := want[id]
		var a, b any
		if !ok || seen[id] {
			return errors.New("unexpected or repeated stored ID")
		}
		seen[id] = true
		if e = json.Unmarshal(w.Payload, &a); e != nil {
			return e
		}
		if e = json.Unmarshal(payload, &b); e != nil {
			return e
		}
		if typ != w.EventType || !occurred.Equal(w.OccurredAt) || !received.Equal(w.ReceivedAt) || !reflect.DeepEqual(a, b) {
			return errors.New("stored event differs from generated event")
		}
	}
	if e = rows.Err(); e != nil {
		return e
	}
	if len(seen) != len(want) {
		return errors.New("missing stored IDs")
	}
	return nil
}
func RunDatabase(ctx context.Context, c DatabaseConfig) (DatabaseReport, error) {
	r := DatabaseReport{Kind: "database", Workload: c.Workload, BatchSize: c.BatchSize, Repetitions: c.Repetitions, Notes: []string{"Generation, dataset reset, warm-up and exact ID/payload verification excluded from elapsed time.", "Single INSERT commits each row; batch INSERT and production COPY/merge commit each batch. Same table, indexes and conflict policy.", "BatchNS records client-observed calls; single INSERT calls contain one row. Durability settings are captured in manifest.json."}}
	if c.Pool == nil || c.BatchSize <= 0 || c.BatchSize > 1000 || c.Repetitions < 3 {
		return r, errors.New("pool, batch size 1..1000 and at least three repetitions required")
	}
	es, e := Workload(c.Workload)
	if e != nil {
		return r, e
	}
	tenant := "bench-" + uuid.NewString()
	for i := range es {
		es[i].TenantID = tenant
	}
	reset := func() error {
		_, e := c.Pool.Exec(ctx, "DELETE FROM ingestkit.events WHERE tenant_id=$1", tenant)
		return e
	}
	defer func() {
		end, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = c.Pool.Exec(end, "DELETE FROM ingestkit.events WHERE tenant_id=$1", tenant)
	}()
	// Warm every code path with a complete batch before measured repetitions.
	for _, m := range methods {
		if e = reset(); e != nil {
			return r, e
		}
		if _, e = writeMethod(ctx, c.Pool, m, es[:min(c.BatchSize, len(es))]); e != nil {
			return r, e
		}
	}
	for rep := 0; rep < c.Repetitions; rep++ {
		for order := range methods {
			m := methods[(rep+order)%len(methods)]
			run := DatabaseRun{Method: m, Repetition: rep + 1, Submitted: len(es)}
			elapsed, n, e := measure(reset, func() (int64, error) {
				var n int64
				size := c.BatchSize
				if m == "insert" {
					size = 1
				}
				for start := 0; start < len(es); start += size {
					at := time.Now()
					w, e := writeMethod(ctx, c.Pool, m, es[start:min(start+size, len(es))])
					run.BatchNS = append(run.BatchNS, time.Since(at).Nanoseconds())
					if e != nil {
						return n, e
					}
					n += w
				}
				return n, nil
			}, time.Now)
			if e != nil {
				return r, fmt.Errorf("%s repetition %d: %w", m, rep+1, e)
			}
			if e = verifyDatabase(ctx, c.Pool, tenant, es); e != nil {
				return r, e
			}
			run.ElapsedNS = elapsed.Nanoseconds()
			run.Inserted = n
			run.Duplicates = int64(len(es)) - n
			run.Correct = n == int64(len(expectedEvents(es)))
			r.Runs = append(r.Runs, run)
		}
	}
	return r, r.Validate()
}
