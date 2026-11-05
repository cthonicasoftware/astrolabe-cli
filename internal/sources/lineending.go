package sources

import "strings"

// LineEnding is the terminator appended to each line an operator sends to a
// device.
type LineEnding string

// Supported line endings, in the order Next cycles through them.
const (
	LineEndingLF   LineEnding = "LF"
	LineEndingCRLF LineEnding = "CRLF"
	LineEndingCR   LineEnding = "CR"
	LineEndingNone LineEnding = "None"
)

// DefaultLineEnding matches what most microcontroller firmware expects.
const DefaultLineEnding = LineEndingLF

var lineEndingOrder = []LineEnding{LineEndingLF, LineEndingCRLF, LineEndingCR, LineEndingNone}

// Bytes returns the bytes written after a line. Unknown values act like LF.
func (le LineEnding) Bytes() []byte {
	switch le {
	case LineEndingCRLF:
		return []byte("\r\n")
	case LineEndingCR:
		return []byte("\r")
	case LineEndingNone:
		return nil
	default:
		return []byte("\n")
	}
}

// Next returns the following line ending in the cycle LF → CRLF → CR → None → LF.
func (le LineEnding) Next() LineEnding {
	for i, v := range lineEndingOrder {
		if v == le {
			return lineEndingOrder[(i+1)%len(lineEndingOrder)]
		}
	}
	return DefaultLineEnding
}

// ParseLineEnding reads a line ending from config text, ignoring case.
// Unknown or empty values return DefaultLineEnding.
func ParseLineEnding(s string) LineEnding {
	for _, v := range lineEndingOrder {
		if strings.EqualFold(string(v), strings.TrimSpace(s)) {
			return v
		}
	}
	return DefaultLineEnding
}
