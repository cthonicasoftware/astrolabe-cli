package normalize

import (
	"strings"
	"testing"
)

func lines(t *testing.T, l *LineJSON, chunks ...string) []string {
	t.Helper()
	var out []string
	for _, c := range chunks {
		recs, err := l.Ingest([]byte(c))
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range recs {
			out = append(out, r.Payload["line"].(string))
		}
	}
	for _, r := range l.Flush() {
		out = append(out, r.Payload["line"].(string))
	}
	return out
}

func TestLineJSON_Terminators(t *testing.T) {
	got := lines(t, NewLineJSON(), "a\rb\r", "\nc\r\nd\n", "e")
	want := []string{"a", "b", "c", "d", "e"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestLineJSON_MaxLineLen(t *testing.T) {
	got := lines(t, NewLineJSON(), strings.Repeat("x", maxLineLen+5))
	if len(got) != 2 || len(got[0]) != maxLineLen || len(got[1]) != 5 {
		t.Fatalf("unexpected split: %d records", len(got))
	}
}
