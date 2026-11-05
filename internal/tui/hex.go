package tui

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// minHexTextWidth is the narrowest the text column may get to make room for
// a wider hex column.
const minHexTextWidth = 20

// hexColumnWidth is the width of a hex row with perRow bytes, e.g. "68 69 0d 0a".
func hexColumnWidth(perRow int) int { return perRow*3 - 1 }

// hexBytesPerRow picks the widest hex column (16, 8 or 4 bytes per row) that
// leaves at least minHexTextWidth for text, out of width cells.
func hexBytesPerRow(width int) int {
	for _, n := range []int{16, 8} {
		if width-hexColumnWidth(n)-1 >= minHexTextWidth {
			return n
		}
	}
	return 4
}

// formatHexRows splits raw into rows of perRow space-separated hex bytes.
func formatHexRows(raw []byte, perRow int) []string {
	var rows []string
	for len(raw) > 0 {
		n := min(perRow, len(raw))
		rows = append(rows, fmt.Sprintf("% x", raw[:n]))
		raw = raw[n:]
	}
	return rows
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
	return "hex " + fmt.Sprintf("% x", b)
}
