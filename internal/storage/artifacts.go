package storage

import (
	"encoding/json"
	"fmt"
	"os"
	stdpath "path"
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
	Name             string            `json:"name"`
	RelPath          string            `json:"rel_path"`
	MediaType        string            `json:"media_type"`
	Role             core.ArtifactRole `json:"role"`
	SizeBytes        int64             `json:"size_bytes"`
	Checksum         core.Checksum     `json:"checksum"`
	CreatedAt        time.Time         `json:"created_at"`
	UploadedAt       *time.Time        `json:"uploaded_at,omitempty"`        // set after a successful upload/confirm
	RemoteArtifactID string            `json:"remote_artifact_id,omitempty"` // backend-assigned ULID
}

// NewArtifactsDocument builds an ArtifactsDocument from in-memory artifacts,
// normalizing each artifact's absolute Path into a runDir-relative slash path.
func NewArtifactsDocument(runDir string, artifacts []core.Artifact) (ArtifactsDocument, error) {
	doc := ArtifactsDocument{
		SchemaVersion: "1",
		Artifacts:     make([]ArtifactRecord, 0, len(artifacts)),
	}

	for _, artifact := range artifacts {
		relPath, err := filepath.Rel(runDir, artifact.Path)
		if err != nil {
			return ArtifactsDocument{}, fmt.Errorf("relativize artifact path %s: %w", artifact.Name, err)
		}
		doc.Artifacts = append(doc.Artifacts, ArtifactRecord{
			Name:             artifact.Name,
			RelPath:          filepath.ToSlash(relPath),
			MediaType:        artifact.MediaType,
			Role:             artifact.Role,
			SizeBytes:        artifact.SizeBytes,
			Checksum:         artifact.Checksum,
			CreatedAt:        artifact.CreatedAt,
			UploadedAt:       artifact.UploadedAt,
			RemoteArtifactID: artifact.RemoteArtifactID,
		})
	}

	return doc, nil
}

// SaveArtifacts writes an artifacts.json document to disk.
func SaveArtifacts(filePath string, doc ArtifactsDocument) error {
	payload, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal artifacts: %w", err)
	}
	payload = append(payload, '\n')

	if err := os.WriteFile(filePath, payload, 0o644); err != nil {
		return fmt.Errorf("write artifacts: %w", err)
	}

	return nil
}

// LoadArtifacts reads an artifacts.json file from disk.
func LoadArtifacts(filePath string) (ArtifactsDocument, error) {
	payload, err := os.ReadFile(filePath)
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
		normalized := strings.ReplaceAll(artifact.RelPath, "\\", "/")
		clean := stdpath.Clean(normalized)
		if strings.HasPrefix(normalized, "/") || clean == ".." || strings.HasPrefix(clean, "../") {
			return ArtifactsDocument{}, fmt.Errorf("decode artifacts: invalid rel_path for %s", artifact.Name)
		}
		doc.Artifacts[i].RelPath = clean
	}

	return doc, nil
}
