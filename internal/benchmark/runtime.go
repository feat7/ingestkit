package benchmark

import (
	"context"
	"github.com/feat7/ingestkit/internal/runenv"
	"github.com/feat7/ingestkit/internal/runtimeinfo"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"io"
	"net/http"
	"strings"
	"time"
)

func DriverPoolSnapshot(p *pgxpool.Pool) map[string]any {
	if p == nil {
		return map[string]any{"status": "unavailable"}
	}
	s := p.Stat()
	return map[string]any{"status": "available", "max_conns": s.MaxConns(), "acquired": s.AcquiredConns(), "idle": s.IdleConns(), "total": s.TotalConns(), "empty_acquires": s.EmptyAcquireCount(), "empty_wait_ns": s.EmptyAcquireWaitTime().Nanoseconds(), "cancelled_acquires": s.CanceledAcquireCount(), "acquire_ns": s.AcquireDuration().Nanoseconds()}
}

// Keep numeric runtime signals only. No arbitrary metric labels, tenant IDs,
// addresses or raw scrape errors enter a public artifact.
func numericRuntimeMetrics(raw string) (map[string]float64, error) {
	parser := expfmt.NewTextParser(model.UTF8Validation)
	families, err := parser.TextToMetricFamilies(strings.NewReader(raw))
	if err != nil {
		return nil, err
	}
	out := map[string]float64{}
	for name, f := range families {
		allowed := strings.HasPrefix(name, "ingestkit_db_pool_") || strings.HasPrefix(name, "ingestkit_producer_") || strings.HasPrefix(name, "ingestkit_process_") || name == "ingestkit_api_connection_limit" || name == "ingestkit_publish_failures_total" || name == "ingestkit_stored_total"
		if !allowed {
			continue
		}
		for _, m := range f.Metric {
			key := name
			valid := true
			for _, l := range m.Label {
				k, v := l.GetName(), l.GetValue()
				if k == "reason" && (v == "timeout" || v == "buffer_saturated" || v == "broker_error") {
					key += "{reason=" + v + "}"
				} else if k == "resource" && strings.HasPrefix(v, "process_") {
					key += "{resource=" + v + "}"
				} else {
					valid = false
				}
			}
			if !valid {
				continue
			}
			if m.Untyped != nil {
				out[key] = m.Untyped.GetValue()
			} else if m.Gauge != nil {
				out[key] = m.Gauge.GetValue()
			} else if m.Counter != nil {
				out[key] = m.Counter.GetValue()
			}
		}
	}
	return out, nil
}
func runtimeScrape(ctx context.Context, address string) map[string]any {
	if address == "" {
		return map[string]any{"status": "unavailable"}
	}
	req, err := http.NewRequestWithContext(ctx, "GET", address+"/metrics", nil)
	if err != nil {
		return map[string]any{"status": "unavailable"}
	}
	client := &http.Client{Timeout: 2 * time.Second}
	defer client.CloseIdleConnections()
	res, err := client.Do(req)
	if err != nil {
		return map[string]any{"status": "unavailable"}
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil || res.StatusCode != 200 {
		return map[string]any{"status": "unavailable"}
	}
	metrics, err := numericRuntimeMetrics(string(raw))
	if err != nil {
		return map[string]any{"status": "unavailable"}
	}
	// JSON cannot represent infinity; availability stays explicit in numeric metrics.
	values := map[string]any{}
	for k, v := range metrics {
		if v > 1.7976931348623157e308 {
			values[k] = "unlimited"
		} else {
			values[k] = v
		}
	}
	return map[string]any{"status": "available", "metrics": values}
}
func RuntimeSnapshot(ctx context.Context, s *runenv.Stack, p *pgxpool.Pool) map[string]any {
	out := map[string]any{"at": time.Now().UTC(), "driver_pool": DriverPoolSnapshot(p), "generator_limits": runtimeinfo.CaptureProcessLimits(), "api": runtimeScrape(ctx, s.APIProbeURL), "consumer": runtimeScrape(ctx, s.ConsumerURL)}
	if p != nil {
		rows, err := p.Query(ctx, "SELECT usename, CASE WHEN application_name='ingestkit-benchmark' THEN application_name ELSE 'other' END, state, count(*) FROM pg_stat_activity WHERE datname=current_database() GROUP BY 1,2,3 ORDER BY 1,2,3")
		if err != nil {
			out["database_sessions"] = map[string]any{"status": "unavailable"}
		} else {
			out["database_sessions"] = sessionObservation(rows)
		}
	}
	return out
}

func sessionObservation(rows pgx.Rows) map[string]any {
	defer rows.Close()
	groups := []map[string]any{}
	for rows.Next() {
		var role, app string
		var state *string
		var count int64
		if rows.Scan(&role, &app, &state, &count) != nil {
			return map[string]any{"status": "unavailable"}
		}
		groups = append(groups, map[string]any{"role": role, "application": app, "state": state, "count": count})
	}
	if rows.Err() != nil {
		return map[string]any{"status": "unavailable"}
	}
	return map[string]any{"status": "available", "groups": groups}
}
