package runenv

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/feat7/ingestkit/internal/config"
	"github.com/feat7/ingestkit/internal/event"
	"os"
	"regexp"
	"strings"
)

// DeploymentConfig contains credentials. Secret fields are never serialized.
type DeploymentConfig struct {
	Domain         string            `json:"domain"`
	APIKeys        map[string]string `json:"-"`
	AdminPassword  string            `json:"-"`
	WriterPassword string            `json:"-"`
}

var deploymentSecret = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
var hostnameLabel = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)

func validDeploymentDomain(domain string) bool {
	if len(domain) == 0 || len(domain) > 253 || domain != "localhost" && !strings.Contains(domain, ".") {
		return false
	}
	for _, label := range strings.Split(domain, ".") {
		if !hostnameLabel.MatchString(label) {
			return false
		}
	}
	return true
}

// ReadDeploymentConfig parses generated dotenv data; it does not expand or run
// shell expressions. Errors name a field but never echo its credential value.
func ReadDeploymentConfig(path string) (DeploymentConfig, error) {
	var cfg DeploymentConfig
	file, err := os.Open(path)
	if err != nil {
		return cfg, errors.New("cannot open deployment configuration")
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil || !stat.Mode().IsRegular() || stat.Mode().Perm()&0077 != 0 {
		return cfg, errors.New("deployment configuration must be a private regular file (0600)")
	}
	values := map[string]string{}
	scan := bufio.NewScanner(file)
	scan.Buffer(make([]byte, 1024), 65536)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return cfg, errors.New("invalid deployment configuration assignment")
		}
		if _, exists := values[key]; exists {
			return cfg, errors.New("duplicate deployment configuration assignment")
		}
		values[key] = strings.TrimSpace(value)
	}
	if scan.Err() != nil {
		return cfg, errors.New("cannot read deployment configuration")
	}
	cfg.Domain = values["DOMAIN"]
	if !validDeploymentDomain(cfg.Domain) {
		return cfg, errors.New("invalid or missing DOMAIN hostname")
	}
	cfg.AdminPassword = values["POSTGRES_ADMIN_PASSWORD"]
	cfg.WriterPassword = values["POSTGRES_WRITER_PASSWORD"]
	for _, key := range []string{"POSTGRES_ADMIN_PASSWORD", "POSTGRES_WRITER_PASSWORD"} {
		if !deploymentSecret.MatchString(values[key]) {
			return DeploymentConfig{}, fmt.Errorf("invalid or missing %s; use a generated 64-hex secret", key)
		}
	}
	cfg.APIKeys, err = config.ParseAPIKeys(values["API_KEYS"])
	if err != nil || len(cfg.APIKeys) == 0 {
		return DeploymentConfig{}, errors.New("invalid or missing API_KEYS mapping")
	}
	for key, tenant := range cfg.APIKeys {
		if !deploymentSecret.MatchString(key) || !event.ValidTenant(tenant) {
			return DeploymentConfig{}, errors.New("invalid API_KEYS key or tenant")
		}
	}
	return cfg, nil
}
