//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/feat7/ingestkit/internal/event"
	"github.com/feat7/ingestkit/internal/runenv"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestRecoveryDemoReconciles(t *testing.T) {
	ctx, c := context.WithTimeout(context.Background(), 90*time.Second)
	defer c()
	var output bytes.Buffer
	require.NoError(t, runenv.RunDemo(ctx, runenv.DemoOptions{Recovery: true, Cleanup: true, Writer: &output}))
	require.Contains(t, output.String(), "Reconciled 3 accepted IDs")
	require.Contains(t, output.String(), "database unavailable")
}
func TestDemoInterruptCollectsEvidenceAndScopesCleanup(t *testing.T) {
	ctx, c := context.WithTimeout(context.Background(), 90*time.Second)
	var directory string
	var output bytes.Buffer
	err := runenv.RunDemo(ctx, runenv.DemoOptions{Recovery: true, Cleanup: true, Writer: &output, OnStage: func(stage string, s *runenv.Stack) {
		directory = s.Directory
		if stage == "database_unavailable" {
			c()
		}
	}})
	require.Error(t, err)
	require.FileExists(t, filepath.Join(directory, "services.log"))
	require.FileExists(t, filepath.Join(directory, "demo.json"))
	probe, end := context.WithTimeout(context.Background(), 5*time.Second)
	defer end()
	require.NoError(t, waitStatus(probe, stack.APIURL+"/ready", 200))
}
func TestPythonAndNodeCallersAgree(t *testing.T) {
	root, err := runenv.Root()
	require.NoError(t, err)
	for _, command := range [][]string{{"python3", "examples/checkout/send.py"}, {"node", "examples/checkout/send.mjs"}} {
		cmd := exec.Command(command[0], command[1])
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "API_URL="+stack.APIURL, "INGESTKIT_API_KEY=local-demo-key")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
		require.Contains(t, string(out), "accepted")
		cmd = exec.Command(command[0], command[1])
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "API_URL=http://127.0.0.1:1", "INGESTKIT_API_KEY=local-demo-key")
		require.Error(t, cmd.Run())
	}
	raw, err := os.ReadFile(filepath.Join(root, "examples/checkout/events.json"))
	require.NoError(t, err)
	var fixture struct{ Events []event.Input }
	require.NoError(t, json.Unmarshal(raw, &fixture))
	ids := []uuid.UUID{}
	for _, e := range fixture.Events {
		ids = append(ids, e.EventID)
	}
	ctx, c := context.WithTimeout(context.Background(), 10*time.Second)
	defer c()
	require.NoError(t, waitForIDs(ctx, pool, "demo", ids))
}

func TestCallerModesPreserveRetryBytes(t *testing.T) {
	root, e := runenv.Root()
	require.NoError(t, e)
	for _, command := range [][]string{{"python3", "examples/checkout/send.py"}, {"node", "examples/checkout/send.mjs"}} {
		for _, mode := range []string{"single", "batch"} {
			t.Run(command[0]+"/"+mode, func(t *testing.T) {
				var mu sync.Mutex
				attempts := map[string]int{}
				wrong := false
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					want := "/v1/events"
					if mode == "batch" {
						want += "/batch"
					}
					if r.URL.Path != want {
						mu.Lock()
						wrong = true
						mu.Unlock()
						w.WriteHeader(400)
						return
					}
					raw, _ := io.ReadAll(r.Body)
					mu.Lock()
					attempts[string(raw)]++
					n := attempts[string(raw)]
					mu.Unlock()
					if n == 1 {
						w.WriteHeader(503)
						return
					}
					var ids []uuid.UUID
					if mode == "single" {
						var in event.Input
						if json.Unmarshal(raw, &in) != nil {
							w.WriteHeader(400)
							return
						}
						ids = append(ids, in.EventID)
					} else {
						var in struct{ Events []event.Input }
						if json.Unmarshal(raw, &in) != nil {
							w.WriteHeader(400)
							return
						}
						for _, v := range in.Events {
							ids = append(ids, v.EventID)
						}
					}
					w.WriteHeader(202)
					_ = json.NewEncoder(w).Encode(map[string]any{"status": "accepted", "event_ids": ids})
				}))
				defer server.Close()
				cmd := exec.Command(command[0], command[1])
				cmd.Dir = root
				cmd.Env = append(os.Environ(), "API_URL="+server.URL, "SEND_MODE="+mode)
				out, e := cmd.CombinedOutput()
				require.NoError(t, e, string(out))
				mu.Lock()
				defer mu.Unlock()
				require.False(t, wrong)
				want := 3
				if mode == "batch" {
					want = 1
				}
				require.Len(t, attempts, want)
				for _, n := range attempts {
					require.Equal(t, 2, n)
				}
			})
		}
	}
}
