package benchmark

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
)

type StudyIdentity struct {
	SourceSHA, SourceFingerprint, ControlsFingerprint, ImagesFingerprint, ConfigFingerprint, Mode string
	Seed                                                                                          int64
	SourceDirty                                                                                   bool
}
type CaseProgress struct {
	Status               string
	Attempt              int
	CompletedRepetitions []int
	Artifact             ArtifactRef
	Summary              json.RawMessage
	History              []CaseAttempt
	ErrorClass           string
}
type StudyJournal struct {
	Status            string
	ExecutionAttempts int
	MetadataArtifacts []ArtifactRef
	ManifestFiles     []string
	Resources         []ArtifactRef
	Schema            int
	Identity          StudyIdentity
	RunNamespace      string
	Cases             map[string]CaseProgress
}

func (j StudyJournal) Compatible(identity StudyIdentity) error {
	if j.Schema != 2 {
		return errors.New("unsupported study journal schema")
	}
	differences := []string{}
	for _, field := range []struct {
		name          string
		before, after any
	}{{"revision", j.Identity.SourceSHA, identity.SourceSHA}, {"source", j.Identity.SourceFingerprint, identity.SourceFingerprint}, {"controls", j.Identity.ControlsFingerprint, identity.ControlsFingerprint}, {"images", j.Identity.ImagesFingerprint, identity.ImagesFingerprint}, {"runtime", j.Identity.ConfigFingerprint, identity.ConfigFingerprint}, {"seed", j.Identity.Seed, identity.Seed}, {"mode", j.Identity.Mode, identity.Mode}, {"dirty status", j.Identity.SourceDirty, identity.SourceDirty}} {
		if !reflect.DeepEqual(field.before, field.after) {
			differences = append(differences, field.name)
		}
	}
	if len(differences) > 0 {
		return fmt.Errorf("study identity differs (%s); use the original inputs or a new output directory", strings.Join(differences, ", "))
	}
	return nil
}

func fingerprint(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
func gitBytes(ctx context.Context, root string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	raw, e := cmd.Output()
	if e != nil {
		return nil, errors.New("cannot inspect study source checkout")
	}
	return raw, nil
}
func sourceIdentity(ctx context.Context, root, out string) (sha, signature string, dirty bool, result error) {
	exclusion := []string{"--", "."}
	if relative, e := filepath.Rel(root, out); e == nil && filepath.IsLocal(relative) && relative != "." {
		exclusion = append(exclusion, ":(exclude)"+filepath.ToSlash(relative))
	}
	raw, e := gitBytes(ctx, root, "rev-parse", "HEAD")
	if e != nil {
		return "", "", false, e
	}
	sha = strings.TrimSpace(string(raw))
	args := append([]string{"status", "--porcelain", "-z", "--untracked-files=all"}, exclusion...)
	raw, e = gitBytes(ctx, root, args...)
	if e != nil {
		return "", "", false, e
	}
	dirty = len(raw) > 0
	args = append([]string{"ls-files", "-z", "--cached", "--others", "--exclude-standard"}, exclusion...)
	raw, e = gitBytes(ctx, root, args...)
	if e != nil {
		return "", "", false, e
	}
	names := strings.Split(strings.TrimRight(string(raw), "\x00"), "\x00")
	slices.Sort(names)
	names = slices.Compact(names)
	type sourceFile struct{ Name, Mode, Digest string }
	files := []sourceFile{}
	for _, name := range names {
		if name == "" {
			continue
		}
		path := filepath.Join(root, name)
		info, e := os.Lstat(path)
		if e != nil {
			return "", "", false, errors.New("source changed during fingerprinting")
		}
		var data []byte
		if info.Mode()&os.ModeSymlink != 0 {
			target, e := os.Readlink(path)
			if e != nil {
				return "", "", false, e
			}
			data = []byte(target)
		} else if info.Mode().IsRegular() {
			data, e = os.ReadFile(path)
			if e != nil {
				return "", "", false, e
			}
		} else {
			return "", "", false, errors.New("unsupported study source file")
		}
		sum := sha256.Sum256(data)
		files = append(files, sourceFile{name, info.Mode().String(), hex.EncodeToString(sum[:])})
	}
	return sha, fingerprint(files), dirty, nil
}
func runtimeFingerprint(m Manifest) string {
	host := map[string]string{}
	for _, key := range []string{"go", "os", "architecture", "generator_logical_cpus", "kernel", "docker_vm", "memory_bytes", "os_version", "docker", "compose"} {
		host[key] = m.Host[key]
	}
	// cpuinfo may include changing CPU MHz on Linux; model/architecture and VM
	// allocation are enough for this identity while full host evidence is retained.
	if m.Host["os"] == "darwin" {
		host["cpu"] = m.Host["cpu"]
	} else {
		stable := []string{}
		for _, line := range strings.Split(m.Host["cpu"], "\n") {
			key, value, ok := strings.Cut(line, ":")
			key = strings.TrimSpace(key)
			if ok && (key == "model name" || key == "Hardware" || key == "CPU architecture" || key == "CPU part") {
				stable = append(stable, key+":"+strings.TrimSpace(value))
			}
		}
		slices.Sort(stable)
		host["cpu"] = strings.Join(slices.Compact(stable), "\n")
	}
	return fingerprint(struct {
		Settings map[string]any
		Database map[string]string
		Topics   string
		Host     map[string]string
	}{m.Settings, m.Database, m.TopicConfiguration, host})
}
func imageFingerprint(ctx context.Context, m Manifest) (string, error) {
	ids := []string{}
	for _, line := range strings.Split(m.Images, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			ids = append(ids, fields[0])
		}
	}
	if len(ids) == 0 {
		return "", errors.New("missing measured images")
	}
	args := append([]string{"image", "inspect"}, ids...)
	raw, e := exec.CommandContext(ctx, "docker", args...).Output()
	if e != nil {
		return "", errors.New("cannot inspect image identity")
	}
	return canonicalImageFingerprint(raw)
}
func canonicalImageFingerprint(raw []byte) (string, error) {
	var images []struct {
		Architecture, Os, Variant string
		Config                    map[string]any
		RootFS                    struct{ Layers []string }
	}
	if json.Unmarshal(raw, &images) != nil || len(images) == 0 {
		return "", errors.New("invalid image identity inspection")
	}
	signatures := []string{}
	for _, image := range images {
		if len(image.RootFS.Layers) == 0 || image.Config == nil {
			return "", errors.New("incomplete image identity")
		}
		// Compose creates a distinct config ID solely for each project's label.
		// Keep all content layers and runtime configuration, excluding that one label.
		if labels, ok := image.Config["Labels"].(map[string]any); ok {
			delete(labels, "com.docker.compose.project")
		}
		signatures = append(signatures, fingerprint(struct {
			Architecture, OS, Variant string
			Layers                    []string
			Config                    map[string]any
		}{image.Architecture, image.Os, image.Variant, image.RootFS.Layers, image.Config}))
	}
	slices.Sort(signatures)
	return fingerprint(signatures), nil
}

func readJournal(path string) (StudyJournal, error) {
	var j StudyJournal
	info, e := os.Lstat(path)
	if e != nil {
		return j, e
	}
	if !info.Mode().IsRegular() || info.Size() > 8<<20 {
		return j, errors.New("invalid study journal file")
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		return j, e
	}
	e = json.Unmarshal(raw, &j)
	if e != nil {
		return j, errors.New("invalid study journal JSON")
	}
	return j, nil
}
