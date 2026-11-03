package telemetry

import (
	storage "github.com/feat7/ingestkit/internal/persistence"
	"github.com/stretchr/testify/require"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func scrape(t *testing.T, m *Metrics) string {
	r := httptest.NewRecorder()
	m.Handler().ServeHTTP(r, httptest.NewRequest("GET", "/metrics", nil))
	b, err := io.ReadAll(r.Result().Body)
	require.NoError(t, err)
	return string(b)
}
func TestMetricsIndependentRegistries(t *testing.T) {
	a, b := New(), New()
	a.ObserveRetry()
	require.Contains(t, scrape(t, a), "ingestkit_retries_total 1")
	require.Contains(t, scrape(t, b), "ingestkit_retries_total 0")
}
func TestMetricsCountersAndHistogramBuckets(t *testing.T) {
	m := New()
	m.ObserveHTTP("single", "202", time.Millisecond)
	m.ObservePublish("success", time.Millisecond)
	m.ObserveWrite(storage.WriteResult{Inserted: 2, Duplicates: 1}, []time.Time{time.Now().Add(-time.Second)}, time.Millisecond)
	m.ObserveQuarantine("success")
	s := scrape(t, m)
	require.Contains(t, s, "ingestkit_stored_total 2")
	require.Contains(t, s, "ingestkit_duplicates_total 1")
	require.Contains(t, s, "ingestkit_storage_seconds_count 1")
	require.Contains(t, s, "ingestkit_quarantine_total{outcome=\"success\"} 1")
}
func TestMetricsHaveBoundedLabels(t *testing.T) {
	m := New()
	m.ObserveHTTP("/v1/secret-id", "arbitrary", time.Second)
	m.ObservePublish("tenant-secret", time.Second)
	s := scrape(t, m)
	require.NotContains(t, s, "secret")
	require.NotContains(t, s, "events_per_second")
	require.Contains(t, s, "route=\"other\"")
}
func TestReceiptLatencyObservedAfterCommit(t *testing.T) {
	m := New()
	m.ObserveWrite(storage.WriteResult{Inserted: 1}, []time.Time{time.Now().Add(-time.Second), time.Now().Add(time.Hour)}, time.Millisecond)
	s := scrape(t, m)
	require.True(t, strings.Contains(s, "ingestkit_receipt_to_storage_seconds_count 2"))
}
