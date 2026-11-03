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
