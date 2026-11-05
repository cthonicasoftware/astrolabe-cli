// Package normalize provides normalizers that convert raw source bytes into
// structured core.Record values ready for storage.
package normalize

import (
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

// TimedIngester is an optional interface for normalizers that stamp records
// with the time their bytes arrived rather than the time they were processed.
// The pipeline prefers it over Ingest when available.
type TimedIngester interface {
	IngestAt(raw []byte, at time.Time) ([]core.Record, error)
}

// maxLineLen caps the buffered partial line. A stream with no terminator
// (binary data, wrong baud rate) is force-emitted in chunks of this size
// instead of growing without bound.
const maxLineLen = 64 * 1024

// LineJSON is a normalizer that wraps each line in a minimal JSON record with a
// "line" payload field. It is the default normalizer for interactive captures.
//
// Sources such as serial and TCP deliver arbitrary read chunks, not lines, so
// LineJSON buffers input and emits one record per terminated line. LF, CRLF
// and a lone CR all terminate a line and are stripped from the stored line.
// Lines longer than maxLineLen are split.
//
// Each record's ts is the arrival time of the line's first byte, so a line
// split across several reads keeps the time it started arriving.
type LineJSON struct {
	buf       []byte
	lineStart time.Time // arrival time of buf's first byte
	skipLF    bool      // previous byte was CR; swallow an immediately following LF
}

// NewLineJSON returns a new LineJSON normalizer.
func NewLineJSON() *LineJSON { return &LineJSON{} }

// Init resets any buffered partial line.
func (l *LineJSON) Init(*core.Run) error {
	l.buf = l.buf[:0]
	l.skipLF = false
	return nil
}

// Ingest scans b, treating it as arriving now.
func (l *LineJSON) Ingest(b []byte) ([]core.Record, error) {
	return l.IngestAt(b, time.Now().UTC())
}

// IngestAt scans b, which arrived at at, and returns a record for each
// complete line.
func (l *LineJSON) IngestAt(b []byte, at time.Time) ([]core.Record, error) {
	var records []core.Record
	emit := func() {
		start := l.lineStart
		if len(l.buf) == 0 {
			start = at // empty line: it arrived with its terminator
		}
		records = append(records, lineRecord(l.buf, start))
		l.buf = l.buf[:0] // reuse backing array; lineRecord copies into a string
	}
	for _, c := range b {
		if l.skipLF {
			l.skipLF = false
			if c == '\n' {
				continue
			}
		}
		switch c {
		case '\n':
			emit()
		case '\r':
			emit()
			l.skipLF = true
		default:
			if len(l.buf) == 0 {
				l.lineStart = at
			}
			l.buf = append(l.buf, c)
			if len(l.buf) >= maxLineLen {
				emit()
			}
		}
	}
	return records, nil
}

// Flush returns a record for any buffered partial line and clears the buffer.
func (l *LineJSON) Flush() []core.Record {
	l.skipLF = false
	if len(l.buf) == 0 {
		return nil
	}
	rec := lineRecord(l.buf, l.lineStart)
	l.buf = l.buf[:0]
	return []core.Record{rec}
}

func lineRecord(line []byte, at time.Time) core.Record {
	return core.Record{
		TS:      at.UTC(),
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
