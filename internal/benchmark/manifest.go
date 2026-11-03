package benchmark

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/feat7/ingestkit/internal/runenv"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Manifest struct {
	BrokerFsyncBypass          bool              `json:"broker_fsync_bypass"`
	CapturedAt                 time.Time         `json:"captured_at"`
	SourceSHA                  string            `json:"source_sha"`
	SourceDirty                bool              `json:"source_dirty"`
	Host                       map[string]string `json:"host"`
	Database                   map[string]string `json:"database"`
	Images, TopicConfiguration string
	Settings                   map[string]any `json:"settings"`
	Caveats                    []string       `json:"caveats"`
}

func localOutput(ctx context.Context, name string, args ...string) string {
	out, e := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if e != nil {
		return fmt.Sprintf("unavailable (%s)", name)
	}
	return strings.TrimSpace(string(out))
}
func CaptureManifest(ctx context.Context, s *runenv.Stack, p *pgxpool.Pool) (Manifest, error) {
	m := Manifest{CapturedAt: time.Now().UTC(), SourceSHA: localOutput(ctx, "git", "rev-parse", "HEAD"), SourceDirty: localOutput(ctx, "git", "status", "--porcelain", "--untracked-files=no") != "", Host: map[string]string{"go": runtime.Version(), "os": runtime.GOOS, "architecture": runtime.GOARCH, "generator_logical_cpus": fmt.Sprint(runtime.NumCPU()), "kernel": localOutput(ctx, "uname", "-a"), "disk_space": localOutput(ctx, "df", "-k", "."), "docker": localOutput(ctx, "docker", "version", "--format", "{{json .Server}}"), "compose": localOutput(ctx, "docker", "compose", "version"), "docker_vm": localOutput(ctx, "docker", "info", "--format", "CPUs={{.NCPU}} MemoryBytes={{.MemTotal}} StorageDriver={{.Driver}} Kernel={{.KernelVersion}} OS={{.OperatingSystem}}")}, Database: map[string]string{}, Settings: map[string]any{"api_rate_limit_rps": 10000, "producer_buffer_bytes": 67108864, "producer_buffer_records": 1000, "producer_ack": "all ISR; idempotent", "publish_timeout_ns": int64(5 * time.Second), "consumer_max_poll_records": 500, "consumer_processing_deadline_ns": int64(15 * time.Second), "consumer_session_timeout_ns": int64(10 * time.Second), "consumer_rebalance_timeout_ns": int64(60 * time.Second), "redpanda_smp": 1, "redpanda_memory": "512M", "topology": "one broker; one database; one API; one consumer"}, Caveats: []string{"Local Docker VM; generator and SQL observer run on the same physical host as services and compete for resources.", "Single broker, replication factor one. Disk-flush configuration does not establish tolerance of broker disk/host loss.", "Topic and database settings are captured below. Source defaults at the recorded revision define remaining client settings.", "Host filesystem reported by df is not a calibrated Docker VM disk specification. No dedicated disk latency or cloud cost measurement.", "Resource samples are coarse Docker snapshots, not isolation or proof of a particular bottleneck."}}

	// Topic write.caching does not reveal Seastar's process-wide fsync bypass.
	// Read the actual broker rather than inferring it from a Compose mode name.
	command := `for p in /proc/[0-9]*; do case "$(readlink "$p/exe" 2>/dev/null)" in */redpanda) cat "$p/cmdline"; exit 0;; esac; done; exit 1`
	rawArgs, err := s.Compose(ctx, "exec", "-T", "redpanda", "sh", "-c", command)
	if err != nil {
		return m, fmt.Errorf("inspect broker fsync setting: %w", err)
	}
	m.BrokerFsyncBypass, err = brokerFsyncBypass(rawArgs)
	if err != nil {
		return m, err
	}
	if m.BrokerFsyncBypass {
		return m, errors.New("refusing benchmark: running broker bypasses fsync")
	}
	if runtime.GOOS == "darwin" {
		m.Host["cpu"] = localOutput(ctx, "sysctl", "-n", "machdep.cpu.brand_string")
		m.Host["memory_bytes"] = localOutput(ctx, "sysctl", "-n", "hw.memsize")
		m.Host["os_version"] = localOutput(ctx, "sw_vers")
	} else {
		raw, _ := os.ReadFile("/proc/cpuinfo")
		m.Host["cpu"] = string(raw)
		raw, _ = os.ReadFile("/proc/meminfo")
		m.Host["memory"] = string(raw)
	}
	for _, key := range []string{"server_version", "fsync", "synchronous_commit", "full_page_writes", "wal_level", "max_connections"} {
		var value string
		if e := p.QueryRow(ctx, "SELECT current_setting($1)", key).Scan(&value); e != nil {
			return m, e
		}
		m.Database[key] = value
	}
	var indexes string
	if e := p.QueryRow(ctx, "SELECT string_agg(indexdef,E'\n' ORDER BY indexname) FROM pg_indexes WHERE schemaname='ingestkit' AND tablename='events'").Scan(&indexes); e != nil {
		return m, e
	}
	m.Database["indexes"] = indexes
	images, e := s.Compose(ctx, "images", "-q")
	if e != nil {
		return m, e
	}
	seen := map[string]bool{}
	for _, id := range strings.Fields(images) {
		if !seen[id] {
			m.Images += localOutput(ctx, "docker", "image", "inspect", id, "--format", "{{.Id}} {{json .RepoDigests}}") + "\n"
			seen[id] = true
		}
	}
	for _, topic := range []string{"ingestkit.events", "ingestkit.dlq"} {
		summary, e := s.Compose(ctx, "exec", "-T", "redpanda", "rpk", "topic", "describe", topic, "-X", "brokers=redpanda:9092")
		if e != nil {
			return m, e
		}
		m.TopicConfiguration += summary + "\n"
		out, e := s.Compose(ctx, "exec", "-T", "redpanda", "rpk", "topic", "describe", topic, "-c", "-X", "brokers=redpanda:9092")
		if e != nil {
			return m, e
		}
		m.TopicConfiguration += out + "\n"
	}
	return m, nil
}
func WriteJSON(path string, value any) error {
	raw, e := json.MarshalIndent(value, "", "  ")
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0755); e != nil {
		return e
	}
	return os.WriteFile(path, append(raw, '\n'), 0644)
}

type ResourceSample struct {
	At         time.Time `json:"at"`
	Statistics string    `json:"docker_statistics"`
	Error      bool      `json:"error"`
}

// SampleResources returns only this stack's container stats. Cancellation stops
// the sampler; the caller waits for the result before deleting its containers.
func SampleResources(ctx context.Context, s *runenv.Stack) <-chan []ResourceSample {
	done := make(chan []ResourceSample, 1)
	go func() {
		var samples []ResourceSample
		for {
			if ctx.Err() != nil {
				done <- samples
				return
			}
			call, cancel := context.WithTimeout(ctx, 5*time.Second)
			out, e := s.Compose(call, "stats", "--no-stream", "--format", "json")
			cancel()
			samples = append(samples, ResourceSample{At: time.Now().UTC(), Statistics: out, Error: e != nil})
			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				done <- samples
				return
			case <-timer.C:
			}
		}
	}()
	return done
}

// Fail closed if the effective argument is absent, malformed or ambiguous;
// silently filling in false would make an unverified manifest look durable.
func brokerFsyncBypass(raw string) (bool, error) {
	args := strings.Split(raw, "\x00")
	seen := false
	value := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		var text string
		if arg == "--unsafe-bypass-fsync" {
			i++
			if i >= len(args) {
				return false, errors.New("broker fsync argument has no value")
			}
			text = args[i]
		} else if strings.HasPrefix(arg, "--unsafe-bypass-fsync=") {
			text = strings.TrimPrefix(arg, "--unsafe-bypass-fsync=")
		} else {
			continue
		}
		if seen {
			return false, errors.New("ambiguous broker fsync arguments")
		}
		seen = true
		var err error
		value, err = strconv.ParseBool(text)
		if err != nil {
			return false, errors.New("invalid broker fsync argument")
		}
	}
	if !seen {
		return false, errors.New("could not confirm explicit broker fsync bypass setting")
	}
	return value, nil
}
