package benchmark

import (
	"encoding/json"
	"github.com/feat7/ingestkit/internal/event"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestConnectionCasesAreFixedAndRequestSafe(t *testing.T) {
	cases := ConnectionCases()
	require.Len(t, cases, 12)
	seen := map[string]bool{}
	for i, c := range cases {
		require.False(t, seen[c.ID])
		seen[c.ID] = true
		require.Equal(t, 128, c.Concurrency)
		require.Equal(t, 128, c.HTTPMaxIdleConnsPerHost)
		require.Equal(t, 3, c.Repetitions)
		require.Equal(t, 10*time.Second, c.Window)
		if i < 7 {
			require.Equal(t, []int{4, 8, 16, 32, 64, 128, 0}[i], c.HTTPMaxConnsPerHost)
			require.Equal(t, 15000, c.Rate)
			require.Equal(t, 1024, c.PayloadBytes)
			require.Equal(t, 100, c.BatchSize)
		} else {
			require.Equal(t, 128, c.HTTPMaxConnsPerHost)
		}
		if i == 7 || i == 8 {
			require.Equal(t, []int{30000, 50000}[i-7], c.Rate)
		}
		if i >= 9 {
			require.Equal(t, []int{256, 1024, 16384}[i-9], c.PayloadBytes)
			require.Equal(t, 10, c.BatchSize)
			require.Equal(t, 1000, c.Rate)
		}
		require.NoError(t, ValidatePipelineWorkload(PipelineConfig{Rate: c.Rate, Duration: c.Window, BatchSize: c.BatchSize, PayloadBytes: c.PayloadBytes}))
	}
	es, err := Workload(WorkloadConfig{Count: 10, PayloadBytes: 16384})
	require.NoError(t, err)
	inputs := []event.Input{}
	for _, e := range es {
		inputs = append(inputs, e.Input)
	}
	raw, err := json.Marshal(map[string]any{"events": inputs})
	require.NoError(t, err)
	require.Less(t, len(raw), event.MaxRequestBytes)
}
func TestPipelineRejectsOversizedOrLongWorkloadBeforeInfrastructure(t *testing.T) {
	require.ErrorContains(t, ValidatePipelineWorkload(PipelineConfig{Rate: 50000, Duration: 60 * time.Second, BatchSize: 100, PayloadBytes: 1024}), "1000000")
	require.ErrorContains(t, ValidatePipelineWorkload(PipelineConfig{Rate: 1000, Duration: time.Second, BatchSize: 100, PayloadBytes: 16384}), "request")
}
