package benchmark

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestManifestRedactsSecrets(t *testing.T) {
	raw, e := json.Marshal(PublicPipelineConfig(PipelineConfig{APIURL: "http://localhost:1", APIKey: "secret-token", Tenant: "demo"}))
	require.NoError(t, e)
	require.NotContains(t, string(raw), "secret-token")
	require.NotContains(t, string(raw), "postgres://")
}

func TestBrokerFsyncStateRequiresAnExplicitRuntimeFlag(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want bool
		bad  bool
	}{
		{"redpanda\x00--unsafe-bypass-fsync=true\x00", true, false},
		{"redpanda\x00--unsafe-bypass-fsync=false\x00", false, false},
		{"redpanda\x00--unsafe-bypass-fsync\x000\x00", false, false},
		{"redpanda\x00--unsafe-bypass-fsync=maybe\x00", false, true},
		{"redpanda\x00", false, true},
		{"redpanda\x00--unsafe-bypass-fsync=false\x00--unsafe-bypass-fsync=true\x00", false, true},
	} {
		got, e := brokerFsyncBypass(tc.raw)
		if tc.bad {
			require.Error(t, e)
		} else {
			require.NoError(t, e)
			require.Equal(t, tc.want, got)
		}
	}
}
