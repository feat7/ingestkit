// Package telemetry exposes low-cardinality ingestion measurements.
package telemetry

import (
	storage "github.com/feat7/ingestkit/internal/persistence"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"time"
)

type Metrics struct {
	runtime                     *runtimeMetrics
	publishFailures             *prometheus.CounterVec
	registry                    *prometheus.Registry
	http                        *prometheus.CounterVec
	httpTime                    *prometheus.HistogramVec
	publish                     *prometheus.HistogramVec
	stored, duplicates, retries prometheus.Counter
	writes, receipts, batches   prometheus.Histogram
	quarantine                  *prometheus.CounterVec
}

func New() *Metrics {
	m := &Metrics{registry: prometheus.NewRegistry(), runtime: newRuntimeMetrics(), publishFailures: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "ingestkit_publish_failures_total", Help: "Publish failure class; timeout includes deadline or cancellation."}, []string{"reason"}),
		http:       prometheus.NewCounterVec(prometheus.CounterOpts{Name: "ingestkit_http_total", Help: "HTTP request outcomes."}, []string{"route", "status"}),
		httpTime:   prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "ingestkit_http_seconds", Help: "HTTP handler latency.", Buckets: prometheus.DefBuckets}, []string{"route"}),
		publish:    prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "ingestkit_publish_seconds", Help: "Broker publish call latency and outcomes.", Buckets: prometheus.DefBuckets}, []string{"outcome"}),
		stored:     prometheus.NewCounter(prometheus.CounterOpts{Name: "ingestkit_stored_total", Help: "New event rows committed."}),
		duplicates: prometheus.NewCounter(prometheus.CounterOpts{Name: "ingestkit_duplicates_total", Help: "Duplicate events ignored after successful commit."}),
		retries:    prometheus.NewCounter(prometheus.CounterOpts{Name: "ingestkit_retries_total", Help: "Storage retries."}),
		writes:     prometheus.NewHistogram(prometheus.HistogramOpts{Name: "ingestkit_storage_seconds", Help: "Successful database transaction latency.", Buckets: prometheus.DefBuckets}),
		receipts:   prometheus.NewHistogram(prometheus.HistogramOpts{Name: "ingestkit_receipt_to_storage_seconds", Help: "Server receipt to observed successful transaction completion; assumes synchronized clocks.", Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2, 5, 10, 30, 60}}),
		batches:    prometheus.NewHistogram(prometheus.HistogramOpts{Name: "ingestkit_batch_events", Help: "Events per committed write batch.", Buckets: []float64{1, 10, 50, 100, 250, 500}}),
		quarantine: prometheus.NewCounterVec(prometheus.CounterOpts{Name: "ingestkit_quarantine_total", Help: "Broker quarantine outcomes."}, []string{"outcome"}),
	}
	m.registry.MustRegister(m.runtime, m.publishFailures, m.http, m.httpTime, m.publish, m.stored, m.duplicates, m.retries, m.writes, m.receipts, m.batches, m.quarantine)
	return m
}
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
func route(s string) string {
	switch s {
	case "single", "batch", "health", "ready", "metrics":
		return s
	}
	return "other"
}
func outcome(s string) string {
	switch s {
	case "success", "error", "saturated", "timeout":
		return s
	}
	return "error"
}
func (m *Metrics) ObserveHTTP(r, s string, d time.Duration) {
	switch s {
	case "200", "202", "400", "401", "404", "413", "429", "500", "503":
	default:
		s = "other"
	}
	m.http.WithLabelValues(route(r), s).Inc()
	m.httpTime.WithLabelValues(route(r)).Observe(d.Seconds())
}
func (m *Metrics) ObservePublish(o string, d time.Duration) {
	m.publish.WithLabelValues(outcome(o)).Observe(d.Seconds())
}
func (m *Metrics) ObserveWrite(r storage.WriteResult, received []time.Time, d time.Duration) {
	m.stored.Add(float64(r.Inserted))
	m.duplicates.Add(float64(r.Duplicates))
	m.writes.Observe(d.Seconds())
	m.batches.Observe(float64(r.Inserted + r.Duplicates))
	now := time.Now()
	for _, at := range received {
		elapsed := now.Sub(at).Seconds()
		if elapsed < 0 {
			elapsed = 0
		}
		m.receipts.Observe(elapsed)
	}
}
func (m *Metrics) ObserveRetry()              { m.retries.Inc() }
func (m *Metrics) ObserveQuarantine(o string) { m.quarantine.WithLabelValues(outcome(o)).Inc() }
