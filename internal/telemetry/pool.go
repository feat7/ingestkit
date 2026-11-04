package telemetry

import (
	"math"
	"sync"

	"github.com/feat7/ingestkit/internal/runtimeinfo"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

type ProducerBufferSource interface {
	BufferedProduceRecords() int64
	BufferedProduceBytes() int64
}
type runtimeMetrics struct {
	mu                             sync.RWMutex
	pool                           *pgxpool.Pool
	producer                       ProducerBufferSource
	maxRecords, maxBytes, apiLimit int
	desc                           map[string]*prometheus.Desc
	available                      *prometheus.Desc
	capture                        func() runtimeinfo.ProcessLimits
}

func newRuntimeMetrics() *runtimeMetrics {
	r := &runtimeMetrics{desc: map[string]*prometheus.Desc{}, capture: runtimeinfo.CaptureProcessLimits}
	for _, name := range []string{"db_pool_available", "db_pool_max_conns", "db_pool_acquired_conns", "db_pool_idle_conns", "db_pool_total_conns", "db_pool_empty_acquires_total", "db_pool_empty_acquire_wait_seconds_total", "db_pool_cancelled_acquires_total", "db_pool_acquire_seconds_total", "producer_buffer_available", "producer_max_buffered_records", "producer_max_buffered_bytes", "producer_buffered_records", "producer_buffered_bytes", "api_connection_limit", "process_fd_soft_limit", "process_fd_hard_limit", "process_open_fds", "process_ephemeral_port_first", "process_ephemeral_port_last"} {
		r.desc[name] = prometheus.NewDesc("ingestkit_"+name, "Private runtime measurement: "+name, nil, nil)
	}
	r.available = prometheus.NewDesc("ingestkit_process_limit_available", "Whether a numeric process measurement is available.", []string{"resource"}, nil)
	return r
}
func (r *runtimeMetrics) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range r.desc {
		ch <- d
	}
	ch <- r.available
}
func (r *runtimeMetrics) Collect(ch chan<- prometheus.Metric) {
	r.mu.RLock()
	pool, producer, maxRecords, maxBytes, apiLimit := r.pool, r.producer, r.maxRecords, r.maxBytes, r.apiLimit
	r.mu.RUnlock()
	gauge := func(name string, value float64) {
		ch <- prometheus.MustNewConstMetric(r.desc[name], prometheus.GaugeValue, value)
	}
	counter := func(name string, value float64) {
		ch <- prometheus.MustNewConstMetric(r.desc[name], prometheus.CounterValue, value)
	}
	if pool == nil {
		gauge("db_pool_available", 0)
	} else {
		gauge("db_pool_available", 1)
		// Emit one consistent Stat snapshot; never put the connection configuration in labels.
		s := pool.Stat()
		gauge("db_pool_max_conns", float64(s.MaxConns()))
		gauge("db_pool_acquired_conns", float64(s.AcquiredConns()))
		gauge("db_pool_idle_conns", float64(s.IdleConns()))
		gauge("db_pool_total_conns", float64(s.TotalConns()))
		counter("db_pool_empty_acquires_total", float64(s.EmptyAcquireCount()))
		counter("db_pool_empty_acquire_wait_seconds_total", s.EmptyAcquireWaitTime().Seconds())
		counter("db_pool_cancelled_acquires_total", float64(s.CanceledAcquireCount()))
		counter("db_pool_acquire_seconds_total", s.AcquireDuration().Seconds())
	}
	if producer == nil {
		gauge("producer_buffer_available", 0)
	} else {
		gauge("producer_buffer_available", 1)
		gauge("producer_max_buffered_records", float64(maxRecords))
		gauge("producer_max_buffered_bytes", float64(maxBytes))
		gauge("producer_buffered_records", float64(producer.BufferedProduceRecords()))
		gauge("producer_buffered_bytes", float64(producer.BufferedProduceBytes()))
	}
	if apiLimit > 0 {
		gauge("api_connection_limit", float64(apiLimit))
	}
	limits := r.capture()
	for name, value := range map[string]runtimeinfo.Limit{"process_fd_soft_limit": limits.SoftFDLimit, "process_fd_hard_limit": limits.HardFDLimit, "process_open_fds": limits.OpenFDs, "process_ephemeral_port_first": limits.EphemeralFirst, "process_ephemeral_port_last": limits.EphemeralLast} {
		if value.Status == "known" && value.Value != nil {
			ch <- prometheus.MustNewConstMetric(r.available, prometheus.GaugeValue, 1, name)
			gauge(name, float64(*value.Value))
		} else if value.Status == "unlimited" {
			ch <- prometheus.MustNewConstMetric(r.available, prometheus.GaugeValue, 1, name)
			gauge(name, math.Inf(1))
		} else {
			ch <- prometheus.MustNewConstMetric(r.available, prometheus.GaugeValue, 0, name)
		}
	}
}
func (m *Metrics) RegisterPool(pool *pgxpool.Pool) {
	m.runtime.mu.Lock()
	m.runtime.pool = pool
	m.runtime.mu.Unlock()
}
func (m *Metrics) RegisterProducer(source ProducerBufferSource, maxRecords, maxBytes int) {
	m.runtime.mu.Lock()
	m.runtime.producer = source
	m.runtime.maxRecords = maxRecords
	m.runtime.maxBytes = maxBytes
	m.runtime.mu.Unlock()
}
func (m *Metrics) SetAPIConnectionLimit(limit int) {
	m.runtime.mu.Lock()
	m.runtime.apiLimit = limit
	m.runtime.mu.Unlock()
}
func (m *Metrics) ObservePublishFailure(reason string) {
	switch reason {
	case "buffer_saturated", "timeout", "broker_error":
	default:
		reason = "broker_error"
	}
	m.publishFailures.WithLabelValues(reason).Inc()
}
