//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"net/http"
	"strings"
	"testing"
	"time"
)

func waitStatus(ctx context.Context, url string, status int) error {
	client := &http.Client{Timeout: 2 * time.Second}
	for {
		req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
		res, err := client.Do(req)
		if err == nil {
			res.Body.Close()
			if res.StatusCode == status {
				return nil
			}
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("waiting for HTTP %d: %w", status, ctx.Err())
		case <-timer.C:
		}
	}
}
func restoreService(t *testing.T, name string) {
	t.Helper()
	ctx, c := context.WithTimeout(context.Background(), 30*time.Second)
	defer c()
	if err := stack.SetServiceRunning(ctx, name, true); err != nil {
		t.Error(err)
	}
}
func TestDatabaseOutageRecovery(t *testing.T) {
	ctx, c := context.WithTimeout(context.Background(), 40*time.Second)
	defer c()
	require.NoError(t, stack.SetServiceRunning(ctx, "postgres", false))
	t.Cleanup(func() { restoreService(t, "postgres") })
	require.NoError(t, waitStatus(ctx, stack.ConsumerURL+"/ready", 503))
	ids := []uuid.UUID{}
	for i := 0; i < 5; i++ {
		e := freshEvent()
		ids = append(ids, e.EventID)
		require.Equal(t, 202, send(t, e, "local-demo-key"))
	}
	require.NoError(t, stack.SetServiceRunning(ctx, "postgres", true))
	require.NoError(t, waitForIDs(ctx, pool, "demo", ids))
	require.NoError(t, waitStatus(ctx, stack.ConsumerURL+"/ready", 200))
	t.Log("recovered all five broker-acknowledged IDs; single-node broker; retention seven days")
}
func TestBrokerOutageAndStableIDRetry(t *testing.T) {
	ctx, c := context.WithTimeout(context.Background(), 45*time.Second)
	defer c()
	require.NoError(t, stack.SetServiceRunning(ctx, "redpanda", false))
	t.Cleanup(func() { restoreService(t, "redpanda") })
	e := freshEvent()
	require.Equal(t, 503, send(t, e, "local-demo-key"))
	require.NoError(t, stack.SetServiceRunning(ctx, "redpanda", true))
	require.NoError(t, waitStatus(ctx, stack.APIURL+"/ready", 200))
	require.Equal(t, 202, send(t, e, "local-demo-key"))
	require.NoError(t, waitForIDs(ctx, pool, "demo", []uuid.UUID{e.EventID}))
	require.Equal(t, 202, send(t, e, "local-demo-key"))
	require.NoError(t, waitForIDs(ctx, pool, "demo", []uuid.UUID{e.EventID}))
}
func TestGracefulAndForcedShutdown(t *testing.T) {
	ctx, c := context.WithTimeout(context.Background(), 40*time.Second)
	defer c()
	e := freshEvent()
	require.Equal(t, 202, send(t, e, "local-demo-key"))
	require.NoError(t, stack.SetServiceRunning(ctx, "api", false))
	t.Cleanup(func() { restoreService(t, "api") })
	out, err := stack.Compose(ctx, "ps", "-a", "--format", "{{.ExitCode}}", "api")
	require.NoError(t, err)
	require.Contains(t, out, "0")
	require.NoError(t, stack.SetServiceRunning(ctx, "api", true))
	require.NoError(t, waitStatus(ctx, stack.APIURL+"/ready", 200))
	_, err = stack.Compose(ctx, "kill", "-s", "KILL", "consumer")
	require.NoError(t, err)
	t.Cleanup(func() { restoreService(t, "consumer") })
	queued := freshEvent()
	require.Equal(t, 202, send(t, queued, "local-demo-key"))
	require.NoError(t, stack.SetServiceRunning(ctx, "consumer", true))
	require.NoError(t, waitForIDs(ctx, pool, "demo", []uuid.UUID{e.EventID, queued.EventID}))
}

func TestConsumerGracefulStopExitsSuccessfully(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	e := freshEvent()
	require.Equal(t, 202, send(t, e, "local-demo-key"))
	require.NoError(t, waitForIDs(ctx, pool, "demo", []uuid.UUID{e.EventID}))
	require.NoError(t, stack.SetServiceRunning(ctx, "consumer", false))
	t.Cleanup(func() { restoreService(t, "consumer") })
	out, err := stack.Compose(ctx, "ps", "-a", "--format", "{{.ExitCode}}", "consumer")
	require.NoError(t, err)
	require.Equal(t, "0", strings.TrimSpace(out))
	require.NoError(t, stack.SetServiceRunning(ctx, "consumer", true))
	require.NoError(t, waitStatus(ctx, stack.ConsumerURL+"/ready", 200))
}
func TestHTTPRejectsUnstoreableJSON(t *testing.T) {
	for _, payload := range []string{`{"s":"\u0000"}`, `{"s":"\ud800"}`, `{"n":1e1000000}`} {
		e := freshEvent()
		e.Payload = json.RawMessage(payload)
		require.Equal(t, 400, send(t, e, "local-demo-key"))
	}
}
