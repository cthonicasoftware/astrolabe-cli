// Package normalize provides normalizers that convert raw source bytes into
// structured core.Record values ready for storage.
package normalize

import (
	"strings"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
)

// Normalizer converts raw source bytes into one or more structured Records.
// Implementations are stateful: Init is called once at the start of a run,
// Ingest is called per frame, and Close is called when the source is exhausted.
type Normalizer interface {
	Init(run *core.Run) error
	Ingest(raw []byte) ([]core.Record, error)
	Close() error
}

// LineJSON is a normalizer that wraps each raw line in a minimal JSON record
// with a "line" payload field. It is the default normalizer for interactive captures.
type LineJSON struct{}

// NewLineJSON returns a new LineJSON normalizer.
func NewLineJSON() *LineJSON { return &LineJSON{} }

// Init satisfies the Normalizer interface; LineJSON requires no initialization.
func (l *LineJSON) Init(*core.Run) error { return nil }

// Ingest wraps the raw bytes in a single Record with a "line" payload field.
func (l *LineJSON) Ingest(b []byte) ([]core.Record, error) {
	s := strings.TrimSpace(string(b))
	rec := core.Record{
		TS:      time.Now().UTC(),
		Seq:     0,
		Type:    "sample",
		Payload: map[string]any{"line": s},
	}
	return []core.Record{rec}, nil
}

// Close satisfies the Normalizer interface; LineJSON holds no resources.
func (l *LineJSON) Close() error { return nil }

