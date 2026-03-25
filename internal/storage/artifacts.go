package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
)

const ArtifactsFileName = "artifacts.json"

// ArtifactsDocument caches artifact metadata for a run.
type ArtifactsDocument struct {
	SchemaVersion string           `json:"schema_version"`
	Artifacts     []ArtifactRecord `json:"artifacts"`
}

// ArtifactRecord is the persisted form of a single artifact entry.
type ArtifactRecord struct {
	Name      string            `json:"name"`
	RelPath   string            `json:"rel_path"`
	MediaType string            `json:"media_type"`
	Role      core.ArtifactRole `json:"role"`
	SizeBytes int64             `json:"size_bytes"`
	Checksum  core.Checksum     `json:"checksum"`
	CreatedAt time.Time         `json:"created_at"`
}

// LoadArtifacts reads an artifacts.json file from disk.
func LoadArtifacts(path string) (ArtifactsDocument, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return ArtifactsDocument{}, fmt.Errorf("read artifacts: %w", err)
	}

	var doc ArtifactsDocument
	if err := json.Unmarshal(payload, &doc); err != nil {
		return ArtifactsDocument{}, fmt.Errorf("decode artifacts: %w", err)
	}

	for i, artifact := range doc.Artifacts {
		if artifact.Name == "" {
			return ArtifactsDocument{}, fmt.Errorf("decode artifacts: artifact name missing at index %d", i)
		}
		if artifact.RelPath == "" {
			return ArtifactsDocument{}, fmt.Errorf("decode artifacts: rel_path missing for %s", artifact.Name)
		}
		clean := filepath.Clean(artifact.RelPath)
		if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.IsAbs(clean) {
			return ArtifactsDocument{}, fmt.Errorf("decode artifacts: invalid rel_path for %s", artifact.Name)
		}
	}

	return doc, nil
}
