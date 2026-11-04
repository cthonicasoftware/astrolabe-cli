// Package normalize provides normalizers that convert raw source bytes into
// structured core.Record values ready for storage.
package normalize

import (
	"bytes"
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

// Flusher is an optional interface for normalizers that buffer partial input.
// The pipeline calls Flush once after the last frame so trailing data that never
// saw a line terminator is still recorded.
type Flusher interface {
	Flush() []core.Record
}

// LineJSON is a normalizer that wraps each line in a minimal JSON record with a
// "line" payload field. It is the default normalizer for interactive captures.
//
// Sources such as serial and TCP deliver arbitrary read chunks, not lines, so
// LineJSON buffers input and emits one record per '\n'-terminated line. Line
// terminators ("\n" and "\r\n") are stripped from the stored line.
type LineJSON struct {
	buf []byte
}

// NewLineJSON returns a new LineJSON normalizer.
func NewLineJSON() *LineJSON { return &LineJSON{} }

// Init resets any buffered partial line.
func (l *LineJSON) Init(*core.Run) error {
	l.buf = l.buf[:0]
	return nil
}

// Ingest appends b to the line buffer and returns a record for each complete line.
func (l *LineJSON) Ingest(b []byte) ([]core.Record, error) {
	l.buf = append(l.buf, b...)

	var records []core.Record
	for {
		i := bytes.IndexByte(l.buf, '\n')
		if i < 0 {
			break
		}
		records = append(records, lineRecord(l.buf[:i]))
		l.buf = l.buf[i+1:]
	}

	// Compact so the backing array doesn't grow without bound.
	if len(l.buf) == 0 {
		l.buf = nil
	} else {
		l.buf = append([]byte(nil), l.buf...)
	}
	return records, nil
}

// Flush returns a record for any buffered partial line and clears the buffer.
func (l *LineJSON) Flush() []core.Record {
	if len(l.buf) == 0 {
		return nil
	}
	rec := lineRecord(l.buf)
	l.buf = nil
	return []core.Record{rec}
}

func lineRecord(line []byte) core.Record {
	line = bytes.TrimSuffix(line, []byte("\r"))
	return core.Record{
		TS:      time.Now().UTC(),
		Seq:     0,
		Type:    "sample",
		Payload: map[string]any{"line": string(line)},
	}
}

// Close satisfies the Normalizer interface; LineJSON holds no resources.
func (l *LineJSON) Close() error { return nil }

// Format implements the optional format describer consulted by the capture
// service to record the record format in run manifests.
func (l *LineJSON) Format() string { return "jsonl" }
