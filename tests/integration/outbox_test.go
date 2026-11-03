//go:build integration

package integration

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/feat7/ingestkit/internal/runenv"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func outboxCommand(t *testing.T, ctx context.Context, s *runenv.Stack, database string, args ...string) ([]byte, error) {
	t.Helper()
	root, e := runenv.Root()
	require.NoError(t, e)
	command := append([]string{"-m", "examples.outbox"}, args...)
	command = append(command, "--database", database)
	cmd := exec.CommandContext(ctx, "python3", command...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "API_URL="+s.APIURL, "INGESTKIT_API_KEY="+s.APIKey(), "INGESTKIT_CA_FILE="+filepath.Join(s.Directory, "root.crt"))
	return cmd.CombinedOutput()
}
func createOutboxOrder(t *testing.T, ctx context.Context, s *runenv.Stack, database string) uuid.UUID {
	t.Helper()
	out, e := outboxCommand(t, ctx, s, database, "create-order", "--order-id", "order-"+uuid.NewString(), "--amount-cents", "2500")
	require.NoError(t, e, string(out))
	var created struct {
		ID uuid.UUID `json:"event_id"`
	}
	require.NoError(t, json.Unmarshal(out, &created))
	require.NotEqual(t, uuid.Nil, created.ID)
	return created.ID
}
func assertOutboxState(t *testing.T, ctx context.Context, s *runenv.Stack, database, state string, n int) {
	t.Helper()
	out, e := outboxCommand(t, ctx, s, database, "status")
	require.NoError(t, e, string(out))
	var counts map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(out, &counts))
	var found int
	require.NoError(t, json.Unmarshal(counts[state], &found))
	require.Equal(t, n, found)
}
func TestOutboxSurvivesBrokerOutage(t *testing.T) {
	s, ctx := deployment(t, true)
	p := deploymentPool(t, ctx, s)
	database := filepath.Join(t.TempDir(), "orders.db")
	id := createOutboxOrder(t, ctx, s, database)
	require.NoError(t, s.SetServiceRunning(ctx, "redpanda", false))
	t.Cleanup(func() {
		end, c := context.WithTimeout(context.Background(), 30*time.Second)
		defer c()
		require.NoError(t, s.SetServiceRunning(end, "redpanda", true))
	})
	out, e := outboxCommand(t, ctx, s, database, "dispatch", "--max-attempts", "1")
	require.Error(t, e)
	require.Contains(t, string(out), `"acknowledged": 0`)
	assertOutboxState(t, ctx, s, database, "pending", 1)
	require.NoError(t, s.SetServiceRunning(ctx, "redpanda", true))
	client, e := s.HTTPClient()
	require.NoError(t, e)
	require.Eventually(t, func() bool {
		res, e := client.Get(s.APIURL + "/ready")
		if e != nil {
			return false
		}
		defer res.Body.Close()
		return res.StatusCode == http.StatusOK
	}, 30*time.Second, 100*time.Millisecond)
	out, e = outboxCommand(t, ctx, s, database, "dispatch")
	require.NoError(t, e, string(out))
	assertOutboxState(t, ctx, s, database, "acknowledged", 1)
	require.NoError(t, waitForIDs(ctx, p, "default", []uuid.UUID{id}))
}
func TestOutboxCrashAfterAckReplaysSafely(t *testing.T) {
	s, ctx := deployment(t, true)
	p := deploymentPool(t, ctx, s)
	database := filepath.Join(t.TempDir(), "orders.db")
	id := createOutboxOrder(t, ctx, s, database)
	root, e := runenv.Root()
	require.NoError(t, e)
	script, e := os.ReadFile(filepath.Join(root, "tests/fixtures/outbox/crash_dispatch.py"))
	require.NoError(t, e)
	cmd := exec.CommandContext(ctx, "python3", "-c", string(script))
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "API_URL="+s.APIURL, "INGESTKIT_API_KEY="+s.APIKey(), "INGESTKIT_CA_FILE="+filepath.Join(s.Directory, "root.crt"), "OUTBOX_DATABASE="+database)
	stdout, e := cmd.StdoutPipe()
	require.NoError(t, e)
	require.NoError(t, cmd.Start())
	defer func() {
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	signaled := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			signaled <- scanner.Text()
		} else {
			signaled <- "child exited before ACK"
		}
	}()
	select {
	case line := <-signaled:
		require.Equal(t, "broker-acknowledged", line)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	require.NoError(t, cmd.Process.Kill())
	require.Error(t, cmd.Wait())
	assertOutboxState(t, ctx, s, database, "pending", 1)
	require.NoError(t, waitForIDs(ctx, p, "default", []uuid.UUID{id}))
	out, e := outboxCommand(t, ctx, s, database, "dispatch")
	require.NoError(t, e, string(out))
	assertOutboxState(t, ctx, s, database, "acknowledged", 1)
	require.NoError(t, waitForIDs(ctx, p, "default", []uuid.UUID{id}))
	var n int
	require.NoError(t, p.QueryRow(ctx, "SELECT count(*) FROM ingestkit.events WHERE tenant_id='default' AND event_id=$1", id.String()).Scan(&n))
	require.Equal(t, 1, n)
}
