package benchmark

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestWorkloadSeedReproducesIDsAndBytes(t *testing.T) {
	c := WorkloadConfig{Seed: 42, Count: 20, PayloadBytes: 256, DuplicateFraction: .25}
	a, e := Workload(c)
	require.NoError(t, e)
	b, e := Workload(c)
	require.NoError(t, e)
	require.Equal(t, a, b)
	require.Len(t, a[0].Payload, 256)
	require.True(t, json.Valid(a[0].Payload))
	c.Seed++
	b, e = Workload(c)
	require.NoError(t, e)
	require.NotEqual(t, a[0].EventID, b[0].EventID)
	for _, n := range []int{0, -1} {
		c.Count = n
		_, e = Workload(c)
		require.Error(t, e)
	}
}
func TestDuplicateWorkloadHasMatchingSemantics(t *testing.T) {
	es, e := Workload(WorkloadConfig{Count: 20, Seed: 1, PayloadBytes: 128, DuplicateFraction: .25})
	require.NoError(t, e)
	require.Len(t, expectedEvents(es), 15)
	require.Equal(t, es[0], es[15])
}
func TestTimingExcludesSetup(t *testing.T) {
	now := time.Unix(0, 0)
	setup := func() error { now = now.Add(time.Hour); return nil }
	work := func() (int64, error) { now = now.Add(2 * time.Second); return 3, nil }
	elapsed, n, e := measure(setup, work, func() time.Time { return now })
	require.NoError(t, e)
	require.Equal(t, 2*time.Second, elapsed)
	require.EqualValues(t, 3, n)
}
