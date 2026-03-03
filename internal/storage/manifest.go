package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
)

// ManifestDocument represents the serialized metadata persisted alongside capture artifacts.
type ManifestDocument struct {
	RunID        string               `json:"run_id"`
	Schema       string               `json:"schema_version"`
	Source       core.SourceMeta      `json:"source"`
	Manifest     core.Manifest        `json:"manifest"`
	Capture      core.CaptureSettings `json:"capture"`
	Started      time.Time            `json:"started"`
	Completed    *time.Time           `json:"completed,omitempty"`
	RecordsCount uint64               `json:"records_count"`
	PrimaryData  string               `json:"primary_data_uri"`
}

// LoadManifest reads a manifest.json file from disk.
func LoadManifest(path string) (ManifestDocument, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return ManifestDocument{}, fmt.Errorf("read manifest: %w", err)
	}

	var doc ManifestDocument
	if err := json.Unmarshal(payload, &doc); err != nil {
		return ManifestDocument{}, fmt.Errorf("decode manifest: %w", err)
	}

	if doc.RunID == "" {
		return ManifestDocument{}, fmt.Errorf("decode manifest: run_id missing")
	}

	return doc, nil
}

