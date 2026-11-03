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
