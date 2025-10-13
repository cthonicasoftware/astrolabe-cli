package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/core"
)

// Metadata captures operator, device, and test context common to CLI and TUI workflows.
type Metadata struct {
	Operator   string            `json:"operator,omitempty"`
	Location   string            `json:"location,omitempty"`
	Device     core.DeviceInfo   `json:"device"`
	Test       core.TestInfo     `json:"test"`
	Tags       []string          `json:"tags,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

const metadataFileName = "metadata.json"

// metadataFilePath resolves the metadata file location within the qa-agent config directory.
func metadataFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate metadata file: %w", err)
	}
	return filepath.Join(home, ".qa-agent", metadataFileName), nil
}

// MetadataPath exposes the configured metadata file location.
func MetadataPath() (string, error) {
	return metadataFilePath()
}

// LoadMetadata reads the persisted metadata file if present, returning defaults otherwise.
func LoadMetadata() (Metadata, error) {
	path, err := metadataFilePath()
	if err != nil {
		return Metadata{}, err
	}

	payload, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Metadata{}, nil
	}
	if err != nil {
		return Metadata{}, fmt.Errorf("read metadata: %w", err)
	}

	var meta Metadata
	if err := json.Unmarshal(payload, &meta); err != nil {
		return Metadata{}, fmt.Errorf("decode metadata: %w", err)
	}

	meta.normalize()
	return meta, nil
}

// SaveMetadata writes the metadata document to disk, ensuring directory creation.
func SaveMetadata(meta Metadata) (string, error) {
	if err := meta.Validate(); err != nil {
		return "", err
	}

	path, err := metadataFilePath()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create metadata directory: %w", err)
	}

	meta.normalize()

	payload, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode metadata: %w", err)
	}
	payload = append(payload, '\n')

	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return "", fmt.Errorf("write metadata: %w", err)
	}
	return path, nil
}

// Validate ensures the metadata conforms to simple structural constraints.
func (m Metadata) Validate() error {
	for i, tag := range m.Tags {
		if strings.TrimSpace(tag) == "" {
			return fmt.Errorf("tag at index %d is empty", i)
		}
	}

	for k, v := range m.Attributes {
		if strings.TrimSpace(k) == "" {
			return fmt.Errorf("attribute key cannot be empty")
		}
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("attribute %q has an empty value", k)
		}
	}

	return nil
}

func (m *Metadata) normalize() {
	if m.Attributes == nil {
		m.Attributes = map[string]string{}
	}
	if len(m.Tags) > 0 {
		for i := range m.Tags {
			m.Tags[i] = strings.TrimSpace(m.Tags[i])
		}
		// Remove any duplicates post-normalization.
		unique := make([]string, 0, len(m.Tags))
		seen := make(map[string]struct{}, len(m.Tags))
		for _, tag := range m.Tags {
			if tag == "" {
				continue
			}
			if _, ok := seen[tag]; ok {
				continue
			}
			seen[tag] = struct{}{}
			unique = append(unique, tag)
		}
		sort.Strings(unique)
		m.Tags = unique
	}
}
