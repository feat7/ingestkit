package benchmark

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestCalibrationIsTLSDriverEvidenceWithExactACKs(t *testing.T) {
	r, err := RunCalibration(context.Background(), PipelineConfig{Rate: 20, Duration: 500 * time.Millisecond, Concurrency: 4, BatchSize: 2, PayloadBytes: 1024, Seed: 42, Repetitions: 3, HTTPMaxConnsPerHost: 2, HTTPMaxIdleConnsPerHost: 4})
	require.NoError(t, err)
	require.Equal(t, "tls_driver_calibration", r.Kind)
	require.Equal(t, true, r.Config["https"])
	require.Len(t, r.Runs, 3)
	for _, run := range r.Runs {
		require.Equal(t, 10, run.Planned)
		require.Equal(t, run.Planned, run.Accepted+run.GeneratorMissed)
		require.Equal(t, run.Attempted, run.Accepted)
		require.Positive(t, run.Connections.Protocols["HTTP/1.1"])
		require.LessOrEqual(t, run.Connections.PeakOpenTCP, int64(2))
		require.NotEmpty(t, run.IDSetSHA256)
		require.True(t, run.Correct)
	}
	raw, err := json.Marshal(r)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "Persisted")
	require.NotContains(t, string(raw), "https://127.0.0.1")
}
