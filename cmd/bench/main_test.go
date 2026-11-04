package main

import (
	"context"
	"encoding/json"
	"github.com/feat7/ingestkit/internal/benchmark"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMeasuredModesUseDurableOwnedProfile(t *testing.T) {
	for _, mode := range []string{"pipeline", "database"} {
		o := measuredOptions(mode)
		require.Equal(t, "selfhost", o.Profile)
		require.True(t, o.ExposeDependencies)
	}
}
func TestCLIRejectsInvalidFlagsBeforeLaunching(t *testing.T) {
	for _, args := range [][]string{{"-unknown"}, {"-mode", "study", "-suite", "wrong"}, {"-suite", "connections"}, {"-mode", "pipeline", "-rate", "50000", "-duration", "60s"}, {"-mode", "pipeline", "-payload", "16384", "-batch", "100"}} {
		require.Error(t, runArgs(args))
	}
}

func TestInterruptedCalibrationPreservesCompletedAndPartialEvidence(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	report, runErr := benchmark.RunCalibration(ctx, benchmark.PipelineConfig{Rate: 10, Duration: time.Second, Concurrency: 2, BatchSize: 1, PayloadBytes: 64, Seed: 4, Repetitions: 3, HTTPMaxConnsPerHost: 2, HTTPMaxIdleConnsPerHost: 2})
	require.ErrorIs(t, runErr, context.DeadlineExceeded)
	require.GreaterOrEqual(t, len(report.Runs), 2)
	require.Positive(t, report.Runs[0].Accepted)
	require.Positive(t, report.Runs[1].Accepted)
	out := t.TempDir()
	require.ErrorIs(t, writeCalibrationResult(out, report, runErr), context.DeadlineExceeded)
	raw, err := os.ReadFile(filepath.Join(out, "calibration.json"))
	require.NoError(t, err)
	var saved struct {
		Status string `json:"status"`
		Runs   []benchmark.CalibrationRun
	}
	require.NoError(t, json.Unmarshal(raw, &saved))
	require.Equal(t, "incomplete", saved.Status)
	require.Equal(t, report.Runs, saved.Runs)
}
