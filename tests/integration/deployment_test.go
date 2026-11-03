//go:build integration

package integration

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/feat7/ingestkit/internal/event"
	"github.com/feat7/ingestkit/internal/persistence"
	"github.com/feat7/ingestkit/internal/runenv"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func deployment(t *testing.T, probes bool) (*runenv.Stack, context.Context) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	s, err := runenv.Start(ctx, runenv.Options{Purpose: "selfhost-test", Profile: "selfhost", ExposeDependencies: probes})
	require.NoError(t, err)
	t.Cleanup(func() {
		evidence, finishEvidence := context.WithTimeout(context.Background(), 10*time.Second)
		evidenceErr := s.CollectEvidence(evidence, s.Directory)
		finishEvidence()
		cleanup, finishCleanup := context.WithTimeout(context.Background(), 30*time.Second)
		cleanupErr := s.Destroy(cleanup)
		finishCleanup()
		// Evidence failure must not skip cleanup or consume its time budget.
		require.NoError(t, evidenceErr)
		require.NoError(t, cleanupErr)
	})
	return s, ctx
}
func deploymentPool(t *testing.T, ctx context.Context, s *runenv.Stack) *pgxpool.Pool {
	t.Helper()
	p, e := pgxpool.New(ctx, s.DatabaseURL)
	require.NoError(t, e)
	t.Cleanup(p.Close)
	return p
}
func deploymentSend(t *testing.T, s *runenv.Stack, path, key string, body any) int {
	t.Helper()
	raw, e := json.Marshal(body)
	require.NoError(t, e)
	req, e := http.NewRequest("POST", s.APIURL+path, bytes.NewReader(raw))
	require.NoError(t, e)
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	client, e := s.HTTPClient()
	require.NoError(t, e)
	res, e := client.Do(req)
	require.NoError(t, e)
	defer res.Body.Close()
	return res.StatusCode
}
func TestSelfhostTLSFirstEventAndDuplicate(t *testing.T) {
	s, ctx := deployment(t, true)
	p := deploymentPool(t, ctx, s)
	rawArgs, err := s.Compose(ctx, "exec", "-T", "redpanda", "sh", "-c", `for p in /proc/[0-9]*; do case "$(readlink "$p/exe" 2>/dev/null)" in */redpanda) cat "$p/cmdline"; exit 0;; esac; done; exit 1`)
	require.NoError(t, err)
	found := false
	args := strings.Split(rawArgs, "\x00")
	for i, arg := range args {
		if arg == "--unsafe-bypass-fsync=false" || arg == "--unsafe-bypass-fsync=0" {
			found = true
		}
		if arg == "--unsafe-bypass-fsync" {
			require.Less(t, i+1, len(args))
			require.Contains(t, []string{"false", "0"}, args[i+1])
			found = true
		}
	}
	require.True(t, found, "running broker must explicitly retain fsync")
	for _, setting := range []string{"fsync", "synchronous_commit"} {
		var value string
		require.NoError(t, p.QueryRow(ctx, "SHOW "+setting).Scan(&value))
		require.Equal(t, "on", value)
	}

	_, err = http.Get(s.APIURL + "/health")
	require.Error(t, err, "private CA must not be implicitly trusted")
	client, err := s.HTTPClient()
	require.NoError(t, err)
	for _, path := range []string{"/health", "/ready", "/metrics", "/admin", "/config/"} {
		res, e := client.Get(s.APIURL + path)
		require.NoError(t, e)
		res.Body.Close()
		want := 404
		if path == "/health" || path == "/ready" {
			want = 200
		}
		require.Equal(t, want, res.StatusCode, path)
	}
	a, b := freshEvent(), freshEvent()
	require.Equal(t, 202, deploymentSend(t, s, "/v1/events", s.APIKey(), a))
	require.Equal(t, 202, deploymentSend(t, s, "/v1/events/batch", s.APIKey(), map[string]any{"events": []event.Input{a, b}}))
	require.NoError(t, waitForIDs(ctx, p, "default", []uuid.UUID{a.EventID, b.EventID}))
	var n int
	require.NoError(t, p.QueryRow(ctx, "SELECT count(*) FROM ingestkit.events WHERE event_id=$1", a.EventID.String()).Scan(&n))
	require.Equal(t, 1, n)
}
func TestSelfhostPublishesOnlyGateway(t *testing.T) {
	root, e := runenv.Root()
	require.NoError(t, e)
	dir := t.TempDir()
	cmd := exec.Command("sh", filepath.Join(root, "deploy/configure.sh"), "events.example.com", dir)
	require.NoError(t, cmd.Run())
	cmd = exec.Command("docker", "compose", "--env-file", filepath.Join(dir, ".env"), "-f", filepath.Join(root, "deploy/compose.yml"), "config", "--format", "json")
	raw, e := cmd.Output()
	require.NoError(t, e)
	var config struct {
		Services map[string]struct{ Ports []struct{ Target int } }
	}
	require.NoError(t, json.Unmarshal(raw, &config))
	require.Len(t, config.Services["gateway"].Ports, 2)
	for name, service := range config.Services {
		if name != "gateway" {
			require.Empty(t, service.Ports, name)
		}
	}

	s, ctx := deployment(t, false)
	ids, err := s.Compose(ctx, "ps", "-q")
	require.NoError(t, err)
	for _, id := range strings.Fields(ids) {
		cmd := exec.CommandContext(ctx, "docker", "inspect", "--format", `{{json .NetworkSettings.Ports}}`, id)
		out, e := cmd.Output()
		require.NoError(t, e)
		var ports map[string][]struct{ HostIP, HostPort string }
		require.NoError(t, json.Unmarshal(out, &ports))
		for port, bindings := range ports {
			if len(bindings) > 0 {
				require.Contains(t, []string{"80/tcp", "443/tcp"}, port)
				for _, b := range bindings {
					require.Equal(t, "127.0.0.1", b.HostIP)
				}
			}
		}
	}
	require.Empty(t, s.DatabaseURL)
	require.Empty(t, s.Brokers)
	require.Empty(t, s.ConsumerURL)
}
func TestSelfhostWriterPrivileges(t *testing.T) {
	s, ctx := deployment(t, true)
	cfg, e := runenv.ReadDeploymentConfig(filepath.Join(s.Directory, "run.env"))
	require.NoError(t, e)
	u, e := url.Parse(s.DatabaseURL)
	require.NoError(t, e)
	u.User = url.UserPassword("ingestkit_writer", cfg.WriterPassword)
	p, e := pgxpool.New(ctx, u.String())
	require.NoError(t, e)
	defer p.Close()
	var super bool
	require.NoError(t, p.QueryRow(ctx, "SELECT rolsuper FROM pg_roles WHERE rolname=current_user").Scan(&super))
	require.False(t, super)
	_, e = p.Exec(ctx, "CREATE TABLE ingestkit.forbidden(id int)")
	require.Error(t, e)
	_, e = p.Exec(ctx, "DROP TABLE ingestkit.events")
	require.Error(t, e)
	input := freshEvent()
	_, e = persistence.NewEventStore(p).WriteBatch(ctx, []event.Envelope{{Input: input, TenantID: "default", ReceivedAt: time.Now().UTC()}})
	require.NoError(t, e)
}
func TestSelfhostRestartPreservesIDs(t *testing.T) {
	s, ctx := deployment(t, true)
	p := deploymentPool(t, ctx, s)
	a := freshEvent()
	require.Equal(t, 202, deploymentSend(t, s, "/v1/events", s.APIKey(), a))
	require.NoError(t, waitForIDs(ctx, p, "default", []uuid.UUID{a.EventID}))
	require.NoError(t, s.Stop(ctx))
	_, e := s.Compose(ctx, "up", "-d", "--wait", "--wait-timeout", "90")
	require.NoError(t, e)
	require.Equal(t, 202, deploymentSend(t, s, "/v1/events", s.APIKey(), a))
	require.NoError(t, waitForIDs(ctx, p, "default", []uuid.UUID{a.EventID}))
	var n int
	require.NoError(t, p.QueryRow(ctx, "SELECT count(*) FROM public.ingestkit_schema_migrations").Scan(&n))
	require.Equal(t, 1, n)
}
func TestSelfhostMissingConfigFails(t *testing.T) {
	root, e := runenv.Root()
	require.NoError(t, e)
	empty := filepath.Join(t.TempDir(), "empty.env")
	require.NoError(t, os.WriteFile(empty, nil, 0600))
	cmd := exec.Command("docker", "compose", "--env-file", empty, "-f", filepath.Join(root, "deploy/compose.yml"), "config", "--quiet")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME")}
	out, e := cmd.CombinedOutput()
	require.Error(t, e)
	require.Contains(t, string(out), "required")
}

// This builds the real context; checking .dockerignore text would miss pattern errors.
func TestDeploymentBuildContextExcludesNestedSecrets(t *testing.T) {
	root, e := runenv.Root()
	require.NoError(t, e)
	name := "context-canary-" + uuid.NewString()
	dir := filepath.Join(root, "internal", name)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "nested/private"), 0700))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "nested/backups"), 0700))
	defer os.RemoveAll(dir)
	for _, path := range []string{"nested/.env", "nested/.env.production", "nested/private/key", "nested/backups/dump", "keep.txt"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, path), []byte("context-canary"), 0600))
	}
	image := "ingestkit-" + name
	cmd := exec.Command("docker", "build", "-q", "-t", image, "-f", "-", root)
	cmd.Stdin = strings.NewReader("FROM scratch\nCOPY . /context\nCMD [\"unused\"]\n")
	out, e := cmd.CombinedOutput()
	require.NoError(t, e, string(out))
	defer exec.Command("docker", "image", "rm", image).Run()
	out, e = exec.Command("docker", "create", image).Output()
	require.NoError(t, e)
	id := strings.TrimSpace(string(out))
	defer exec.Command("docker", "rm", id).Run()
	cmd = exec.Command("docker", "export", id)
	reader, e := cmd.StdoutPipe()
	require.NoError(t, e)
	require.NoError(t, cmd.Start())
	tr := tar.NewReader(reader)
	found := false
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		require.NoError(t, e)
		if strings.Contains(h.Name, name+"/") {
			require.NotContains(t, h.Name, ".env")
			require.NotContains(t, h.Name, "/private/")
			require.NotContains(t, h.Name, "/backups/")
			if strings.HasSuffix(h.Name, "/keep.txt") {
				found = true
			}
		}
	}
	require.NoError(t, cmd.Wait())
	require.True(t, found, "positive context control must be copied")
}

func TestWriteDeniedMakesDeploymentUnready(t *testing.T) {
	s, ctx := deployment(t, true)
	p := deploymentPool(t, ctx, s)
	_, e := p.Exec(ctx, "REVOKE INSERT ON ingestkit.events FROM ingestkit_writer")
	require.NoError(t, e)
	t.Cleanup(func() {
		c, end := context.WithTimeout(context.Background(), 5*time.Second)
		defer end()
		_, _ = p.Exec(c, "GRANT INSERT ON ingestkit.events TO ingestkit_writer")
	})
	a := freshEvent()
	a.Payload = json.RawMessage(`{"private":"payload-secret-canary"}`)
	require.Equal(t, 202, deploymentSend(t, s, "/v1/events", s.APIKey(), a))
	require.NoError(t, waitStatus(ctx, s.ConsumerURL+"/ready", 503))
	cfg, e := runenv.ReadDeploymentConfig(filepath.Join(s.Directory, "run.env"))
	require.NoError(t, e)
	u, e := url.Parse(s.DatabaseURL)
	require.NoError(t, e)
	u.User = url.UserPassword("ingestkit_writer", cfg.WriterPassword)
	writer, e := pgxpool.New(ctx, u.String())
	require.NoError(t, e)
	defer writer.Close()
	require.NoError(t, writer.Ping(ctx))
	var n int
	require.NoError(t, p.QueryRow(ctx, "SELECT count(*) FROM ingestkit.events WHERE event_id=$1", a.EventID.String()).Scan(&n))
	require.Zero(t, n)
	logs, e := s.Compose(ctx, "logs", "--no-color", "consumer")
	require.NoError(t, e)
	require.Contains(t, logs, `"sqlstate":"42501"`)
	for _, secret := range []string{s.APIKey(), cfg.WriterPassword, cfg.AdminPassword, "payload-secret-canary"} {
		require.NotContains(t, logs, secret)
	}
	_, e = p.Exec(ctx, "GRANT INSERT ON ingestkit.events TO ingestkit_writer")
	require.NoError(t, e)
	require.NoError(t, waitForIDs(ctx, p, "default", []uuid.UUID{a.EventID}))
	require.NoError(t, waitStatus(ctx, s.ConsumerURL+"/ready", 200))
}
func replaceDeploymentKeys(t *testing.T, s *runenv.Stack, keys map[string]string) {
	t.Helper()
	path := filepath.Join(s.Directory, "run.env")
	raw, e := os.ReadFile(path)
	require.NoError(t, e)
	encoded, e := json.Marshal(keys)
	require.NoError(t, e)
	lines := strings.Split(string(raw), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "API_KEYS=") {
			lines[i] = "API_KEYS=" + string(encoded)
		}
	}
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0600))
}
func TestCredentialOverlapAndRefresh(t *testing.T) {
	s, ctx := deployment(t, true)
	p := deploymentPool(t, ctx, s)
	old := s.APIKey()
	newKey := strings.ReplaceAll(uuid.NewString()+uuid.NewString(), "-", "")
	a, b := freshEvent(), freshEvent()
	require.Equal(t, 202, deploymentSend(t, s, "/v1/events", old, a))
	replaceDeploymentKeys(t, s, map[string]string{old: "default", newKey: "default"})
	_, e := s.Compose(ctx, "up", "-d", "--no-deps", "--force-recreate", "--wait", "api")
	require.NoError(t, e)
	require.Equal(t, 202, deploymentSend(t, s, "/v1/events", old, a))
	require.Equal(t, 202, deploymentSend(t, s, "/v1/events", newKey, b))
	replaceDeploymentKeys(t, s, map[string]string{newKey: "default"})
	_, e = s.Compose(ctx, "up", "-d", "--no-deps", "--force-recreate", "--wait", "api")
	require.NoError(t, e)
	require.Equal(t, 401, deploymentSend(t, s, "/v1/events", old, a))
	require.Equal(t, 202, deploymentSend(t, s, "/v1/events", newKey, b))
	require.NoError(t, waitForIDs(ctx, p, "default", []uuid.UUID{a.EventID, b.EventID}))
}
func TestBackupRestoresIntoOwnedReplacement(t *testing.T) {
	source, ctx := deployment(t, true)
	sourcePool := deploymentPool(t, ctx, source)
	a := freshEvent()
	require.Equal(t, 202, deploymentSend(t, source, "/v1/events", source.APIKey(), a))
	require.NoError(t, waitForIDs(ctx, sourcePool, "default", []uuid.UUID{a.EventID}))
	dump, e := source.Compose(ctx, "exec", "-T", "postgres", "pg_dump", "-U", "ingestkit_admin", "-d", "ingestkit", "-Fc")
	require.NoError(t, e)
	require.True(t, strings.HasPrefix(dump, "PGDMP"))
	replacement, replacementCtx := deployment(t, true)
	p := deploymentPool(t, replacementCtx, replacement)
	require.NotEqual(t, source.Project, replacement.Project)
	require.NoError(t, replacement.SetServiceRunning(replacementCtx, "consumer", false))
	var n int
	require.NoError(t, p.QueryRow(replacementCtx, "SELECT count(*) FROM ingestkit.events").Scan(&n))
	require.Zero(t, n, "restore target must be an owned empty replacement")
	// Recreate only this test-owned empty database so the documented restore is
	// exercised without --clean or a pre-existing migration/table scaffold.
	p.Close()
	_, e = replacement.Compose(replacementCtx, "exec", "-T", "postgres", "psql", "-U", "ingestkit_admin", "-d", "postgres", "-v", "ON_ERROR_STOP=1", "-c", "DROP DATABASE ingestkit WITH (FORCE)")
	require.NoError(t, e)
	_, e = replacement.Compose(replacementCtx, "exec", "-T", "postgres", "psql", "-U", "ingestkit_admin", "-d", "postgres", "-v", "ON_ERROR_STOP=1", "-c", "CREATE DATABASE ingestkit OWNER ingestkit_admin")
	require.NoError(t, e)
	p = deploymentPool(t, replacementCtx, replacement)

	path := filepath.Join(t.TempDir(), "snapshot.dump")
	require.NoError(t, os.WriteFile(path, []byte(dump), 0600))
	_, e = replacement.Compose(replacementCtx, "cp", path, "postgres:/tmp/snapshot.dump")
	require.NoError(t, e)
	_, e = replacement.Compose(replacementCtx, "exec", "-T", "postgres", "pg_restore", "-U", "ingestkit_admin", "-d", "ingestkit", "--exit-on-error", "/tmp/snapshot.dump")
	require.NoError(t, e)
	require.NoError(t, replacement.SetServiceRunning(replacementCtx, "consumer", true))
	b := freshEvent()
	require.Equal(t, 202, deploymentSend(t, replacement, "/v1/events/batch", replacement.APIKey(), map[string]any{"events": []event.Input{a, b}}))
	require.NoError(t, waitForIDs(replacementCtx, p, "default", []uuid.UUID{a.EventID, b.EventID}))
	var payload string
	require.NoError(t, p.QueryRow(replacementCtx, "SELECT payload->>'amount' FROM ingestkit.events WHERE event_id=$1", a.EventID.String()).Scan(&payload))
	require.Equal(t, "25", payload)
	require.NoError(t, waitForIDs(ctx, sourcePool, "default", []uuid.UUID{a.EventID}))
	require.NoError(t, sourcePool.QueryRow(ctx, "SELECT count(*) FROM ingestkit.events WHERE event_id=$1", b.EventID.String()).Scan(&n))
	require.Zero(t, n)
}

func TestReusableClientsThroughHTTPS(t *testing.T) {
	s, ctx := deployment(t, true)
	p := deploymentPool(t, ctx, s)
	root, e := runenv.Root()
	require.NoError(t, e)
	events := []event.Input{freshEvent(), freshEvent(), freshEvent()}
	raw, e := json.Marshal(map[string]any{"events": events})
	require.NoError(t, e)
	fixture := filepath.Join(t.TempDir(), "events.json")
	require.NoError(t, os.WriteFile(fixture, raw, 0600))
	baseEnv := append(os.Environ(), "API_URL="+s.APIURL, "INGESTKIT_API_KEY="+s.APIKey(), "INGESTKIT_CA_FILE="+filepath.Join(s.Directory, "root.crt"), "EVENTS_FILE="+fixture)
	for _, language := range []struct{ binary, path string }{{"python3", "examples/checkout/send.py"}, {"node", "examples/checkout/send.mjs"}} {
		for _, mode := range []string{"single", "batch"} {
			cmd := exec.CommandContext(ctx, language.binary, language.path)
			cmd.Dir = root
			cmd.Env = append(baseEnv, "SEND_MODE="+mode)
			out, e := cmd.CombinedOutput()
			require.NoError(t, e, string(out))
			require.Contains(t, string(out), "accepted")
		}
	}
	// Verify the documented Python package import as well as direct-file CLIs.
	cmd := exec.CommandContext(ctx, "python3", "-c", `import json,os; from examples.checkout.ingest_client import Client; c=Client(os.environ['API_URL'],os.environ['INGESTKIT_API_KEY'],ca_file=os.environ['INGESTKIT_CA_FILE']); c.send(json.load(open(os.environ['EVENTS_FILE']))['events'][0])`)
	cmd.Dir = root
	cmd.Env = baseEnv
	out, e := cmd.CombinedOutput()
	require.NoError(t, e, string(out))
	ids := []uuid.UUID{}
	for _, event := range events {
		ids = append(ids, event.EventID)
	}
	require.NoError(t, waitForIDs(ctx, p, "default", ids))
}
