package config

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRuntimeConfigValidation(t *testing.T) {
	good := map[string]string{"API_KEYS": `{"key":"demo"}`, "DATABASE_URL": "postgres://user:pass@localhost/test"}
	get := func(k string) string { return good[k] }
	a, err := LoadAPIEnv(get)
	require.NoError(t, err)
	require.Equal(t, ":8080", a.ListenAddress)
	require.Equal(t, "demo", a.APIKeys["key"])
	c, err := LoadConsumerEnv(get)
	require.NoError(t, err)
	require.NotEmpty(t, c.GroupID)
	delete(good, "API_KEYS")
	_, err = LoadAPIEnv(get)
	require.Error(t, err)
	good["API_KEYS"] = "bad"
	_, err = LoadAPIEnv(get)
	require.Error(t, err)
	delete(good, "DATABASE_URL")
	_, err = LoadConsumerEnv(get)
	require.Error(t, err)
	good["API_KEYS"] = `{"key":"demo"}`
	good["RATE_LIMIT_RPS"] = "0"
	_, err = LoadAPIEnv(get)
	require.Error(t, err)
}

func TestRuntimeRejectsDecodedDuplicateAPIKeys(t *testing.T) {
	for _, raw := range []string{
		`{"private-canary-key":"tenant-a","private-canary-key":"tenant-b"}`,
		`{"private-canary-key":"tenant-a","private\u002dcanary-key":"tenant-b"}`,
	} {
		t.Run(raw, func(t *testing.T) {
			_, err := LoadAPIEnv(func(k string) string {
				if k == "API_KEYS" {
					return raw
				}
				return ""
			})
			require.Error(t, err)
			require.NotContains(t, err.Error(), "private-canary-key")
		})
	}
	a, err := LoadAPIEnv(func(k string) string {
		if k == "API_KEYS" {
			return `{"old-key":"tenant-a","new-key":"tenant-a"}`
		}
		return ""
	})
	require.NoError(t, err)
	require.Len(t, a.APIKeys, 2)
}
