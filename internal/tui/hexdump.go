package tui

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

// MaxHexBytes is how many received bytes the hex view keeps: as many 16-byte
// rows as the text view keeps lines.
const MaxHexBytes = MaxBufferedLines * 16

// HexLog keeps the received bytes, unchanged, for the hex view, with the
// echoes and markers that arrived between them. LineBuffer cannot serve here
// because it strips \r and splits on \n.
type HexLog struct {
	entries []hexEntry
	total   int64 // bytes received since the capture started
	held    int   // data bytes currently kept in entries
}

type hexEntry struct {
	kind lineKind
	data []byte // lineData only
	off  int64  // stream offset of data[0]
	text string // echoes and markers only
	at   time.Time
}

// viewLine is one line of the live view before wrapping: a text line or
// dump row, an echo, or a marker.
type viewLine struct {
	kind lineKind
	text string
	at   time.Time
}

// NewHexLog creates an empty hex log.
func NewHexLog() *HexLog { return &HexLog{} }

// AddData records bytes that arrived at at.
func (h *HexLog) AddData(data []byte, at time.Time) {
	if len(data) == 0 {
		return
	}
	h.entries = append(h.entries, hexEntry{kind: lineData, data: data, off: h.total, at: at})
	h.total += int64(len(data))
	h.held += len(data)
	h.trim()
}

// AddEcho records a send shown as text between dump rows.
func (h *HexLog) AddEcho(text string, at time.Time) {
	h.entries = append(h.entries, hexEntry{kind: lineEcho, text: EchoPrefix + text, at: at})
}

// AddMarker records a status line, such as a link drop.
func (h *HexLog) AddMarker(text string, at time.Time) {
	h.entries = append(h.entries, hexEntry{kind: lineMarker, text: text, at: at})
}

// Total returns how many bytes have been received, including cleared ones.
func (h *HexLog) Total() int64 { return h.total }

// Clear drops what is shown. Offsets keep counting from the stream start so
// they still match the device's byte stream.
func (h *HexLog) Clear() {
	h.entries = nil
	h.held = 0
}

// trim drops the oldest bytes, and any echoes or markers before them, once
// more than MaxHexBytes are held.
func (h *HexLog) trim() {
	for h.held > MaxHexBytes && len(h.entries) > 0 {
		e := &h.entries[0]
		if e.kind != lineData {
			h.entries = h.entries[1:]
			continue
		}
		drop := h.held - MaxHexBytes
		if drop < len(e.data) {
			e.data = e.data[drop:]
			e.off += int64(drop)
			h.held -= drop
			return
		}
		h.held -= len(e.data)
		h.entries = h.entries[1:]
	}
}

// Rows lays out the log with perRow bytes per dump row. Bytes from separate
// chunks share rows; an echo or marker ends the current row, and the dump
// resumes on a new row at the next offset. A row's time is when its first
// byte arrived.
func (h *HexLog) Rows(perRow int) []viewLine {
	var rows []viewLine
	var row []byte
	var rowOff int64
	var rowAt time.Time
	flush := func() {
		if len(row) > 0 {
			rows = append(rows, viewLine{kind: lineData, text: formatHexRow(rowOff, row, perRow), at: rowAt})
			row = row[:0]
		}
	}
	for _, e := range h.entries {
		if e.kind != lineData {
			flush()
			rows = append(rows, viewLine{kind: e.kind, text: e.text, at: e.at})
			continue
		}
		for i, b := range e.data {
			if len(row) == 0 {
				rowOff, rowAt = e.off+int64(i), e.at
			}
			row = append(row, b)
			if len(row) == perRow {
				flush()
			}
		}
	}
	flush()
	return rows
}

// hexRowWidth is the width of a dump row with perRow bytes, e.g. for 16:
// "00000000  48 65 6c 6c 6f 0a 00 01  02 03 04 05 06 07 08 09  |Hello...........|"
func hexRowWidth(perRow int) int {
	w := 8 + 2 + perRow*3 - 1 + 2 + 1 + perRow + 1
	if perRow == 16 {
		w++ // gap between the two halves
	}
	return w
}

// hexBytesPerRow picks the widest row (16, 8 or 4 bytes) that fits width.
func hexBytesPerRow(width int) int {
	for _, n := range []int{16, 8} {
		if hexRowWidth(n) <= width {
			return n
		}
	}
	return 4
}

// formatHexRow renders offset, hex bytes and ASCII column, like hexdump -C.
// A short row is padded so its ASCII column lines up with full rows.
func formatHexRow(off int64, b []byte, perRow int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%08x  ", off)
	for i := range perRow {
		if i == 8 && perRow == 16 {
			sb.WriteByte(' ')
		}
		if i < len(b) {
			fmt.Fprintf(&sb, "%02x", b[i])
		} else {
			sb.WriteString("  ")
		}
		if i < perRow-1 {
			sb.WriteByte(' ')
		}
	}
	sb.WriteString("  |")
	for _, c := range b {
		if c >= 0x20 && c < 0x7f {
			sb.WriteByte(c)
		} else {
			sb.WriteByte('.')
		}
	}
	sb.WriteByte('|')
	return sb.String()
}

// parseHex reads bytes typed as hex. Pairs may be run together or split by
// spaces or commas, and each group may start with 0x: "01FF0a",
// "01 ff 0a" and "0x01,0xff,0x0a" all give 01 ff 0a.
func parseHex(s string) ([]byte, error) {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == ','
	})
	var digits strings.Builder
	for _, f := range fields {
		f = strings.TrimPrefix(strings.TrimPrefix(f, "0x"), "0X")
		if len(f)%2 != 0 {
			return nil, fmt.Errorf("invalid hex %q: odd number of digits", f)
		}
		digits.WriteString(f)
	}
	b, err := hex.DecodeString(digits.String())
	if err != nil {
		var inv hex.InvalidByteError
		if errors.As(err, &inv) {
			return nil, fmt.Errorf("invalid hex: %q is not a hex digit", rune(inv))
		}
		return nil, fmt.Errorf("invalid hex: %w", err)
	}
	return b, nil
}

// formatHexEcho shows sent bytes in the echo line, e.g. "hex 01 ff 0a".
func formatHexEcho(b []byte) string {
	return "hex " + strings.TrimSpace(fmt.Sprintf("% x", b))
}
