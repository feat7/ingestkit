package benchmark

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func exerciseConnections(t *testing.T, activeCap, idleCap, held int, http2 bool) ConnectionSnapshot {
	t.Helper()
	entered := make(chan struct{}, 16)
	release := make(chan struct{})
	s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		<-release
		_, _ = w.Write([]byte("ok"))
	}))
	s.EnableHTTP2 = http2
	s.StartTLS()
	recorder := NewConnectionRecorder()
	transport := s.Client().Transport.(*http.Transport).Clone()
	transport.MaxConnsPerHost = activeCap
	transport.MaxIdleConnsPerHost = idleCap
	transport.ForceAttemptHTTP2 = http2
	transport = recorder.WrapTransport(transport)
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
	var wg sync.WaitGroup
	var releaseOnce sync.Once
	errors := make(chan error, 16)
	defer func() { releaseOnce.Do(func() { close(release) }); wg.Wait(); client.CloseIdleConnections(); s.Close() }()
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, e := http.NewRequest("GET", s.URL, nil)
			if e != nil {
				errors <- e
				return
			}
			req, finish := recorder.TraceRequest(req)
			res, e := client.Do(req)
			if e == nil {
				_, _ = io.Copy(io.Discard, res.Body)
				res.Body.Close()
			}
			finish(res, e)
			errors <- e
		}()
	}
	for i := 0; i < held; i++ {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatal("expected active requests did not reach the server")
		}
	}
	before := recorder.Snapshot()
	if http2 {
		require.EqualValues(t, 1, before.PeakOpenTCP)
	} else {
		require.GreaterOrEqual(t, before.PeakOpenTCP, int64(held))
		require.LessOrEqual(t, before.PeakOpenTCP, int64(activeCap))
	}
	releaseOnce.Do(func() { close(release) })
	wg.Wait()
	for i := 0; i < 16; i++ {
		require.NoError(t, <-errors)
	}
	final := recorder.Snapshot()
	require.Zero(t, final.ActiveRequests)
	require.Positive(t, final.ConnectionAcquireNS)
	return final
}

func TestConnectionCapsMeasureSocketsAndWaits(t *testing.T) {
	for _, cap := range []int{1, 8} {
		t.Run(string(rune('0'+cap)), func(t *testing.T) {
			s := exerciseConnections(t, cap, 16, cap, false)
			require.EqualValues(t, cap, s.PeakOpenTCP)
			require.Equal(t, cap, s.MaxConnsPerHost)
		})
	}
}
func TestIdleRetentionDoesNotCapActiveConnections(t *testing.T) {
	s := exerciseConnections(t, 16, 2, 8, false)
	require.Greater(t, s.PeakOpenTCP, int64(2))
	require.Equal(t, 2, s.MaxIdleConnsPerHost)
}
func TestHTTP2MultiplexesRequestsOnOneTCPConnection(t *testing.T) {
	s := exerciseConnections(t, 1, 2, 8, true)
	require.EqualValues(t, 1, s.PeakOpenTCP)
	require.Greater(t, s.PeakActiveRequests, int64(1))
	require.EqualValues(t, 16, s.Protocols["HTTP/2.0"])
}

func TestConnectionWindowResetsCountersKeepsExistingSockets(t *testing.T) {
	r := NewConnectionRecorder()
	r.state.OpenTCP = 3
	r.state.PeakOpenTCP = 8
	r.state.Dials = 10
	r.state.Protocols["HTTP/1.1"] = 9
	r.BeginWindow()
	s := r.Snapshot()
	require.EqualValues(t, 3, s.OpenTCP)
	require.EqualValues(t, 3, s.PeakOpenTCP)
	require.Zero(t, s.Dials)
	require.Empty(t, s.Protocols)
}
