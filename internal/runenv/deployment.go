package runenv

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func (s *Stack) APIKey() string {
	if s.profile != "selfhost" {
		return "local-demo-key"
	}
	cfg, err := ReadDeploymentConfig(filepath.Join(s.Directory, "run.env"))
	if err != nil {
		return ""
	}
	for key, tenant := range cfg.APIKeys {
		if tenant == "default" {
			return key
		}
	}
	return ""
}
func (s *Stack) HTTPClient() (*http.Client, error) {
	client := &http.Client{Timeout: 8 * time.Second}
	if s.profile != "selfhost" {
		return client, nil
	}
	raw, e := os.ReadFile(s.caPath)
	if e != nil {
		return nil, errors.New("cannot read deployment CA")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(raw) {
		return nil, errors.New("invalid deployment CA")
	}
	client.Transport = &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}
	return client, nil
}
func (s *Stack) prepareDeployment(ctx context.Context, o Options) error {
	s.profile = "selfhost"
	s.exposeDependencies = o.ExposeDependencies
	cmd := exec.CommandContext(ctx, "sh", filepath.Join(s.root, "deploy/configure.sh"), "localhost", s.Directory)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("configure owned deployment: %w: %s", err, out)
	}
	cfg, err := os.ReadFile(filepath.Join(s.Directory, ".env"))
	if err != nil {
		return err
	}
	var reservations []net.Listener
	defer func() {
		for _, l := range reservations {
			l.Close()
		}
	}()
	ports := make([]int, 5)
	for i := range ports {
		l, e := net.Listen("tcp", "127.0.0.1:0")
		if e != nil {
			return e
		}
		reservations = append(reservations, l)
		ports[i] = l.Addr().(*net.TCPAddr).Port
	}
	s.env = []string{fmt.Sprintf("INGESTKIT_HTTP_PORT=%d", ports[0]), fmt.Sprintf("INGESTKIT_HTTPS_PORT=%d", ports[1]), fmt.Sprintf("POSTGRES_PORT=%d", ports[2]), fmt.Sprintf("METRICS_PORT=%d", ports[3]), fmt.Sprintf("INGESTKIT_BROKER_PORT=%d", ports[4]), "INGESTKIT_REPO_ROOT=" + s.root, "INGESTKIT_RUN_ID=" + s.token, "RATE_LIMIT_RPS=10000", "LOG_LEVEL=info", "RETENTION_MS=604800000", "CONSUMER_GROUP=ingestkit-consumer"}
	cfg = append(cfg, []byte(strings.Join(s.env, "\n")+"\n")...)
	if err = os.WriteFile(filepath.Join(s.Directory, "run.env"), cfg, 0600); err != nil {
		return err
	}
	s.APIURL = fmt.Sprintf("https://localhost:%d", ports[1])
	s.caPath = filepath.Join(s.Directory, "root.crt")
	if o.ExposeDependencies {
		c, e := ReadDeploymentConfig(filepath.Join(s.Directory, "run.env"))
		if e != nil {
			return e
		}
		s.DatabaseURL = fmt.Sprintf("postgres://ingestkit_admin:%s@127.0.0.1:%d/ingestkit?sslmode=disable", url.QueryEscape(c.AdminPassword), ports[2])
		s.ConsumerURL = fmt.Sprintf("http://127.0.0.1:%d", ports[3])
		s.Brokers = fmt.Sprintf("127.0.0.1:%d", ports[4])
	}
	return nil
}
func (s *Stack) startDeployment(ctx context.Context) error {
	if _, err := s.Compose(ctx, "up", "-d", "--build", "--wait", "--wait-timeout", "100"); err != nil {
		return err
	}
	deadline := time.Now().Add(15 * time.Second)
	for {
		_, err := s.Compose(ctx, "cp", "gateway:/data/caddy/pki/authorities/local/root.crt", s.caPath)
		if err == nil {
			client, e := s.HTTPClient()
			if e != nil {
				return e
			}
			res, e := client.Get(s.APIURL + "/health")
			if e == nil {
				res.Body.Close()
				if res.StatusCode == 200 {
					return nil
				}
			}
		}
		if time.Now().After(deadline) {
			return errors.New("local TLS deployment did not become healthy")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
