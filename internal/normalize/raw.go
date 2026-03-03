package normalize

import (
	"strings"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
)

// RawConfig defines raw text parsing behavior.
type RawConfig struct {
	RecordType   string // record type to assign (default: "raw")
	TrimSpace    bool   // trim whitespace from lines
	SkipEmpty    bool   // skip empty lines
	PreserveRaw  bool   // include raw line in payload
	SplitOnLines bool   // split on newlines (default: true)
}

// DefaultRawConfig returns standard raw parsing defaults.
func DefaultRawConfig() RawConfig {
	return RawConfig{
		RecordType:   "raw",
		TrimSpace:    true,
		SkipEmpty:    true,
		PreserveRaw:  true,
		SplitOnLines: true,
	}
}

// Raw normalizer converts raw text lines into records.
// This is useful for log files, plain text data, or any unstructured content.
type Raw struct {
	config     RawConfig
	lineBuffer string
}

// NewRaw creates a raw normalizer with default configuration.
func NewRaw() *Raw {
	return NewRawWithConfig(DefaultRawConfig())
}

// NewRawWithConfig creates a raw normalizer with explicit configuration.
func NewRawWithConfig(cfg RawConfig) *Raw {
	if cfg.RecordType == "" {
		cfg.RecordType = "raw"
	}
	return &Raw{
		config: cfg,
	}
}

// Init initializes the normalizer.
func (r *Raw) Init(run *core.Run) error {
	r.lineBuffer = ""
	return nil
}

// Ingest processes raw bytes and returns normalized records.
func (r *Raw) Ingest(raw []byte) ([]core.Record, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	if !r.config.SplitOnLines {
		// Treat entire chunk as single record
		return r.processChunk(raw), nil
	}

	// Accumulate data in buffer (handles partial lines)
	r.lineBuffer += string(raw)

	// Split into complete lines
	lines := strings.Split(r.lineBuffer, "\n")

	// Keep the last incomplete line in buffer
	if !strings.HasSuffix(r.lineBuffer, "\n") {
		r.lineBuffer = lines[len(lines)-1]
		lines = lines[:len(lines)-1]
	} else {
		r.lineBuffer = ""
	}

	var records []core.Record

	for _, line := range lines {
		if rec := r.processLine(line); rec != nil {
			records = append(records, *rec)
		}
	}

	return records, nil
}

// processLine converts a single line into a record.
func (r *Raw) processLine(line string) *core.Record {
	original := line

	if r.config.TrimSpace {
		line = strings.TrimSpace(line)
	}

	if r.config.SkipEmpty && line == "" {
		return nil
	}

	payload := make(map[string]any)

	if r.config.PreserveRaw {
		payload["raw"] = original
	}

	payload["content"] = line
	payload["length"] = len(line)

	rec := core.Record{
		TS:      time.Now().UTC(),
		Seq:     0, // will be set by pipeline
		Type:    r.config.RecordType,
		Payload: payload,
	}

	return &rec
}

// processChunk converts an entire chunk into a single record.
func (r *Raw) processChunk(chunk []byte) []core.Record {
	content := string(chunk)

	if r.config.TrimSpace {
		content = strings.TrimSpace(content)
	}

	if r.config.SkipEmpty && content == "" {
		return nil
	}

	payload := make(map[string]interface{})
	payload["content"] = content
	payload["length"] = len(content)

	if r.config.PreserveRaw {
		payload["raw"] = string(chunk)
	}

	rec := core.Record{
		TS:      time.Now().UTC(),
		Seq:     0,
		Type:    r.config.RecordType,
		Payload: payload,
	}

	return []core.Record{rec}
}

// Close finalizes the normalizer (processes any remaining buffered data).
func (r *Raw) Close() error {
	// Process any remaining data in buffer
	if r.config.SplitOnLines && strings.TrimSpace(r.lineBuffer) != "" {
		// Process final incomplete line
		r.lineBuffer = ""
	}
	return nil
}

