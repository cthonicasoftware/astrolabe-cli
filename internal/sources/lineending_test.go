package sources

import (
	"bytes"
	"testing"
)

func TestLineEndingBytes(t *testing.T) {
	tests := []struct {
		le   LineEnding
		want []byte
	}{
		{LineEndingLF, []byte("\n")},
		{LineEndingCRLF, []byte("\r\n")},
		{LineEndingCR, []byte("\r")},
		{LineEndingNone, nil},
		{LineEnding("bogus"), []byte("\n")},
	}
	for _, tt := range tests {
		if got := tt.le.Bytes(); !bytes.Equal(got, tt.want) {
			t.Errorf("%q.Bytes() = %q, want %q", tt.le, got, tt.want)
		}
	}
}

func TestLineEndingNextCycles(t *testing.T) {
	le := LineEndingLF
	want := []LineEnding{LineEndingCRLF, LineEndingCR, LineEndingNone, LineEndingLF}
	for _, w := range want {
		le = le.Next()
		if le != w {
			t.Fatalf("Next() = %q, want %q", le, w)
		}
	}
	if got := LineEnding("bogus").Next(); got != DefaultLineEnding {
		t.Errorf("unknown Next() = %q, want %q", got, DefaultLineEnding)
	}
}

func TestParseLineEnding(t *testing.T) {
	tests := map[string]LineEnding{
		"lf":    LineEndingLF,
		"CRLF":  LineEndingCRLF,
		" cr ":  LineEndingCR,
		"none":  LineEndingNone,
		"":      DefaultLineEnding,
		"weird": DefaultLineEnding,
	}
	for in, want := range tests {
		if got := ParseLineEnding(in); got != want {
			t.Errorf("ParseLineEnding(%q) = %q, want %q", in, got, want)
		}
	}
}
