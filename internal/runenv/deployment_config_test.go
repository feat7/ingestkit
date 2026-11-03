package runenv

import (
	"bytes"
	"github.com/stretchr/testify/require"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func configure(t *testing.T, host, dir string) ([]byte, error) {
	t.Helper()
	root, e := Root()
	require.NoError(t, e)
	cmd := exec.Command("sh", filepath.Join(root, "deploy/configure.sh"), host, dir)
	return cmd.CombinedOutput()
}
func TestConfigureCreatesPrivateUniqueSecrets(t *testing.T) {
	seen := map[string]bool{}
	for _, domain := range []string{"events.example.com", "localhost"} {
		dir := filepath.Join(t.TempDir(), "settings")
		out, e := configure(t, domain, dir)
		require.NoError(t, e, string(out))
		path := filepath.Join(dir, ".env")
		cfg, e := ReadDeploymentConfig(path)
		require.NoError(t, e)
		require.Equal(t, domain, cfg.Domain)
		stat, e := os.Stat(path)
		require.NoError(t, e)
		require.EqualValues(t, 0600, stat.Mode().Perm())
		require.Len(t, cfg.APIKeys, 1)
		secrets := []string{cfg.AdminPassword, cfg.WriterPassword}
		for key, tenant := range cfg.APIKeys {
			secrets = append(secrets, key)
			require.Equal(t, "default", tenant)
		}
		for _, secret := range secrets {
			require.Regexp(t, "^[0-9a-f]{64}$", secret)
			require.False(t, seen[secret], "secrets must be independently generated")
			seen[secret] = true
			require.False(t, strings.Contains(string(out), secret), "script printed a secret")
		}
	}
}
func TestConfigureRejectsUnsafeHostname(t *testing.T) {
	for _, host := range []string{"", "https://events.example.com", "events.example.com:443", "bad domain", "bad\nDOMAIN=other", "$(touch injected)", "-bad.example.com", "bad-.example.com", "a..example.com", strings.Repeat("a", 64) + ".com"} {
		dir := t.TempDir()
		marker := filepath.Join(dir, "injected")
		if strings.Contains(host, "$(") {
			host = "$(touch " + marker + ")"
		}
		out, e := configure(t, host, dir)
		require.NoFileExists(t, marker)
		require.Error(t, e)
		require.NoFileExists(t, filepath.Join(dir, ".env"))
		require.Contains(t, strings.ToLower(string(out)), "hostname")
	}
}
func TestConfigureNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	_, e := configure(t, "events.example.com", dir)
	require.NoError(t, e)
	path := filepath.Join(dir, ".env")
	before, e := os.ReadFile(path)
	require.NoError(t, e)
	_, e = configure(t, "other.example.com", dir)
	require.Error(t, e)
	after, e := os.ReadFile(path)
	require.NoError(t, e)
	require.True(t, bytes.Equal(before, after), "existing configuration changed")
}
func TestConfigureGenerationFailureLeavesNoConfig(t *testing.T) {
	fake := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(fake, "openssl"), []byte("#!/bin/sh\nexit 1\n"), 0700))
	t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))
	dir := t.TempDir()
	_, e := configure(t, "events.example.com", dir)
	require.Error(t, e)
	require.NoFileExists(t, filepath.Join(dir, ".env"))
}
func deploymentFixture() string {
	return "DOMAIN=events.example.com\nAPI_KEYS={\"" + strings.Repeat("a", 64) + "\":\"default\"}\nPOSTGRES_ADMIN_PASSWORD=" + strings.Repeat("b", 64) + "\nPOSTGRES_WRITER_PASSWORD=" + strings.Repeat("c", 64) + "\n"
}
func TestDeploymentConfigRejectsMissingOrDuplicateFields(t *testing.T) {
	raw := deploymentFixture()
	for _, bad := range []string{
		strings.Replace(raw, "DOMAIN=events.example.com\n", "", 1),
		strings.Replace(raw, "events.example.com", "$(touch injected)", 1),
		raw + "DOMAIN=other.example.com\n",
		strings.Replace(raw, strings.Repeat("b", 64), "change-me", 1),
		strings.Replace(raw, strings.Repeat("c", 64), "", 1),
		strings.Replace(raw, "API_KEYS={", "API_KEYS=invalid", 1),
		strings.Replace(raw, `"default"`, `"bad\u0000tenant"`, 1),
		strings.Replace(raw, `"default"}`, `"default","`+strings.Repeat("a", 64)+`":"other"}`, 1),
	} {
		path := filepath.Join(t.TempDir(), ".env")
		require.NoError(t, os.WriteFile(path, []byte(bad), 0600))
		_, e := ReadDeploymentConfig(path)
		require.Error(t, e)
	}
	path := filepath.Join(t.TempDir(), ".env")
	require.NoError(t, os.WriteFile(path, []byte(raw), 0644))
	_, e := ReadDeploymentConfig(path)
	require.Error(t, e, "readable secrets must be rejected")
}
func TestDeploymentConfigSupportsOverlappingKeys(t *testing.T) {
	raw := strings.Replace(deploymentFixture(), `"default"}`, `"default","`+strings.Repeat("d", 64)+`":"default"}`, 1)
	path := filepath.Join(t.TempDir(), ".env")
	require.NoError(t, os.WriteFile(path, []byte(raw), 0600))
	cfg, e := ReadDeploymentConfig(path)
	require.NoError(t, e)
	require.Len(t, cfg.APIKeys, 2)
}
