//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/feat7/ingestkit/internal/benchmark"
	"github.com/feat7/ingestkit/internal/event"
	"github.com/feat7/ingestkit/internal/runenv"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

var stack *runenv.Stack
var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	if os.Getenv("INGESTKIT_INTEGRATION") != "1" {
		fmt.Fprintln(os.Stderr, "run scripts/test-integration.sh; real isolated dependencies are required")
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	var err error
	stack, err = runenv.Start(ctx, runenv.Options{Purpose: "integration"})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	pool, err = pgxpool.New(ctx, stack.DatabaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := 0
	_, err = pool.Exec(ctx, "CREATE TABLE legacy_sentinel(id int); INSERT INTO legacy_sentinel VALUES(7); CREATE TABLE schema_migrations(version bigint,dirty boolean); INSERT INTO schema_migrations VALUES(20251115110430,false)")
	if err == nil {
		_, err = stack.Compose(ctx, "run", "--rm", "migrate")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		code = 1
	} else {
		root, _ := runenv.Root()
		cmd := exec.CommandContext(ctx, "go", "test", "-race", "-tags=integration", "./internal/persistence", "./internal/messaging", "./internal/benchmark", "-count=1")
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "INGESTKIT_TEST_DATABASE_URL="+stack.DatabaseURL, "INGESTKIT_TEST_BROKER="+stack.Brokers, "INGESTKIT_TEST_API_URL="+stack.APIURL)
		out, e := cmd.CombinedOutput()
		fmt.Print(string(out))
		if e != nil {
			code = 1
		} else {
			code = m.Run()
		}
	}
	end, done := context.WithTimeout(context.Background(), 25*time.Second)
	defer done()
	if err := stack.CollectEvidence(end, stack.Directory); err != nil {
		fmt.Fprintln(os.Stderr, err)
		code = 1
	}
	pool.Close()
	if err := stack.Destroy(end); err != nil {
		fmt.Fprintln(os.Stderr, err)
		code = 1
	}
	fmt.Println("integration evidence:", stack.Directory)
	os.Exit(code)
}
func send(t *testing.T, env event.Input, key string) int {
	t.Helper()
	raw, err := json.Marshal(env)
	require.NoError(t, err)
	req, err := http.NewRequest("POST", stack.APIURL+"/v1/events", bytes.NewReader(raw))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 8 * time.Second}
	res, err := client.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode == 202 {
		var ack struct {
			IDs []string `json:"event_ids"`
		}
		require.NoError(t, json.Unmarshal(body, &ack))
		require.Equal(t, []string{env.EventID.String()}, ack.IDs)
	}
	return res.StatusCode
}
func freshEvent() event.Input {
	return event.Input{EventID: uuid.New(), EventType: "order_completed", OccurredAt: time.Now().UTC(), Payload: json.RawMessage("{\"amount\":25}")}
}
func waitForIDs(ctx context.Context, p *pgxpool.Pool, tenant string, ids []uuid.UUID) error {
	for {
		found := 0
		for _, id := range ids {
			var n int
			err := p.QueryRow(ctx, "SELECT count(*) FROM ingestkit.events WHERE tenant_id=$1 AND event_id=$2", tenant, id.String()).Scan(&n)
			if err == nil && n == 1 {
				found++
			}
		}
		if found == len(ids) {
			return nil
		}
		timer := time.NewTimer(50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}
func TestPipelineHappyPath(t *testing.T) {
	ctx, c := context.WithTimeout(context.Background(), 10*time.Second)
	defer c()
	e := freshEvent()
	require.Equal(t, 202, send(t, e, "local-demo-key"))
	require.NoError(t, waitForIDs(ctx, pool, "demo", []uuid.UUID{e.EventID}))
	require.Equal(t, 202, send(t, e, "local-demo-key"))
	require.Equal(t, 202, send(t, e, "local-second-key"))
	require.NoError(t, waitForIDs(ctx, pool, "second", []uuid.UUID{e.EventID}))
	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM ingestkit.events WHERE event_id=$1", e.EventID.String()).Scan(&n))
	require.Equal(t, 2, n)
}
func TestIntegrationMissingConfigurationFails(t *testing.T) {
	cmd := exec.Command("go", "test", "-tags=integration", "./tests/integration", "-run", "^$")
	root, _ := runenv.Root()
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "INGESTKIT_INTEGRATION=0")
	out, err := cmd.CombinedOutput()
	require.Error(t, err)
	require.Contains(t, string(out), "real isolated dependencies")
}

func TestConcurrentStacksIsolateResources(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	other, err := runenv.Start(ctx, runenv.Options{Purpose: "isolation"})
	require.NoError(t, err)
	defer func() {
		cleanup, end := context.WithTimeout(context.Background(), 25*time.Second)
		defer end()
		require.NoError(t, other.Destroy(cleanup))
	}()
	require.NotEqual(t, stack.Project, other.Project)
	require.NotEqual(t, stack.APIURL, other.APIURL)
	require.NotEqual(t, stack.DatabaseURL, other.DatabaseURL)
	require.NotEqual(t, stack.Brokers, other.Brokers)
	ownName := other.Project
	other.Project = stack.Project
	require.Error(t, other.Stop(ctx), "must refuse a different owner's project")
	other.Project = ownName
	res, err := http.Get(stack.APIURL + "/ready")
	require.NoError(t, err)
	res.Body.Close()
	require.Equal(t, 200, res.StatusCode)
}

func TestBrokerAcknowledgementsUseDiskFlush(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := kgo.NewClient(kgo.SeedBrokers(stack.Brokers))
	require.NoError(t, err)
	defer client.Close()
	for _, topic := range []string{"ingestkit.events", "ingestkit.dlq"} {
		req := kmsg.NewPtrDescribeConfigsRequest()
		req.Resources = []kmsg.DescribeConfigsRequestResource{{ResourceType: 2, ResourceName: topic}}
		res, err := client.Request(ctx, req)
		require.NoError(t, err)
		found := false
		for _, resource := range res.(*kmsg.DescribeConfigsResponse).Resources {
			for _, entry := range resource.Configs {
				if entry.Name == "write.caching" {
					require.NotNil(t, entry.Value)
					require.Equal(t, "false", *entry.Value)
					found = true
				}
			}
		}
		require.True(t, found, "topic durability configuration must be observable")
	}
}

func TestBrokerDoesNotBypassFsync(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Inspect the running broker, since the dev-container mode adds flags that do
	// not appear literally in Compose or in the topic configuration response.
	command := `for p in /proc/[0-9]*; do case "$(readlink "$p/exe" 2>/dev/null)" in */redpanda) cat "$p/cmdline"; exit 0;; esac; done; exit 1`
	raw, err := stack.Compose(ctx, "exec", "-T", "redpanda", "sh", "-c", command)
	require.NoError(t, err)
	args := strings.Split(strings.TrimRight(raw, "\x00"), "\x00")
	require.NotEmpty(t, args)
	require.True(t, strings.HasSuffix(args[0], "/redpanda"))
	for i, arg := range args {
		if strings.HasPrefix(arg, "--unsafe-bypass-fsync=") {
			require.Contains(t, []string{"false", "0"}, strings.TrimPrefix(arg, "--unsafe-bypass-fsync="), "broker acknowledgements must not bypass fsync")
		}
		if arg == "--unsafe-bypass-fsync" {
			require.Less(t, i+1, len(args))
			require.Contains(t, []string{"false", "0"}, args[i+1])
		}
	}
}

func TestBenchmarkManifestRecordsBrokerFsync(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	manifest, err := benchmark.CaptureManifest(ctx, stack, pool)
	require.NoError(t, err)
	raw, err := json.Marshal(manifest)
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(raw, &fields))
	require.Equal(t, false, fields["broker_fsync_bypass"], "future performance captures must include the effective broker fsync state")
}
