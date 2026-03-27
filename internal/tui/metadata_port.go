package tui

import "github.com/cthonicasoftware/astrolabe-cli/internal/core"

// MetadataPort abstracts loading and saving run metadata for the TUI.
type MetadataPort interface {
	Load() (MetadataValues, error)
	Save(MetadataValues) (string, error)
	Path() (string, error)
}

// MetadataValues is the TUI-owned representation of editable metadata.
type MetadataValues struct {
	Operator   string
	Location   string
	Device     core.DeviceInfo
	Test       core.TestInfo
	Tags       []string
	Attributes map[string]string
}
