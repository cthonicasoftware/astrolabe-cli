package storage

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
)

// Store abstracts the persistence layer used by capture pipelines.
type Store interface {
	Start(run *core.Run) error
	Write([]core.Record) error
	Finalize(run *core.Run) error
	Artifacts() []core.Artifact
}

// FS persists capture runs to the local filesystem.
type FS struct {
	root         string
	runDir       string
	dataPath     string
	manifestPath string
	dataFile     *os.File
	dataWriter   *bufio.Writer
	dataEncoder  *json.Encoder
	dataHasher   hash.Hash
	artifacts    []core.Artifact
}

// NewFS constructs a filesystem-backed store rooted at the provided path.
func NewFS(path string) *FS {
	return &FS{
		root:      path,
		artifacts: []core.Artifact{},
	}
}

// Start prepares the run directory and data file for streaming records.
func (f *FS) Start(run *core.Run) error {
	if run == nil || run.ID == "" {
		return fmt.Errorf("fs store: run ID is required")
	}

	runDir := filepath.Join(f.root, run.ID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return fmt.Errorf("fs store: create run dir: %w", err)
	}

	dataPath := filepath.Join(runDir, "data.jsonl")
	dataFile, err := os.OpenFile(dataPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("fs store: open data file: %w", err)
	}

	hasher := sha256.New()
	multi := io.MultiWriter(dataFile, hasher)
	writer := bufio.NewWriter(multi)
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)

	f.runDir = runDir
	f.dataPath = dataPath
	f.manifestPath = filepath.Join(runDir, "manifest.json")
	f.dataFile = dataFile
	f.dataWriter = writer
	f.dataEncoder = encoder
	f.dataHasher = hasher
	f.artifacts = f.artifacts[:0]
	return nil
}

// Write appends normalized records to the NDJSON stream.
func (f *FS) Write(records []core.Record) error {
	if f.dataEncoder == nil {
		return fmt.Errorf("fs store: data encoder not initialised")
	}
	for _, record := range records {
		if err := f.dataEncoder.Encode(record); err != nil {
			return fmt.Errorf("fs store: encode record: %w", err)
		}
	}
	return nil
}

// Finalize closes open handles, computes checksums, and emits artifact metadata.
func (f *FS) Finalize(run *core.Run) (retErr error) {
	if f.dataEncoder == nil {
		return fmt.Errorf("fs store: not started")
	}

	dataFile := f.dataFile
	defer func() {
		if dataFile != nil {
			if err := dataFile.Close(); err != nil && retErr == nil {
				retErr = fmt.Errorf("fs store: close data file: %w", err)
			}
		}
		f.dataFile = nil
		f.dataWriter = nil
		f.dataEncoder = nil
		f.dataHasher = nil
	}()

	if err := f.dataWriter.Flush(); err != nil {
		return fmt.Errorf("fs store: flush data writer: %w", err)
	}

	dataStat, err := os.Stat(f.dataPath)
	if err != nil {
		return fmt.Errorf("fs store: stat data file: %w", err)
	}

	dataChecksum := hex.EncodeToString(f.dataHasher.Sum(nil))
	dataArtifact := core.Artifact{
		Name:      "data.jsonl",
		Path:      f.dataPath,
		MediaType: "application/x-ndjson",
		Role:      core.ArtifactRoleData,
		SizeBytes: dataStat.Size(),
		Checksum: core.Checksum{
			Algorithm: "sha256",
			Value:     dataChecksum,
		},
		CreatedAt: run.Started,
	}
	f.artifacts = append(f.artifacts, dataArtifact)

	if run.PrimaryDataURI == "" {
		run.PrimaryDataURI = f.dataPath
	}

	if err := f.writeManifest(run); err != nil {
		return err
	}
	if err := f.writeArtifacts(); err != nil {
		return err
	}

	run.Artifacts = f.Artifacts()

	return nil
}

// Artifacts returns the artifacts generated during Finalize.
func (f *FS) Artifacts() []core.Artifact {
	artifacts := make([]core.Artifact, len(f.artifacts))
	copy(artifacts, f.artifacts)
	return artifacts
}

func (f *FS) writeManifest(run *core.Run) error {
	doc := ManifestDocument{
		RunID:        run.ID,
		Schema:       run.Manifest.SchemaVersion,
		Source:       run.Source,
		Manifest:     run.Manifest,
		Capture:      run.Capture,
		Started:      run.Started,
		Completed:    run.Completed,
		RecordsCount: run.RecordsCount,
		PrimaryData:  run.PrimaryDataURI,
	}

	payload, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("fs store: marshal manifest: %w", err)
	}
	payload = append(payload, '\n')

	if err := os.WriteFile(f.manifestPath, payload, 0o644); err != nil {
		return fmt.Errorf("fs store: write manifest: %w", err)
	}

	manifestBytes, err := os.ReadFile(f.manifestPath)
	if err != nil {
		return fmt.Errorf("fs store: read manifest: %w", err)
	}
	manifestChecksum := sha256.Sum256(manifestBytes)

	stat, err := os.Stat(f.manifestPath)
	if err != nil {
		return fmt.Errorf("fs store: stat manifest: %w", err)
	}

	manifestArtifact := core.Artifact{
		Name:      "manifest.json",
		Path:      f.manifestPath,
		MediaType: "application/json",
		Role:      core.ArtifactRoleManifest,
		SizeBytes: stat.Size(),
		Checksum: core.Checksum{
			Algorithm: "sha256",
			Value:     hex.EncodeToString(manifestChecksum[:]),
		},
		CreatedAt: run.Started,
	}

	// Place manifest first for readability in listings.
	f.artifacts = append([]core.Artifact{manifestArtifact}, f.artifacts...)
	return nil
}

func (f *FS) writeArtifacts() error {
	doc := ArtifactsDocument{
		SchemaVersion: "1",
		Artifacts:     make([]ArtifactRecord, 0, len(f.artifacts)),
	}

	for _, artifact := range f.artifacts {
		relPath, err := filepath.Rel(f.runDir, artifact.Path)
		if err != nil {
			return fmt.Errorf("fs store: relativize artifact path %s: %w", artifact.Name, err)
		}
		doc.Artifacts = append(doc.Artifacts, ArtifactRecord{
			Name:      artifact.Name,
			RelPath:   relPath,
			MediaType: artifact.MediaType,
			Role:      artifact.Role,
			SizeBytes: artifact.SizeBytes,
			Checksum:  artifact.Checksum,
			CreatedAt: artifact.CreatedAt,
		})
	}

	payload, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("fs store: marshal artifacts: %w", err)
	}
	payload = append(payload, '\n')

	path := filepath.Join(f.runDir, ArtifactsFileName)
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("fs store: write artifacts: %w", err)
	}

	return nil
}
