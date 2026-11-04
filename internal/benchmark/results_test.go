package benchmark

import (
	"encoding/csv"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestPipelineCSVLeavesFailedVisibilityEmpty(t *testing.T) {
	dir := t.TempDir()
	r := PipelineReport{Schema: 2, Runs: []PipelineRun{{Correct: true, DeliveryStatus: "passed", ObservationStatus: "failed", Visibility: Latency{P95: 999}}, {Correct: true}, {Correct: true}}}
	require.NoError(t, WriteReport(dir, r))
	f, e := os.Open(filepath.Join(dir, "pipeline.csv"))
	require.NoError(t, e)
	defer f.Close()
	rows, e := csv.NewReader(f).ReadAll()
	require.NoError(t, e)
	column := slices.Index(rows[0], "visibility_p95_ns")
	require.GreaterOrEqual(t, column, 0)
	require.Empty(t, rows[1][column], "failed visibility must not leak an internal partial/zero percentile")
}

func goodReport() DatabaseReport {
	r := DatabaseReport{Repetitions: 3}
	for i := 0; i < 3; i++ {
		for _, m := range methods {
			r.Runs = append(r.Runs, DatabaseRun{Method: m, Repetition: i + 1, Submitted: 10, Inserted: 8, Duplicates: 2, ElapsedNS: 1000, Correct: true})
		}
	}
	return r
}
func TestDatabaseReportRejectsIncorrectRun(t *testing.T) {
	r := goodReport()
	require.NoError(t, r.Validate())
	r.Runs[0].Correct = false
	require.Error(t, r.Validate())
	r = goodReport()
	r.Runs[0].ElapsedNS = 0
	require.Error(t, r.Validate())
	r = goodReport()
	r.Runs = r.Runs[1:]
	require.Error(t, r.Validate())
}
func TestResultsContainEveryRepetition(t *testing.T) {
	d := t.TempDir()
	r := goodReport()
	require.NoError(t, WriteReport(d, r))
	raw, e := os.ReadFile(filepath.Join(d, "database.csv"))
	require.NoError(t, e)
	require.Len(t, strings.Split(strings.TrimSpace(string(raw)), "\n"), 10)
	require.Len(t, r.Summaries(), 3)
}
