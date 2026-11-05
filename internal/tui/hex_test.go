package tui

import (
	"bytes"
	"reflect"
	"testing"
)

func TestFormatHexRows(t *testing.T) {
	got := formatHexRows([]byte("hello\r\n"), 4)
	want := []string{"68 65 6c 6c", "6f 0d 0a"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	if formatHexRows(nil, 4) != nil {
		t.Error("no bytes should give no rows")
	}
	if got := formatHexRows(make([]byte, 16), 16); len(got) != 1 || len(got[0]) != hexColumnWidth(16) {
		t.Errorf("full row %q is not hexColumnWidth(16) = %d wide", got, hexColumnWidth(16))
	}
}

func TestHexBytesPerRowLeavesRoomForText(t *testing.T) {
	cases := map[int]int{200: 16, 68: 16, 67: 8, 44: 8, 43: 4, 10: 4}
	for width, want := range cases {
		if got := hexBytesPerRow(width); got != want {
			t.Errorf("hexBytesPerRow(%d) = %d, want %d", width, got, want)
		}
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
