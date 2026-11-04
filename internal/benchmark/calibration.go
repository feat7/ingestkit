package benchmark

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/feat7/ingestkit/internal/event"
	"github.com/feat7/ingestkit/internal/runtimeinfo"
	"github.com/google/uuid"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
)

type CalibrationReport struct {
	Schema               int
	Kind                 string
	SourceSHA, GoVersion string
	Config               map[string]any
	Limits               runtimeinfo.ProcessLimits
	Runs                 []CalibrationRun
	Notes                []string
}
type CalibrationRun struct {
	Planned, Attempted, Accepted, AcceptedInWindow, GeneratorMissed int
	WindowNS, TotalNS                                               int64
	AcceptedEPS                                                     float64
	Connections                                                     ConnectionSnapshot
	RequestSamples                                                  []RequestObservation
	IDSetSHA256                                                     map[string]string
	Correct                                                         bool
}

// This sink verifies only driver scheduling, TLS and acknowledgement accounting.
// It has no database, broker or ingestion durability contract.
func RunCalibration(ctx context.Context, c PipelineConfig) (CalibrationReport, error) {
	r := CalibrationReport{Schema: 2, Kind: "tls_driver_calibration", SourceSHA: localOutput(ctx, "git", "rev-parse", "HEAD"), GoVersion: runtime.Version(), Config: PublicPipelineConfig(c), Limits: runtimeinfo.CaptureProcessLimits(), Notes: []string{"Owned local TLS acknowledgement sink only; no broker, database or ingestion capacity measurement.", "Shares the physical host with the driver. Payload generation precedes each measured window; encoding remains in request time."}}
	r.Config["generator_logical_cpus"] = runtime.NumCPU()
	r.Config["generator_gomaxprocs"] = runtime.GOMAXPROCS(0)
	if err := ValidatePipelineWorkload(c); err != nil {
		return r, err
	}
	if c.Concurrency < 1 || c.Concurrency > 1000 || c.Repetitions < 1 || c.HTTPMaxConnsPerHost < 0 || c.HTTPMaxIdleConnsPerHost < 0 {
		return r, errors.New("invalid calibration controls")
	}
	var mu sync.Mutex
	received := map[uuid.UUID]bool{}
	sink := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Events []event.Input `json:"events"`
		}
		if json.NewDecoder(io.LimitReader(req.Body, event.MaxRequestBytes)).Decode(&body) != nil {
			w.WriteHeader(400)
			return
		}
		ids := make([]uuid.UUID, len(body.Events))
		mu.Lock()
		for i, e := range body.Events {
			ids[i] = e.EventID
			received[e.EventID] = true
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(202)
		_ = json.NewEncoder(w).Encode(map[string]any{"event_ids": ids})
	}))
	defer sink.Close()
	recorder := NewConnectionRecorder()
	transport := sink.Client().Transport.(*http.Transport).Clone()
	transport.MaxConnsPerHost = c.HTTPMaxConnsPerHost
	transport.MaxIdleConnsPerHost = c.HTTPMaxIdleConnsPerHost
	client := &http.Client{Transport: recorder.WrapTransport(transport), Timeout: 8 * time.Second}
	defer client.CloseIdleConnections()
	for rep := 1; rep <= c.Repetitions; rep++ {
		es, err := Workload(WorkloadConfig{Namespace: "calibration", PayloadProfile: "varied", Seed: c.Seed + int64(rep), Count: int(float64(c.Rate) * c.Duration.Seconds()), PayloadBytes: c.PayloadBytes})
		if err != nil {
			return r, err
		}
		mu.Lock()
		received = map[uuid.UUID]bool{}
		mu.Unlock()
		recorder.BeginWindow()
		jobs := make(chan []event.Envelope, c.Concurrency*2)
		results := make(chan struct {
			ids    []uuid.UUID
			sample RequestObservation
		}, (len(es)+c.BatchSize-1)/c.BatchSize)
		var wg sync.WaitGroup
		started := time.Now()
		for i := 0; i < c.Concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for job := range jobs {
					at := time.Now()
					inputs := make([]event.Input, len(job))
					ids := make([]uuid.UUID, len(job))
					for i, e := range job {
						inputs[i] = e.Input
						ids[i] = e.EventID
					}
					raw, _ := json.Marshal(map[string]any{"events": inputs})
					req, e := http.NewRequestWithContext(ctx, "POST", sink.URL, bytes.NewReader(raw))
					if req != nil {
						req.Header.Set("Content-Type", "application/json")
					}
					sample := RequestObservation{Events: len(job), StartedNS: at.Sub(started).Nanoseconds()}
					if e == nil {
						req, finish := recorder.TraceRequest(req)
						res, callErr := client.Do(req)
						if callErr == nil {
							sample.Status = res.StatusCode
							var ack struct {
								IDs []uuid.UUID `json:"event_ids"`
							}
							decodeErr := json.NewDecoder(res.Body).Decode(&ack)
							res.Body.Close()
							sample.Accepted = res.StatusCode == 202 && decodeErr == nil && slices.Equal(ack.IDs, ids)
						}
						finish(res, callErr)
					}
					sample.FinishedNS = time.Since(started).Nanoseconds()
					sample.ElapsedNS = sample.FinishedNS - sample.StartedNS
					results <- struct {
						ids    []uuid.UUID
						sample RequestObservation
					}{ids, sample}
				}
			}()
		}
		missed := 0
		for i := 0; i < len(es); i += c.BatchSize {
			due := started.Add(time.Duration(float64(i) / float64(c.Rate) * float64(time.Second)))
			timer := time.NewTimer(max(0, time.Until(due)))
			select {
			case <-ctx.Done():
				timer.Stop()
				missed += len(es) - i
				i = len(es)
				continue
			case <-timer.C:
			}
			if time.Since(started) >= c.Duration {
				missed += len(es) - i
				break
			}
			job := es[i:min(i+c.BatchSize, len(es))]
			select {
			case jobs <- job:
			default:
				missed += len(job)
			}
		}
		close(jobs)
		wg.Wait()
		close(results)
		timer := time.NewTimer(max(0, time.Until(started.Add(c.Duration))))
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
		run := CalibrationRun{Planned: len(es), GeneratorMissed: missed, WindowNS: c.Duration.Nanoseconds(), TotalNS: time.Since(started).Nanoseconds(), Connections: recorder.Snapshot()}
		attempted, accepted := []uuid.UUID{}, []uuid.UUID{}
		for result := range results {
			run.RequestSamples = append(run.RequestSamples, result.sample)
			run.Attempted += len(result.ids)
			attempted = append(attempted, result.ids...)
			if result.sample.Accepted {
				run.Accepted += len(result.ids)
				accepted = append(accepted, result.ids...)
				if result.sample.FinishedNS <= run.WindowNS {
					run.AcceptedInWindow += len(result.ids)
				}
			}
		}
		mu.Lock()
		sinkIDs := make([]uuid.UUID, 0, len(received))
		for id := range received {
			sinkIDs = append(sinkIDs, id)
		}
		mu.Unlock()
		run.AcceptedEPS = float64(run.AcceptedInWindow) / c.Duration.Seconds()
		run.IDSetSHA256 = map[string]string{"attempted": IDSetHash(attempted), "accepted": IDSetHash(accepted), "sink_received": IDSetHash(sinkIDs)}
		run.Correct = run.Planned == run.Attempted+run.GeneratorMissed && run.Accepted == run.Attempted && IDSetHash(accepted) == IDSetHash(sinkIDs)
		r.Runs = append(r.Runs, run)
		if ctx.Err() != nil {
			return r, ctx.Err()
		}
	}
	return r, nil
}

func IDSetHash(ids []uuid.UUID) string {
	values := make([]string, 0, len(ids))
	for id := range uniqueIDs(ids) {
		values = append(values, id.String())
	}
	slices.Sort(values)
	sum := sha256.Sum256([]byte(strings.Join(values, "\n")))
	return hex.EncodeToString(sum[:])
}
