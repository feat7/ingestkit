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

func TestRuntimeResourceSettingsRequireActualFlags(t *testing.T) {
	settings, e := brokerResources("redpanda\x00--smp=1\x00--memory\x001G\x00")
	require.NoError(t, e)
	require.Equal(t, 1, settings["redpanda_smp"])
	require.Equal(t, "1G", settings["redpanda_memory"])
	for _, raw := range []string{"redpanda\x00", "redpanda\x00--smp=1\x00--smp=2\x00--memory=1G\x00"} {
		_, e = brokerResources(raw)
		require.Error(t, e)
	}
}
func TestRuntimeAPISettingsAreAllowlisted(t *testing.T) {
	raw, e := json.Marshal([]string{"API_KEYS={\"private-api-key\":\"default\"}", "DATABASE_URL=postgres://private:password@host/db", "RATE_LIMIT_RPS=321", "HTTP_ADDR=:8080", "LOG_LEVEL=debug"})
	require.NoError(t, e)
	settings, e := safeAPISettings(string(raw))
	require.NoError(t, e)
	require.Equal(t, 321, settings["api_rate_limit_rps"])
	encoded, e := json.Marshal(settings)
	require.NoError(t, e)
	for _, secret := range []string{"private-api-key", "private:password", "postgres://", "API_KEYS"} {
		require.NotContains(t, string(encoded), secret)
	}
	_, e = safeAPISettings(`["RATE_LIMIT_RPS=private-api-key"]`)
	require.Error(t, e)
	require.NotContains(t, e.Error(), "private-api-key")
}
