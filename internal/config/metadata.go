package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
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

// MetadataRepository defines the contract for loading and saving metadata.
type MetadataRepository interface {
	Load() (Metadata, error)
	Save(Metadata) (string, error)
	Path() (string, error)
}

// MetadataPathResolver returns the metadata file path.
type MetadataPathResolver func() (string, error)

// FileMetadataRepository persists metadata to disk using a configurable path resolver.
type FileMetadataRepository struct {
	resolvePath MetadataPathResolver
}

// NewFileMetadataRepository constructs a repository that resolves its path on demand.
func NewFileMetadataRepository(resolver MetadataPathResolver) *FileMetadataRepository {
	if resolver == nil {
		resolver = metadataFilePath
	}
	return &FileMetadataRepository{resolvePath: resolver}
}

func (r *FileMetadataRepository) Path() (string, error) {
	return r.resolvePath()
}

func (r *FileMetadataRepository) Load() (Metadata, error) {
	path, err := r.resolvePath()
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

func (r *FileMetadataRepository) Save(meta Metadata) (string, error) {
	if err := meta.Validate(); err != nil {
		return "", err
	}

	path, err := r.resolvePath()
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

var defaultMetadataRepository MetadataRepository = NewFileMetadataRepository(nil)

// metadataFilePath resolves the metadata file location within the astrolabe config directory.
func metadataFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate metadata file: %w", err)
	}
	return filepath.Join(home, ".astrolabe", metadataFileName), nil
}

// MetadataPath exposes the configured metadata file location.
func MetadataPath() (string, error) {
	return defaultMetadataRepository.Path()
}

// LoadMetadata reads the persisted metadata file if present, returning defaults otherwise.
func LoadMetadata() (Metadata, error) {
	return defaultMetadataRepository.Load()
}

// SaveMetadata writes the metadata document to disk, ensuring directory creation.
func SaveMetadata(meta Metadata) (string, error) {
	return defaultMetadataRepository.Save(meta)
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

// ParseTags converts a comma-delimited list into normalized tags.
func ParseTags(input string) ([]string, error) {
	if strings.TrimSpace(input) == "" {
		return nil, nil
	}
	raw := strings.Split(input, ",")
	tags := make([]string, 0, len(raw))
	for _, tag := range raw {
		t := strings.TrimSpace(tag)
		if t == "" {
			return nil, fmt.Errorf("tags cannot contain empty values")
		}
		tags = append(tags, t)
	}
	return tags, nil
}

// ParseAttributes converts newline-delimited key=value pairs to a map.
func ParseAttributes(input string) (map[string]string, error) {
	result := map[string]string{}
	lines := strings.Split(input, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid attribute %q (expected key=value)", line)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			return nil, fmt.Errorf("attribute key missing in %q", line)
		}
		if value == "" {
			return nil, fmt.Errorf("attribute %q has empty value", key)
		}
		result[key] = value
	}
	return result, nil
}

// FormatAttributeLines renders attributes as sorted key=value lines.
func FormatAttributeLines(attrs map[string]string) string {
	if len(attrs) == 0 {
		return ""
	}
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s=%s", k, attrs[k]))
	}
	return strings.Join(lines, "\n")
}

