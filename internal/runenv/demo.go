package runenv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/feat7/ingestkit/internal/event"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type DemoOptions struct {
	Recovery, Cleanup bool
	OutputDirectory   string
	Writer            io.Writer
	OnStage           func(string, *Stack)
}
type DemoReport struct {
	Accepted, Stored    []uuid.UUID
	Funnel              map[string]int
	Recovery, Completed bool
	Error               string
}

func reconcileIDs(expected, actual []uuid.UUID) error {
	want := map[uuid.UUID]bool{}
	for _, id := range expected {
		want[id] = true
	}
	seen := map[uuid.UUID]bool{}
	for _, id := range actual {
		if seen[id] || !want[id] {
			return errors.New("unexpected or duplicate stored ID")
		}
		seen[id] = true
	}
	if len(seen) != len(want) {
		return errors.New("accepted IDs are missing from storage")
	}
	return nil
}
func SendInputs(ctx context.Context, url, key string, inputs []event.Input) ([]uuid.UUID, error) {
	raw, err := json.Marshal(struct {
		Events []event.Input `json:"events"`
	}{inputs})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url+"/v1/events/batch", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	res, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 202 {
		return nil, fmt.Errorf("HTTP %d: retry uncertain delivery with the same event IDs", res.StatusCode)
	}
	var ack struct {
		IDs []uuid.UUID `json:"event_ids"`
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 65536)).Decode(&ack); err != nil {
		return nil, err
	}
	if len(ack.IDs) != len(inputs) {
		return nil, errors.New("acknowledgement ID count mismatch")
	}
	for i, id := range ack.IDs {
		if id != inputs[i].EventID {
			return nil, errors.New("acknowledgement ID mismatch")
		}
	}
	return ack.IDs, nil
}
func RunDemo(ctx context.Context, o DemoOptions) (result error) {
	w := o.Writer
	if w == nil {
		w = os.Stdout
	}
	purpose := "demo"
	if o.Recovery {
		purpose = "recovery"
	}
	s, err := Start(ctx, Options{Purpose: purpose, OutputDirectory: o.OutputDirectory})
	if err != nil {
		return err
	}
	report := DemoReport{Recovery: o.Recovery}
	dbStopped := false
	var pool *pgxpool.Pool
	defer func() {
		cleanup, end := context.WithTimeout(context.Background(), 30*time.Second)
		defer end()
		if dbStopped {
			if err := s.SetServiceRunning(cleanup, "postgres", true); err != nil {
				result = errors.Join(result, err)
			}
		}
		if pool != nil {
			pool.Close()
		}
		if err := s.CollectEvidence(cleanup, s.Directory); err != nil {
			result = errors.Join(result, err)
		}
		if o.Cleanup {
			err = s.Destroy(cleanup)
		} else {
			err = s.Stop(cleanup)
		}
		if err != nil {
			result = errors.Join(result, err)
		}
		report.Completed = result == nil
		if result != nil {
			report.Error = result.Error()
		}
		raw, _ := json.MarshalIndent(report, "", "  ")
		if err := os.WriteFile(filepath.Join(s.Directory, "demo.json"), raw, 0600); err != nil {
			result = errors.Join(result, err)
		}
		fmt.Fprintln(w, "Evidence:", s.Directory)
		if !o.Cleanup {
			fmt.Fprintf(w, "Stopped; volumes preserved. Cleanup only this run: docker compose -p %s -f %s --env-file %s down -v\n", s.Project, filepath.Join(s.root, "docker-compose.yml"), filepath.Join(s.Directory, "run.env"))
		}
	}()
	fmt.Fprintln(w, "Stack:", s.Project)
	pool, err = pgxpool.New(ctx, s.DatabaseURL)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(s.root, "examples", "checkout", "events.json"))
	if err != nil {
		return err
	}
	inputs, err := event.DecodeBatch(raw)
	if err != nil {
		return err
	}
	if o.OnStage != nil {
		o.OnStage("started", s)
	}
	if o.Recovery {
		if err = s.SetServiceRunning(ctx, "postgres", false); err != nil {
			return err
		}
		dbStopped = true
		fmt.Fprintln(w, "database unavailable; broker acknowledgement remains independent")
		if o.OnStage != nil {
			o.OnStage("database_unavailable", s)
		}
	}
	ids, err := SendInputs(ctx, s.APIURL, "local-demo-key", inputs)
	if err != nil {
		return err
	}
	report.Accepted = ids
	for i, id := range ids {
		fmt.Fprintf(w, "Accepted %s %s (broker acknowledged)\n", inputs[i].EventType, id)
	}
	if o.Recovery {
		lag, lagErr := s.Compose(ctx, "exec", "-T", "redpanda", "rpk", "group", "describe", "ingestkit-consumer", "-X", "brokers=redpanda:9092")
		if lagErr == nil {
			fmt.Fprintln(w, lag)
		} else {
			fmt.Fprintln(w, "Consumer group is still assigning; accepted IDs remain buffered.")
		}
		if err = s.SetServiceRunning(ctx, "postgres", true); err != nil {
			return err
		}
		dbStopped = false
	}
	wait, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		rows, e := pool.Query(wait, "SELECT event_id FROM ingestkit.events WHERE tenant_id='demo'")
		actual := []uuid.UUID{}
		if e == nil {
			for rows.Next() {
				var id uuid.UUID
				if e = rows.Scan(&id); e != nil {
					break
				}
				actual = append(actual, id)
			}
			if e == nil {
				e = rows.Err()
			}
			rows.Close()
		}
		if e == nil && reconcileIDs(ids, actual) == nil {
			report.Stored = actual
			break
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-wait.Done():
			timer.Stop()
			return fmt.Errorf("reconciliation failed: %w", wait.Err())
		case <-timer.C:
		}
	}
	rows, err := pool.Query(ctx, "SELECT event_type,count(*) FROM ingestkit.events WHERE tenant_id='demo' GROUP BY event_type ORDER BY event_type")
	if err != nil {
		return err
	}
	defer rows.Close()
	report.Funnel = map[string]int{}
	for rows.Next() {
		var name string
		var n int
		if err = rows.Scan(&name, &n); err != nil {
			return err
		}
		report.Funnel[name] = n
		fmt.Fprintf(w, "Stored %s: %d\n", name, n)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	fmt.Fprintf(w, "Reconciled %d accepted IDs: every ID stored once.\n", len(ids))
	return nil
}
