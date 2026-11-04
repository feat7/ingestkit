//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/feat7/ingestkit/internal/benchmark"
	"github.com/feat7/ingestkit/internal/runenv"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestStudySmokeAndImmutableResume(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	dir := t.TempDir()
	first, stopFirst := context.WithCancel(ctx)
	cfg := benchmark.StudyConfig{OutputDirectory: dir, Smoke: true, Seed: 42, OnCaseStart: func(c benchmark.Case, _ *runenv.Stack) {
		if c.Kind == "pipeline" {
			stopFirst()
		}
	}}
	require.Error(t, benchmark.RunStudy(first, cfg))
	stopFirst()
	partialRaw, e := os.ReadFile(filepath.Join(dir, "index.json"))
	require.NoError(t, e)
	var partial benchmark.StudyIndex
	require.NoError(t, json.Unmarshal(partialRaw, &partial))
	databaseRef := partial.Cases[0].Artifact
	databaseBytes, e := os.ReadFile(filepath.Join(dir, databaseRef.Path))
	require.NoError(t, e)
	// Resume on a fresh owned stack; the completed database case stays immutable.
	cfg.OnCaseStart = nil
	cfg.Resume = true
	require.NoError(t, benchmark.RunStudy(ctx, cfg))
	raw, e := os.ReadFile(filepath.Join(dir, "index.json"))
	require.NoError(t, e)
	var index benchmark.StudyIndex
	require.NoError(t, json.Unmarshal(raw, &index))
	require.Equal(t, "complete", index.Status)
	require.Equal(t, "smoke", index.Identity.Mode)
	require.Len(t, index.Cases, 2)
	require.Equal(t, databaseRef, index.Cases[0].Artifact)
	require.Equal(t, 1, index.Cases[0].Attempt)
	preserved, e := os.ReadFile(filepath.Join(dir, databaseRef.Path))
	require.NoError(t, e)
	require.Equal(t, databaseBytes, preserved)

	captures := map[string][]byte{}
	for _, c := range index.Cases {
		require.Contains(t, []string{"complete", "limited"}, c.Status)
		require.NoError(t, benchmark.VerifyArtifact(dir, c.Artifact))
		raw, e := os.ReadFile(filepath.Join(dir, c.Artifact.Path))
		require.NoError(t, e)
		captures[c.Artifact.Path] = raw
	}
	cfg.Resume = true
	require.NoError(t, benchmark.RunStudy(ctx, cfg))
	for path, expected := range captures {
		actual, e := os.ReadFile(filepath.Join(dir, path))
		require.NoError(t, e)
		require.Equal(t, expected, actual)
	}
	cfg.Seed++
	require.Error(t, benchmark.RunStudy(ctx, cfg))
}
func TestStudyInterruptPreservesEvidenceAndOwnership(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dir := t.TempDir()
	var owned *runenv.Stack
	err := benchmark.RunStudy(ctx, benchmark.StudyConfig{OutputDirectory: dir, Smoke: true, Seed: 42, OnCaseStart: func(_ benchmark.Case, s *runenv.Stack) { owned = s; cancel() }})
	require.Error(t, err)
	require.NotNil(t, owned)
	require.FileExists(t, filepath.Join(dir, "journal.json"))
	require.FileExists(t, filepath.Join(owned.Directory, "services.log"))
	probe, end := context.WithTimeout(context.Background(), 10*time.Second)
	defer end()
	ids, e := owned.Compose(probe, "ps", "-a", "-q")
	require.NoError(t, e)
	require.Empty(t, ids)
	original := owned.Project
	owned.Project = stack.Project
	require.Error(t, owned.Stop(probe))
	owned.Project = original
	require.NoError(t, waitStatus(probe, stack.APIURL+"/ready", 200))
}
func TestStudyRecoveryExperimentsRecordRealFaults(t *testing.T) {
	s, _ := deployment(t, true)
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	p := deploymentPool(t, ctx, s)
	for _, c := range benchmark.StudyCases() {
		if c.Kind != "recovery" {
			continue
		}
		t.Run(c.ID, func(t *testing.T) {
			report, e := benchmark.RunRecoveryCase(ctx, s, p, c, 42, "recovery-test-"+uuid.NewString())
			require.NoError(t, e)
			require.Len(t, report.Runs, 3)
			for _, r := range report.Runs {
				require.True(t, r.Correct)
				require.GreaterOrEqual(t, len(r.Faults), 2)
				require.NotEmpty(t, r.AcceptedIDs)
				require.ElementsMatch(t, r.AcceptedIDs, r.StoredIDs)
				if c.Recovery == "outbox" {
					require.GreaterOrEqual(t, r.FirstAttemptNS, int64(5*time.Second))
					require.Contains(t, []string{"http_503", "delivery_uncertain"}, r.FirstAttemptStatus)
					require.Len(t, r.AmbiguousIDs, 100)
				}
			}
		})
	}
}
