package benchmark

import (
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRuntimeMetricsAllowlistRemovesIdentities(t *testing.T) {
	raw := "ingestkit_producer_buffered_records 7\ningestkit_publish_failures_total{reason=\"timeout\"} 2\nprivate_secret{tenant=\"canary\"} 99\ningestkit_stored_total{tenant=\"canary\"} 4\n"
	got, err := numericRuntimeMetrics(raw)
	require.NoError(t, err)
	require.Equal(t, float64(7), got["ingestkit_producer_buffered_records"])
	require.Equal(t, float64(2), got["ingestkit_publish_failures_total{reason=timeout}"])
	require.Len(t, got, 2)
}

type sessionRowsFixture struct {
	pgx.Rows
	scanned            bool
	scanErr, streamErr error
}

func (r *sessionRowsFixture) Next() bool {
	if r.scanned {
		return false
	}
	r.scanned = true
	return true
}
func (r *sessionRowsFixture) Scan(values ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	*values[0].(*string) = "ingestkit_writer"
	*values[1].(*string) = "other"
	*values[2].(**string) = nil
	*values[3].(*int64) = 1
	return nil
}
func (r *sessionRowsFixture) Err() error { return r.streamErr }
func (r *sessionRowsFixture) Close()     {}
func TestFailedSessionRowsAreUnavailable(t *testing.T) {
	for _, rows := range []*sessionRowsFixture{{scanErr: errors.New("private-scan-canary")}, {streamErr: errors.New("private-stream-canary")}} {
		value := sessionObservation(rows)
		require.Equal(t, "unavailable", value["status"])
		require.Nil(t, value["groups"])
		raw, err := json.Marshal(value)
		require.NoError(t, err)
		require.NotContains(t, string(raw), "canary")
	}
	good := sessionObservation(&sessionRowsFixture{})
	require.Equal(t, "available", good["status"])
	require.Len(t, good["groups"], 1)
}
