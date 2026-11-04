package runtimeinfo

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProcessLimitFixtureRecordsKnownAndUnlimitedValues(t *testing.T) {
	p := filepath.Join(t.TempDir(), "ports")
	require.NoError(t, os.WriteFile(p, []byte("49152 65535\n"), 0600))
	r := captureLimits(func() (uint64, uint64, uint64, error) { return 128, ^uint64(0), ^uint64(0), nil }, func() (int, error) { return 7, nil }, func() ([]byte, error) { return os.ReadFile(p) })
	require.Equal(t, "known", r.SoftFDLimit.Status)
	require.EqualValues(t, 128, *r.SoftFDLimit.Value)
	require.Equal(t, "unlimited", r.HardFDLimit.Status)
	require.Nil(t, r.HardFDLimit.Value)
	require.EqualValues(t, 7, *r.OpenFDs.Value)
	require.EqualValues(t, 49152, *r.EphemeralFirst.Value)
	require.EqualValues(t, 65535, *r.EphemeralLast.Value)
}

func TestUnavailableLimitsSerializeNullNotZero(t *testing.T) {
	r := captureLimits(func() (uint64, uint64, uint64, error) { return 0, 0, 0, fs.ErrPermission }, func() (int, error) { return 0, fs.ErrPermission }, func() ([]byte, error) { return nil, fs.ErrPermission })
	raw, e := json.Marshal(r)
	require.NoError(t, e)
	var result map[string]map[string]any
	require.NoError(t, json.Unmarshal(raw, &result))
	for key, value := range result {
		require.Equal(t, "unavailable", value["status"], key)
		require.Nil(t, value["value"], key)
	}
}

func TestMalformedPortRangeIsUnavailable(t *testing.T) {
	for _, value := range []string{"", "0 123", "60000 10000", "12 34 extra", "private-error-text"} {
		r := captureLimits(func() (uint64, uint64, uint64, error) { return 10, 20, 999, nil }, func() (int, error) { return 2, nil }, func() ([]byte, error) { return []byte(value), nil })
		require.Equal(t, "unavailable", r.EphemeralFirst.Status)
		require.Nil(t, r.EphemeralFirst.Value)
		raw, e := json.Marshal(r)
		require.NoError(t, e)
		require.NotContains(t, string(raw), "private-error-text")
	}
}
