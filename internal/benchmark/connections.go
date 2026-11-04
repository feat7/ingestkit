package benchmark

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptrace"
	"sync"
	"syscall"
	"time"
)

type ConnectionSnapshot struct {
	ActiveRequests, PeakActiveRequests             int64
	OpenTCP, PeakOpenTCP, Dials, Reused            int64
	ConnectionAcquireNS                            int64
	WaitingForConnection, PeakWaitingForConnection int64
	MaxConnsPerHost, MaxIdleConnsPerHost           int
	Protocols, Errors, TLSVersions                 map[string]int64
	TCPTrackingStatus                              string
}
type ConnectionRecorder struct {
	mu    sync.Mutex
	state ConnectionSnapshot
}

func NewConnectionRecorder() *ConnectionRecorder {
	return &ConnectionRecorder{state: ConnectionSnapshot{Protocols: map[string]int64{}, Errors: map[string]int64{}, TLSVersions: map[string]int64{}, TCPTrackingStatus: "not_configured"}}
}

// Clone preserves caller TLS trust and protocol settings. Dial tracking counts
// transport-owned established sockets, not every socket opened by the process.
func (r *ConnectionRecorder) WrapTransport(t *http.Transport) *http.Transport {
	clone := t.Clone()
	r.mu.Lock()
	r.state.MaxConnsPerHost = clone.MaxConnsPerHost
	r.state.MaxIdleConnsPerHost = clone.MaxIdleConnsPerHost
	if clone.MaxIdleConnsPerHost == 0 {
		r.state.MaxIdleConnsPerHost = http.DefaultMaxIdleConnsPerHost
	}
	r.state.TCPTrackingStatus = "transport_owned_connected_tcp"
	if clone.DialTLSContext != nil || clone.DialTLS != nil {
		r.state.TCPTrackingStatus = "unavailable_custom_tls_dial"
	}
	r.mu.Unlock()
	dial := clone.DialContext
	if dial == nil {
		if clone.Dial != nil {
			dial = func(_ context.Context, network, address string) (net.Conn, error) {
				return clone.Dial(network, address)
			}
		} else {
			dial = (&net.Dialer{}).DialContext
		}
	}
	clone.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		r.mu.Lock()
		r.state.Dials++
		r.mu.Unlock()
		conn, e := dial(ctx, network, address)
		if e != nil {
			return nil, e
		}
		if network != "tcp" && network != "tcp4" && network != "tcp6" {
			return conn, nil
		}
		r.mu.Lock()
		r.state.OpenTCP++
		r.state.PeakOpenTCP = max(r.state.PeakOpenTCP, r.state.OpenTCP)
		r.mu.Unlock()
		return &recordedConn{Conn: conn, closed: func() { r.mu.Lock(); r.state.OpenTCP--; r.mu.Unlock() }}, nil
	}
	return clone
}

// Finish runs after consuming/closing the response body. Only bounded error
// classes and protocol names are retained; addresses and raw errors are absent.
func (r *ConnectionRecorder) TraceRequest(req *http.Request) (*http.Request, func(*http.Response, error)) {
	r.mu.Lock()
	r.state.ActiveRequests++
	r.state.PeakActiveRequests = max(r.state.PeakActiveRequests, r.state.ActiveRequests)
	r.mu.Unlock()
	var began time.Time
	waiting := false
	trace := &httptrace.ClientTrace{
		GetConn: func(string) {
			r.mu.Lock()
			defer r.mu.Unlock()
			if !waiting {
				waiting = true
				began = time.Now()
				r.state.WaitingForConnection++
				r.state.PeakWaitingForConnection = max(r.state.PeakWaitingForConnection, r.state.WaitingForConnection)
			}
		},
		GotConn: func(info httptrace.GotConnInfo) {
			r.mu.Lock()
			defer r.mu.Unlock()
			if waiting {
				r.state.WaitingForConnection--
				r.state.ConnectionAcquireNS += time.Since(began).Nanoseconds()
				waiting = false
			}
			if info.Reused {
				r.state.Reused++
			}
		},
		TLSHandshakeDone: func(state tls.ConnectionState, e error) {
			if e == nil {
				r.mu.Lock()
				r.state.TLSVersions[tls.VersionName(state.Version)]++
				r.mu.Unlock()
			}
		},
	}
	var once sync.Once
	finish := func(res *http.Response, e error) {
		once.Do(func() {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.state.ActiveRequests--
			if waiting {
				r.state.WaitingForConnection--
				r.state.ConnectionAcquireNS += time.Since(began).Nanoseconds()
				waiting = false
			}
			if res != nil {
				protocol := res.Proto
				if protocol != "HTTP/1.1" && protocol != "HTTP/2.0" && protocol != "HTTP/1.0" {
					protocol = "other"
				}
				r.state.Protocols[protocol]++
			}
			if e != nil {
				r.state.Errors[connectionErrorClass(e)]++
			}
		})
	}
	return req.WithContext(httptrace.WithClientTrace(req.Context(), trace)), finish
}
func (r *ConnectionRecorder) Snapshot() ConnectionSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.state
	clone := func(m map[string]int64) map[string]int64 {
		c := make(map[string]int64, len(m))
		for k, v := range m {
			c[k] = v
		}
		return c
	}
	out.Protocols = clone(r.state.Protocols)
	out.Errors = clone(r.state.Errors)
	out.TLSVersions = clone(r.state.TLSVersions)
	return out
}

type recordedConn struct {
	net.Conn
	once   sync.Once
	closed func()
}

func (c *recordedConn) Close() error { e := c.Conn.Close(); c.once.Do(c.closed); return e }
func connectionErrorClass(err error) string {
	if errors.Is(err, syscall.EMFILE) || errors.Is(err, syscall.ENFILE) {
		return "fd_limit"
	}
	if errors.Is(err, syscall.EADDRNOTAVAIL) || errors.Is(err, syscall.EADDRINUSE) {
		return "address_unavailable"
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return "connection_refused"
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	var networkError net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &networkError) && networkError.Timeout()) {
		return "timeout"
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		return "dns"
	}
	return "other"
}

// BeginWindow excludes warm-up counts, retaining sockets already available for reuse.
// Call only after all requests from the previous window have finished.
func (r *ConnectionRecorder) BeginWindow() {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.state
	r.state = ConnectionSnapshot{OpenTCP: s.OpenTCP, PeakOpenTCP: s.OpenTCP, MaxConnsPerHost: s.MaxConnsPerHost, MaxIdleConnsPerHost: s.MaxIdleConnsPerHost, TCPTrackingStatus: s.TCPTrackingStatus, Protocols: map[string]int64{}, Errors: map[string]int64{}, TLSVersions: map[string]int64{}}
}
