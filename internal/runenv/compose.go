// Package runenv owns isolated Compose projects for experiments and tests.
package runenv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Options struct{ Purpose, OutputDirectory string }
type Stack struct {
	APIURL, ConsumerURL, DatabaseURL, Brokers, Project, Directory string
	root, token                                                   string
	env                                                           []string
	owned                                                         bool
}

func projectName(purpose string) string {
	purpose = regexp.MustCompile("[^a-z0-9-]").ReplaceAllString(strings.ToLower(purpose), "")
	if len(purpose) > 20 {
		purpose = purpose[:20]
	}
	return "ingestkit-" + purpose + "-" + uuid.NewString()[:8]
}
func apiAddress(port string) string { return "http://" + strings.TrimSpace(port) }
func Root() (string, error) {
	p, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(p, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(p, "docker-compose.yml")); err == nil {
				return p, nil
			}
		}
		parent := filepath.Dir(p)
		if parent == p {
			return "", errors.New("run inside the IngestKit checkout")
		}
		p = parent
	}
}
func (s *Stack) composeArgs(args ...string) []string {
	base := []string{"compose", "--project-directory", s.root, "-p", s.Project, "-f", filepath.Join(s.root, "docker-compose.yml")}
	if s.Directory != "" {
		base = append(base, "--env-file", filepath.Join(s.Directory, "run.env"))
	}
	return append(base, args...)
}
func (s *Stack) docker(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = s.root
	cmd.Env = append(os.Environ(), s.env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		tail := string(out)
		if len(tail) > 5000 {
			tail = tail[len(tail)-5000:]
		}
		return string(out), fmt.Errorf("docker command failed: %w\n%s", err, tail)
	}
	return string(out), nil
}
func (s *Stack) Compose(ctx context.Context, args ...string) (string, error) {
	if !s.owned {
		return "", errors.New("stack is not owned by this run")
	}
	return s.docker(ctx, s.composeArgs(args...)...)
}
func Start(ctx context.Context, o Options) (*Stack, error) {
	root, err := Root()
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 3; attempt++ {
		s := &Stack{root: root, Project: projectName(o.Purpose), token: uuid.NewString(), owned: true}
		s.Directory = o.OutputDirectory
		if s.Directory == "" {
			s.Directory = filepath.Join(root, "tmp", "runs", s.Project)
		}
		s.Directory, err = filepath.Abs(s.Directory)
		if err != nil {
			return nil, err
		}
		if err = os.MkdirAll(s.Directory, 0700); err != nil {
			return nil, err
		}
		reservation, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, err
		}
		port := reservation.Addr().(*net.TCPAddr).Port
		s.env = []string{fmt.Sprintf("INGESTKIT_BROKER_PORT=%d", port), "API_PORT=0", "METRICS_PORT=0", "POSTGRES_PORT=0", "POSTGRES_DB=ingestkit_test", "POSTGRES_PASSWORD=local-test-only", "API_KEYS={\"local-demo-key\":\"demo\",\"local-second-key\":\"second\"}", "INGESTKIT_RUN_ID=" + s.token}
		if err = os.WriteFile(filepath.Join(s.Directory, "run.env"), []byte(strings.Join(s.env, "\n")+"\n"), 0600); err != nil {
			reservation.Close()
			return nil, err
		}
		reservation.Close()
		_, err = s.Compose(ctx, "up", "-d", "--build", "--wait", "--wait-timeout", "90")
		if err != nil {
			cleanup, end := context.WithTimeout(context.Background(), 20*time.Second)
			_ = s.CollectEvidence(cleanup, s.Directory)
			_ = s.Destroy(cleanup)
			end()
			if strings.Contains(err.Error(), "address already in use") {
				continue
			}
			return nil, err
		}
		endpoints := make(map[string]string)
		for service, containerPort := range map[string]string{"api": "8080", "consumer": "8081", "postgres": "5432"} {
			out, e := s.Compose(ctx, "port", service, containerPort)
			if e != nil {
				return nil, e
			}
			endpoints[service] = strings.TrimSpace(out)
		}
		s.APIURL = apiAddress(endpoints["api"])
		s.ConsumerURL = apiAddress(endpoints["consumer"])
		s.DatabaseURL = "postgres://ingestkit:local-test-only@" + endpoints["postgres"] + "/ingestkit_test?sslmode=disable"
		s.Brokers = fmt.Sprintf("127.0.0.1:%d", port)
		manifest, _ := json.MarshalIndent(map[string]string{"project": s.Project, "api_url": s.APIURL, "consumer_url": s.ConsumerURL, "broker": s.Brokers}, "", "  ")
		_ = os.WriteFile(filepath.Join(s.Directory, "stack.json"), manifest, 0600)
		return s, nil
	}
	return nil, errors.New("could not reserve a broker port after three attempts")
}
func (s *Stack) verifyOwner(ctx context.Context) error {
	if !s.owned || s.token == "" || !strings.HasPrefix(s.Project, "ingestkit-") {
		return errors.New("refusing unowned resource operation")
	}
	ids, err := s.Compose(ctx, "ps", "-a", "-q")
	if err != nil {
		return err
	}
	for _, id := range strings.Fields(ids) {
		out, err := s.docker(ctx, "inspect", "--format", `{{index .Config.Labels "io.ingestkit.run"}}`, id)
		if err != nil {
			return err
		}
		if strings.TrimSpace(out) != s.token {
			return errors.New("container ownership label mismatch")
		}
	}
	volumes, err := s.docker(ctx, "volume", "ls", "--filter", "label=com.docker.compose.project="+s.Project, "--format", "{{.Name}}")
	if err != nil {
		return err
	}
	for _, v := range strings.Fields(volumes) {
		out, err := s.docker(ctx, "volume", "inspect", "--format", `{{index .Labels "io.ingestkit.run"}}`, v)
		if err != nil {
			return err
		}
		if strings.TrimSpace(out) != s.token {
			return errors.New("volume ownership label mismatch")
		}
	}
	return nil
}
func (s *Stack) Stop(ctx context.Context) error {
	if err := s.verifyOwner(ctx); err != nil {
		return err
	}
	_, err := s.Compose(ctx, "stop")
	return err
}
func (s *Stack) Destroy(ctx context.Context) error {
	if err := s.verifyOwner(ctx); err != nil {
		return err
	}
	_, err := s.Compose(ctx, "down", "-v", "--remove-orphans")
	return err
}
func (s *Stack) SetServiceRunning(ctx context.Context, name string, running bool) error {
	switch name {
	case "api", "consumer", "postgres", "redpanda":
	default:
		return errors.New("unsupported service control")
	}
	if err := s.verifyOwner(ctx); err != nil {
		return err
	}
	action := "stop"
	if running {
		action = "start"
	}
	_, err := s.Compose(ctx, action, name)
	return err
}
func (s *Stack) CollectEvidence(ctx context.Context, directory string) error {
	if !s.owned {
		return errors.New("unowned stack")
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	for name, args := range map[string][]string{"containers.txt": {"ps", "-a"}, "services.log": {"logs", "--no-color", "--tail", "100"}, "images.txt": {"images"}} {
		out, err := s.Compose(ctx, args...)
		if err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(directory, name), []byte(out), 0600); err != nil {
			return err
		}
	}
	return nil
}
