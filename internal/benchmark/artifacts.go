package benchmark

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"
)

const maxRawArtifact = 128 << 20
const maxCompressedArtifact = 64 << 20

type ArtifactRef struct {
	Codec          string
	Path, SHA256   string
	Size, RawBytes int64
}

func WriteCompressed(path string, value any) (ArtifactRef, error) {
	var ref ArtifactRef
	raw, err := json.Marshal(value)
	if err != nil {
		return ref, err
	}
	if len(raw) > maxRawArtifact {
		return ref, errors.New("raw artifact exceeds128MiB bound")
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return ref, err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".capture-*")
	if err != nil {
		return ref, err
	}
	temp := file.Name()
	defer os.Remove(temp)
	writer, _ := gzip.NewWriterLevel(file, gzip.BestCompression)
	writer.Header.ModTime = time.Unix(0, 0)
	writer.Header.OS = 255
	_, err = writer.Write(raw)
	err = errors.Join(err, writer.Close(), file.Sync(), file.Close())
	if err != nil {
		return ref, err
	}
	data, err := os.ReadFile(temp)
	if err != nil {
		return ref, err
	}
	if len(data) > maxCompressedArtifact {
		return ref, errors.New("compressed artifact exceeds64MiB bound")
	}
	// Link publishes a complete file atomically and refuses an existing target.
	if err = os.Link(temp, path); err != nil {
		return ref, err
	}
	_ = os.Chmod(path, 0644)
	sum := sha256.Sum256(data)
	return ArtifactRef{Codec: "gzip", Path: filepath.Base(path), SHA256: hex.EncodeToString(sum[:]), Size: int64(len(data)), RawBytes: int64(len(raw))}, nil
}
func VerifyArtifact(directory string, ref ArtifactRef) error {
	if !filepath.IsLocal(ref.Path) || ref.Path == "." || len(ref.SHA256) != 64 {
		return errors.New("invalid artifact reference")
	}
	path := filepath.Join(directory, ref.Path)
	base, e := filepath.EvalSymlinks(directory)
	if e != nil {
		return e
	}
	parent, e := filepath.EvalSymlinks(filepath.Dir(path))
	if e != nil {
		return e
	}
	relative, e := filepath.Rel(base, parent)
	if e != nil || !filepath.IsLocal(relative) {
		return errors.New("artifact reference escapes output directory")
	}

	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > maxCompressedArtifact || info.Size() != ref.Size {
		return errors.New("invalid artifact size/type")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != ref.SHA256 {
		return errors.New("artifact checksum mismatch")
	}
	if ref.Codec == "json" {
		if int64(len(data)) != ref.RawBytes || !json.Valid(data) {
			return errors.New("invalid JSON manifest artifact")
		}
		return nil
	}
	if ref.Codec != "" && ref.Codec != "gzip" {
		return errors.New("unsupported artifact codec")
	}
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer reader.Close()
	count, err := io.Copy(io.Discard, io.LimitReader(reader, maxRawArtifact+1))
	if err != nil {
		return err
	}
	if count > maxRawArtifact || count != ref.RawBytes {
		return errors.New("invalid decompressed artifact bound")
	}
	return nil
}
func atomicJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".journal-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, err = file.Write(append(raw, '\n'))
	err = errors.Join(err, file.Sync(), file.Close())
	if err != nil {
		return err
	}
	if err = os.Rename(file.Name(), path); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func WriteJSONArtifact(path string, value any) (ArtifactRef, error) {
	var ref ArtifactRef
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return ref, err
	}
	raw = append(raw, '\n')
	if len(raw) > maxCompressedArtifact {
		return ref, errors.New("JSON artifact exceeds size bound")
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return ref, err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".metadata-*")
	if err != nil {
		return ref, err
	}
	defer os.Remove(file.Name())
	_, err = file.Write(raw)
	err = errors.Join(err, file.Sync(), file.Close())
	if err != nil {
		return ref, err
	}
	if err = os.Link(file.Name(), path); err != nil {
		return ref, err
	}
	_ = os.Chmod(path, 0644)
	sum := sha256.Sum256(raw)
	return ArtifactRef{Codec: "json", Path: filepath.Base(path), SHA256: hex.EncodeToString(sum[:]), Size: int64(len(raw)), RawBytes: int64(len(raw))}, nil
}
