package benchmark

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"math"
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

func TestWorkloadNamespacesAndVariedPayloads(t *testing.T) {
	c := WorkloadConfig{Namespace: "study/case-a", PayloadProfile: "varied", Seed: 7, Count: 20, PayloadBytes: 1024, DuplicateFraction: .25}
	a, e := Workload(c)
	require.NoError(t, e)
	b, e := Workload(c)
	require.NoError(t, e)
	require.Equal(t, a, b)
	distinct := map[byte]bool{}
	for _, v := range a {
		require.Len(t, v.Payload, 1024)
		for _, ch := range v.Payload {
			distinct[ch] = true
		}
	}
	require.Greater(t, len(distinct), 40)
	require.Equal(t, a[0], a[15])
	c.Namespace = "study/case-b"
	b, e = Workload(c)
	require.NoError(t, e)
	require.NotEqual(t, a[0].EventID, b[0].EventID)
	require.NotEqual(t, a[0].EventType, b[0].EventType)
	c.PayloadProfile = "unknown"
	_, e = Workload(c)
	require.Error(t, e)
}
func TestWorkloadRejectsNonFiniteFractions(t *testing.T) {
	for _, fraction := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		require.NotPanics(t, func() {
			_, e := Workload(WorkloadConfig{Count: 10, PayloadBytes: 256, DuplicateFraction: fraction})
			require.Error(t, e)
		})
	}
}
