package tui

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestFormatHexRowMatchesHexdumpLayout(t *testing.T) {
	got := formatHexRow(0, []byte("Hello\r\n\x00\x01\x02ABCDEF"), 16)
	want := "00000000  48 65 6c 6c 6f 0d 0a 00  01 02 41 42 43 44 45 46  |Hello.....ABCDEF|"
	if got != want {
		t.Fatalf("got\n%q\nwant\n%q", got, want)
	}
	if len(got) != hexRowWidth(16) {
		t.Errorf("row width %d, hexRowWidth(16) = %d", len(got), hexRowWidth(16))
	}
}

func TestFormatHexRowPadsShortRow(t *testing.T) {
	full := formatHexRow(0, bytes.Repeat([]byte{'a'}, 8), 8)
	short := formatHexRow(0x10, []byte{0xff}, 8)
	if strings.Index(short, "|") != strings.Index(full, "|") {
		t.Fatalf("ASCII column misaligned:\n%q\n%q", full, short)
	}
	if !strings.HasPrefix(short, "00000010  ff ") || !strings.HasSuffix(short, "|.|") {
		t.Errorf("short row = %q", short)
	}
}

func TestHexBytesPerRowFitsWidth(t *testing.T) {
	cases := map[int]int{200: 16, 78: 16, 77: 8, 45: 8, 44: 4, 10: 4}
	for width, want := range cases {
		if got := hexBytesPerRow(width); got != want {
			t.Errorf("hexBytesPerRow(%d) = %d, want %d", width, got, want)
		}
	}
	for _, n := range []int{4, 8, 16} {
		if got := len(formatHexRow(0, make([]byte, n), n)); got != hexRowWidth(n) {
			t.Errorf("perRow %d: row is %d wide, hexRowWidth says %d", n, got, hexRowWidth(n))
		}
	}
}

func TestHexLogJoinsChunksIntoRows(t *testing.T) {
	t1, t2 := testAt, testAt.Add(time.Second)
	h := NewHexLog()
	h.AddData([]byte("abc"), t1)
	h.AddData([]byte("defgh"), t2)

	rows := h.Rows(4)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2: %+v", len(rows), rows)
	}
	if !strings.HasPrefix(rows[0].text, "00000000  61 62 63 64") || !rows[0].at.Equal(t1) {
		t.Errorf("row 0 = %+v, want bytes abcd from t1", rows[0])
	}
	if !strings.HasPrefix(rows[1].text, "00000004  65 66 67 68") || !rows[1].at.Equal(t2) {
		t.Errorf("row 1 = %+v, want bytes efgh from t2", rows[1])
	}
}

func TestHexLogEchoBreaksRowAndOffsetsContinue(t *testing.T) {
	h := NewHexLog()
	h.AddData([]byte("ab"), testAt)
	h.AddEcho("AT", testAt)
	h.AddMarker("── link down ──", testAt)
	h.AddData([]byte("cd"), testAt)

	rows := h.Rows(16)
	if len(rows) != 4 {
		t.Fatalf("rows = %d, want 4: %+v", len(rows), rows)
	}
	if rows[1].kind != lineEcho || rows[1].text != EchoPrefix+"AT" {
		t.Errorf("row 1 = %+v, want echo", rows[1])
	}
	if rows[2].kind != lineMarker {
		t.Errorf("row 2 = %+v, want marker", rows[2])
	}
	if !strings.HasPrefix(rows[3].text, "00000002  63 64") {
		t.Errorf("row 3 = %q, want offset 2 after the echo", rows[3].text)
	}
}

func TestHexLogTrimKeepsNewestBytesAndOffsets(t *testing.T) {
	h := NewHexLog()
	h.AddEcho("old", testAt)
	h.AddData(make([]byte, MaxHexBytes), testAt)
	h.AddData([]byte{0xaa, 0xbb}, testAt)

	if h.held != MaxHexBytes || h.Total() != MaxHexBytes+2 {
		t.Fatalf("held=%d total=%d", h.held, h.Total())
	}
	rows := h.Rows(16)
	if rows[0].kind != lineData || !strings.HasPrefix(rows[0].text, "00000002  ") {
		t.Errorf("first row = %+v, want data from offset 2 (echo trimmed)", rows[0])
	}
	if last := rows[len(rows)-1].text; !strings.Contains(last, "aa bb") {
		t.Errorf("last row = %q, want newest bytes", last)
	}
}

func TestHexLogClearKeepsCountingOffsets(t *testing.T) {
	h := NewHexLog()
	h.AddData([]byte("abc"), testAt)
	h.Clear()
	if len(h.Rows(16)) != 0 {
		t.Fatal("rows after Clear")
	}
	h.AddData([]byte("d"), testAt)
	if rows := h.Rows(16); !strings.HasPrefix(rows[0].text, "00000003  64") {
		t.Errorf("row after clear = %q, want offset 3", rows[0].text)
	}
}

func TestParseHex(t *testing.T) {
	want := []byte{0x01, 0xff, 0x0a}
	for _, in := range []string{"01ff0a", "01 FF 0a", "0x01,0xff, 0x0A", "  01\tff 0a "} {
		got, err := parseHex(in)
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("parseHex(%q) = % x, %v; want % x", in, got, err, want)
		}
	}
	for _, in := range []string{"1", "01 f", "zz"} {
		if _, err := parseHex(in); err == nil {
			t.Errorf("parseHex(%q) should fail", in)
		}
	}
	if got, err := parseHex(""); err != nil || len(got) != 0 {
		t.Errorf("parseHex(\"\") = %v, %v; want empty", got, err)
	}
}
