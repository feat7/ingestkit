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
	"os"
	"os/exec"
	"path/filepath"
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
