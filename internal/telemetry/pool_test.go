package telemetry

import (
	"github.com/feat7/ingestkit/internal/runtimeinfo"
	"github.com/stretchr/testify/require"
	"testing"
)

type bufferFixture struct{}

func (bufferFixture) BufferedProduceRecords() int64 { return 7 }
func (bufferFixture) BufferedProduceBytes() int64   { return 8192 }
func TestRuntimeSourcesAndUnavailableLimits(t *testing.T) {
	m := New()
	m.runtime.capture = func() runtimeinfo.ProcessLimits { return runtimeinfo.ProcessLimits{} }
	m.RegisterProducer(bufferFixture{}, 1000, 67108864)
	m.RegisterProducer(bufferFixture{}, 1000, 67108864)
	s := scrape(t, m)
	require.Contains(t, s, "ingestkit_producer_buffered_records 7")
	require.Contains(t, s, "ingestkit_producer_buffered_bytes 8192")
	require.Contains(t, s, "ingestkit_producer_max_buffered_records 1000")
	require.Contains(t, s, "ingestkit_process_limit_available{resource=\"process_open_fds\"} 0")
	require.NotContains(t, s, "\ningestkit_process_open_fds 0")
	m.ObservePublishFailure("secret-event-id")
	require.NotContains(t, scrape(t, m), "secret-event-id")
}
