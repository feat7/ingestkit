package benchmark

import (
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
