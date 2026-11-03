package api

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/feat7/ingestkit/internal/event"
	"github.com/feat7/ingestkit/internal/telemetry"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakePublisher struct {
	mu      sync.Mutex
	events  []event.Envelope
	publish func(context.Context, []event.Envelope) error
	pingErr error
}

func (p *fakePublisher) Publish(c context.Context, e []event.Envelope) error {
	p.mu.Lock()
	p.events = append(p.events, e...)
	p.mu.Unlock()
	if p.publish != nil {
		return p.publish(c, e)
	}
	return nil
}
func (p *fakePublisher) Ping(context.Context) error { return p.pingErr }

const validEvent = `{"event_id":"00000000-0000-4000-8000-000000000001","event_type":"order_completed","occurred_at":"2026-11-03T00:00:00Z","payload":{"amount":25}}`

func appForTest(t *testing.T, p *fakePublisher) *fiber.App {
	a, clean, err := NewApp(Config{APIKeys: map[string]string{"test-key": "tenant-a"}, RateLimitRPS: 1000, PublishTimeout: 50 * time.Millisecond}, p, telemetry.New())
	require.NoError(t, err)
	t.Cleanup(clean)
	return a
}
func request(t *testing.T, a *fiber.App, path, body, key string) *http.Response {
	t.Helper()
	r := httptest.NewRequest("POST", path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if key != "" {
		r.Header.Set("Authorization", "Bearer "+key)
	}
	res, err := a.Test(r, -1)
	require.NoError(t, err)
	t.Cleanup(func() { res.Body.Close() })
	return res
}
func TestIngestAcknowledgementBoundary(t *testing.T) {
	called, ack := make(chan struct{}), make(chan struct{})
	p := &fakePublisher{publish: func(ctx context.Context, _ []event.Envelope) error {
		close(called)
		select {
		case <-ack:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}}
	a := appForTest(t, p)
	done := make(chan *http.Response, 1)
	go func() {
		r := httptest.NewRequest("POST", "/v1/events", strings.NewReader(validEvent))
		r.Header.Set("Authorization", "Bearer test-key")
		res, _ := a.Test(r, -1)
		done <- res
	}()
	<-called
	select {
	case <-done:
		t.Fatal("acknowledged too early")
	default:
	}
	close(ack)
	res := <-done
	require.NotNil(t, res)
	defer res.Body.Close()
	require.Equal(t, 202, res.StatusCode)
}
func TestIngestSingleAndBatchContract(t *testing.T) {
	for _, tc := range []struct{ path, body string }{{"/v1/events", validEvent}, {"/v1/events/batch", `{"events":[` + validEvent + `]}`}} {
		p := &fakePublisher{}
		res := request(t, appForTest(t, p), tc.path, tc.body, "test-key")
		require.Equal(t, 202, res.StatusCode)
		var body map[string]any
		require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
		require.Equal(t, "accepted", body["status"])
		require.NotEmpty(t, body["request_id"])
		require.Len(t, body["event_ids"], 1)
		require.Equal(t, "tenant-a", p.events[0].TenantID)
	}
}
func TestInvalidBatchProducesNothing(t *testing.T) {
	p := &fakePublisher{}
	res := request(t, appForTest(t, p), "/v1/events/batch", `{"events":[`+validEvent+`,{}]}`, "test-key")
	require.Equal(t, 400, res.StatusCode)
	require.Empty(t, p.events)
}
func TestIngestRejectsIdentityInjection(t *testing.T) {
	a := appForTest(t, &fakePublisher{})
	res := request(t, a, "/v1/events", strings.Replace(validEvent, `"payload"`, `"tenant_id":"evil","payload"`, 1), "test-key")
	require.Equal(t, 400, res.StatusCode)
}
func TestIngestEnforcesBodyAndEventLimits(t *testing.T) {
	a := appForTest(t, &fakePublisher{})
	for _, n := range []int{event.MaxEventBytes, event.MaxRequestBytes} {
		body := strings.Replace(validEvent, "25", `"`+strings.Repeat("x", n)+`"`, 1)
		require.Equal(t, 413, request(t, a, "/v1/events", body, "test-key").StatusCode)
	}
}
func TestIngestBrokerTimeoutAndPartialFailure(t *testing.T) {
	p := &fakePublisher{publish: func(ctx context.Context, _ []event.Envelope) error {
		<-ctx.Done()
		return errors.New("secret-key-internal-error")
	}}
	a := appForTest(t, p)
	res := request(t, a, "/v1/events", validEvent, "test-key")
	require.Equal(t, 503, res.StatusCode)
	b, _ := io.ReadAll(res.Body)
	require.NotContains(t, string(b), "secret-key")
	require.Contains(t, string(b), "same event IDs")
}
func TestAuthAndRateLimit(t *testing.T) {
	p := &fakePublisher{}
	a, clean, err := NewApp(Config{APIKeys: map[string]string{"key": "tenant"}, RateLimitRPS: 1, PublishTimeout: time.Second}, p, telemetry.New())
	require.NoError(t, err)
	defer clean()
	require.Equal(t, 401, request(t, a, "/v1/events", validEvent, "").StatusCode)
	require.Equal(t, 202, request(t, a, "/v1/events", validEvent, "key").StatusCode)
	require.Equal(t, 429, request(t, a, "/v1/events", validEvent, "key").StatusCode)
}
func TestReadyChecksPublisher(t *testing.T) {
	a := appForTest(t, &fakePublisher{pingErr: errors.New("down")})
	for _, path := range []string{"/health", "/ready"} {
		res, err := a.Test(httptest.NewRequest("GET", path, nil))
		require.NoError(t, err)
		want := 200
		if path == "/ready" {
			want = 503
		}
		require.Equal(t, want, res.StatusCode)
		res.Body.Close()
	}
}
func TestMetricsRoute(t *testing.T) {
	a := appForTest(t, &fakePublisher{})
	res, err := a.Test(httptest.NewRequest("GET", "/metrics", nil))
	require.NoError(t, err)
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	require.Contains(t, string(b), "ingestkit_stored_total")
	require.Equal(t, 404, request(t, a, "/v1/events/user_signup", validEvent, "test-key").StatusCode)
}
