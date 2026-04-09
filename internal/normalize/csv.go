package normalize

import (
	"encoding/csv"
	"fmt"
	"strings"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
)

// CSVConfig defines CSV parsing behavior.
type CSVConfig struct {
	Delimiter   rune     // field delimiter (default: ',')
	HasHeaders  bool     // first row contains column names
	ColumnNames []string // explicit column names (overrides headers)
	TrimSpace   bool     // trim whitespace from fields
}

// DefaultCSVConfig returns standard CSV parsing defaults.
func DefaultCSVConfig() CSVConfig {
	return CSVConfig{
		Delimiter:  ',',
		HasHeaders: true,
		TrimSpace:  true,
	}
}

// CSV normalizer converts CSV rows into structured records.
type CSV struct {
	config      CSVConfig
	headers     []string
	initialized bool
	lineBuffer  string // buffer for incomplete lines
}

// NewCSV creates a CSV normalizer with default configuration.
func NewCSV() *CSV {
	return NewCSVWithConfig(DefaultCSVConfig())
}

// NewCSVWithConfig creates a CSV normalizer with explicit configuration.
func NewCSVWithConfig(cfg CSVConfig) *CSV {
	if cfg.Delimiter == 0 {
		cfg.Delimiter = ','
	}
	return &CSV{
		config: cfg,
	}
}

// Init initializes the normalizer (extracts headers if needed).
func (c *CSV) Init(run *core.Run) error {
	c.initialized = false
	c.lineBuffer = ""

	// Use explicit column names if provided
	if len(c.config.ColumnNames) > 0 {
		c.headers = c.config.ColumnNames
		c.initialized = true
	}

	return nil
}

// Ingest parses CSV data and returns normalized records.
func (c *CSV) Ingest(raw []byte) ([]core.Record, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	// Accumulate data in buffer (handles partial lines)
	c.lineBuffer += string(raw)

	// Split into complete lines
	lines := strings.Split(c.lineBuffer, "\n")

	// Keep the last incomplete line in buffer
	if !strings.HasSuffix(c.lineBuffer, "\n") {
		c.lineBuffer = lines[len(lines)-1]
		lines = lines[:len(lines)-1]
	} else {
		c.lineBuffer = ""
	}

	var records []core.Record

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		rec, err := c.parseLine(line)
		if err != nil {
			// Log error but continue processing
			continue
		}

		if rec != nil {
			records = append(records, *rec)
		}
	}

	return records, nil
}

// parseLine converts a single CSV line into a record.
func (c *CSV) parseLine(line string) (*core.Record, error) {
	reader := csv.NewReader(strings.NewReader(line))
	reader.Comma = c.config.Delimiter
	reader.TrimLeadingSpace = c.config.TrimSpace

	fields, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("csv parse error: %w", err)
	}

	// If not initialized and config expects headers, use this line as headers
	if !c.initialized && c.config.HasHeaders && len(c.headers) == 0 {
		c.headers = fields
		c.initialized = true
		return nil, nil // don't create a record for the header row
	}

	c.initialized = true

	// If no headers are set, generate column names (col_0, col_1, ...)
	if len(c.headers) == 0 {
		c.headers = make([]string, len(fields))
		for i := range c.headers {
			c.headers[i] = fmt.Sprintf("col_%d", i)
		}
	}

	// Build payload from fields
	payload := make(map[string]any)

	for i, field := range fields {
		if i >= len(c.headers) {
			// More fields than headers - use generated names
			payload[fmt.Sprintf("col_%d", i)] = field
		} else {
			payload[c.headers[i]] = field
		}
	}

	// Handle case where we have more headers than fields (fill with empty strings)
	for i := len(fields); i < len(c.headers); i++ {
		payload[c.headers[i]] = ""
	}

	rec := core.Record{
		TS:      time.Now().UTC(),
		Seq:     0, // will be set by pipeline
		Type:    "csv_row",
		Payload: payload,
	}

	return &rec, nil
}

// Close finalizes the normalizer (processes any remaining buffered data).
func (c *CSV) Close() error {
	// Process any remaining data in buffer
	if strings.TrimSpace(c.lineBuffer) != "" {
		_, err := c.parseLine(c.lineBuffer)
		c.lineBuffer = ""
		return err
	}
	return nil
}
