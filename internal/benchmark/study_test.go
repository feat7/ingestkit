package benchmark

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/feat7/ingestkit/internal/event"
	"github.com/stretchr/testify/require"
)

func TestStudyRegistryHas25ControlledCases(t *testing.T) {
	cases := StudyCases()
	require.Len(t, cases, 25)
	counts := map[string]int{}
	seen := map[string]bool{}
	baseline256Batch10 := false
	large := 0
	for _, c := range cases {
		require.False(t, seen[c.ID])
		seen[c.ID] = true
		counts[c.Kind]++
		require.Equal(t, 3, c.Repetitions)
		require.Equal(t, "varied", c.PayloadProfile)
		if c.Kind == "database" {
			require.Equal(t, 5000, c.Count)
			continue
		}
		if c.Kind == "pipeline" {
			require.Equal(t, 10*time.Second, c.Window)
			require.Equal(t, 2*time.Second, c.Warmup)
			require.Equal(t, 30*time.Second, c.Drain)
			require.Equal(t, 50*time.Millisecond, c.Poll)
			if c.PayloadBytes == 256 && c.BatchSize == 10 {
				baseline256Batch10 = true
			}
			if c.PayloadBytes > 256 {
				large++
				require.Equal(t, 10, c.BatchSize)
			}
			inputs, e := Workload(WorkloadConfig{Namespace: "registry", PayloadProfile: c.PayloadProfile, Count: c.BatchSize, PayloadBytes: c.PayloadBytes})
			require.NoError(t, e)
			events := []event.Input{}
			for _, v := range inputs {
				events = append(events, v.Input)
			}
			raw, e := json.Marshal(map[string]any{"events": events})
			require.NoError(t, e)
			require.Less(t, len(raw), event.MaxRequestBytes)
		}
	}
	require.Equal(t, map[string]int{"database": 11, "pipeline": 11, "recovery": 3}, counts)
	require.True(t, baseline256Batch10)
	require.Equal(t, 2, large)
}
func TestJournalRejectsRevisionOrControlChanges(t *testing.T) {
	identity := StudyIdentity{SourceSHA: "revision", SourceFingerprint: "source", ControlsFingerprint: "controls", ImagesFingerprint: "images", ConfigFingerprint: "runtime", Seed: 42, Mode: "full"}
	journal := StudyJournal{Schema: 1, Identity: identity, RunNamespace: "stable", Cases: map[string]CaseProgress{}}
	require.NoError(t, journal.Compatible(identity))
	for _, field := range []string{"source", "controls", "images", "runtime", "revision", "seed", "mode"} {
		changed := identity
		switch field {
		case "source":
			changed.SourceFingerprint = "other"
		case "controls":
			changed.ControlsFingerprint = "other"
		case "images":
			changed.ImagesFingerprint = "other"
		case "runtime":
			changed.ConfigFingerprint = "other"
		case "revision":
			changed.SourceSHA = "other"
		case "seed":
			changed.Seed++
		case "mode":
			changed.Mode = "smoke"
		}
		require.ErrorContains(t, journal.Compatible(changed), field)
	}
}
func TestCompressedArtifactsHaveStableChecksums(t *testing.T) {
	dir := t.TempDir()
	value := map[string]any{"samples": []int64{1, 2, 3}, "case": "example"}
	a, e := WriteCompressed(filepath.Join(dir, "a.json.gz"), value)
	require.NoError(t, e)
	b, e := WriteCompressed(filepath.Join(dir, "b.json.gz"), value)
	require.NoError(t, e)
	require.Equal(t, a.SHA256, b.SHA256)
	first, e := os.ReadFile(filepath.Join(dir, "a.json.gz"))
	require.NoError(t, e)
	second, e := os.ReadFile(filepath.Join(dir, "b.json.gz"))
	require.NoError(t, e)
	require.True(t, bytes.Equal(first, second))
	_, e = WriteCompressed(filepath.Join(dir, "a.json.gz"), map[string]bool{"overwrite": true})
	require.Error(t, e)
	require.NoError(t, VerifyArtifact(dir, a))
	require.Error(t, VerifyArtifact(dir, ArtifactRef{Path: "../outside", SHA256: a.SHA256}))
	first[len(first)-1] ^= 1
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.json.gz"), first, 0600))
	require.Error(t, VerifyArtifact(dir, a))
}
func TestEveryMeasuredRepetitionRetained(t *testing.T) {
	r := PipelineReport{Runs: []PipelineRun{{Repetition: 1, Correct: true}, {Repetition: 2, Correct: true}, {Repetition: 3, Correct: true}}}
	require.NoError(t, validateStudyRepetitions(Case{Kind: "pipeline", Repetitions: 3}, r))
	r.Runs[2].Repetition = 2
	require.Error(t, validateStudyRepetitions(Case{Kind: "pipeline", Repetitions: 3}, r))
	db := goodReport()
	require.NoError(t, validateStudyRepetitions(Case{Kind: "database", Repetitions: 3}, db))
	db.Runs = db.Runs[:8]
	require.Error(t, validateStudyRepetitions(Case{Kind: "database", Repetitions: 3}, db))
}
func TestStudyRejectsOutputRootAndExistingUnownedFiles(t *testing.T) {
	root, e := os.Getwd()
	require.NoError(t, e)
	require.Error(t, RunStudy(context.Background(), StudyConfig{OutputDirectory: root, Smoke: true}))
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("preserve"), 0600))
	require.Error(t, RunStudy(context.Background(), StudyConfig{OutputDirectory: dir, Smoke: true}))
	raw, e := os.ReadFile(filepath.Join(dir, "keep.txt"))
	require.NoError(t, e)
	require.Equal(t, "preserve", string(raw))
}

func TestJSONManifestArtifactsAreImmutableAndChecked(t *testing.T) {
	dir := t.TempDir()
	ref, e := WriteJSONArtifact(filepath.Join(dir, "manifest.json"), map[string]bool{"fsync": true})
	require.NoError(t, e)
	require.NoError(t, VerifyArtifact(dir, ref))
	_, e = WriteJSONArtifact(filepath.Join(dir, "manifest.json"), map[string]bool{"fsync": false})
	require.Error(t, e)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"fsync":false}`), 0600))
	require.Error(t, VerifyArtifact(dir, ref))
}
func TestArtifactReferenceCannotEscapeThroughDirectorySymlink(t *testing.T) {
	dir, elsewhere := t.TempDir(), t.TempDir()
	ref, e := WriteCompressed(filepath.Join(elsewhere, "capture.json.gz"), map[string]int{"sample": 1})
	require.NoError(t, e)
	require.NoError(t, os.Symlink(elsewhere, filepath.Join(dir, "linked")))
	ref.Path = "linked/capture.json.gz"
	require.Error(t, VerifyArtifact(dir, ref))
}

func TestImageIdentityIgnoresOnlyComposeProjectLabels(t *testing.T) {
	a := `[{"Id":"first","Architecture":"arm64","Os":"linux","RootFS":{"Layers":["content-a"]},"Config":{"User":"65532","Labels":{"com.docker.compose.project":"owned-first","com.docker.compose.service":"api"}}}]`
	b := strings.ReplaceAll(a, "first", "second")
	x, e := canonicalImageFingerprint([]byte(a))
	require.NoError(t, e)
	y, e := canonicalImageFingerprint([]byte(b))
	require.NoError(t, e)
	require.Equal(t, x, y)
	changed := strings.ReplaceAll(b, "content-a", "content-b")
	y, e = canonicalImageFingerprint([]byte(changed))
	require.NoError(t, e)
	require.NotEqual(t, x, y)
	changed = strings.ReplaceAll(b, "65532", "0")
	y, e = canonicalImageFingerprint([]byte(changed))
	require.NoError(t, e)
	require.NotEqual(t, x, y)
}

func TestCleanupWaitsForActualCommittedOffsets(t *testing.T) {
	ends := map[int32]int64{0: 10, 1: 0, 2: 7}
	require.False(t, offsetsCaughtUp(ends, map[int32]int64{0: 9, 2: 7}))
	require.False(t, offsetsCaughtUp(ends, map[int32]int64{0: 11, 2: 7}))
	require.False(t, offsetsCaughtUp(ends, map[int32]int64{0: 10}))
	require.True(t, offsetsCaughtUp(ends, map[int32]int64{0: 10, 1: -1, 2: 7}))
}
