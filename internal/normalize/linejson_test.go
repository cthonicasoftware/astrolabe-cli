package normalize

import (
	"testing"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
)

func linesOf(t *testing.T, records []core.Record) []string {
	t.Helper()
	out := make([]string, 0, len(records))
	for _, r := range records {
		line, ok := r.Payload["line"].(string)
		if !ok {
			t.Fatalf("record payload missing string line: %#v", r.Payload)
		}
		out = append(out, line)
	}
	return out
}

func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestLineJSONReassemblesSplitChunks(t *testing.T) {
	// Serial ports often return the first byte of a line on its own.
	chunks := []string{"T", "emp: 22\r\n", "H", "um: 40\r", "\nPres", "s: 1013\nlast"}

	l := NewLineJSON()
	if err := l.Init(nil); err != nil {
		t.Fatalf("init: %v", err)
	}

	var got []string
	for _, c := range chunks {
		recs, err := l.Ingest([]byte(c))
		if err != nil {
			t.Fatalf("ingest %q: %v", c, err)
		}
		got = append(got, linesOf(t, recs)...)
	}

	want := []string{"Temp: 22", "Hum: 40", "Press: 1013"}
	if !equalLines(got, want) {
		t.Fatalf("lines = %q, want %q", got, want)
	}

	flushed := linesOf(t, l.Flush())
	if !equalLines(flushed, []string{"last"}) {
		t.Fatalf("flush = %q, want [\"last\"]", flushed)
	}
	if again := l.Flush(); len(again) != 0 {
		t.Fatalf("second flush returned %d records, want 0", len(again))
	}
}

func TestLineJSONKeepsEmptyLines(t *testing.T) {
	l := NewLineJSON()
	recs, err := l.Ingest([]byte("a\n\nb\n"))
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	got := linesOf(t, recs)
	want := []string{"a", "", "b"}
	if !equalLines(got, want) {
		t.Fatalf("lines = %q, want %q", got, want)
	}
}

func TestLineJSONInitClearsBuffer(t *testing.T) {
	l := NewLineJSON()
	if _, err := l.Ingest([]byte("stale")); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if err := l.Init(nil); err != nil {
		t.Fatalf("init: %v", err)
	}
	if recs := l.Flush(); len(recs) != 0 {
		t.Fatalf("flush after init returned %d records, want 0", len(recs))
	}
}
