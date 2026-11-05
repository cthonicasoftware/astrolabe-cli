package core

import "time"

// LinkState is the connection state a LinkEvent reports.
type LinkState string

const (
	LinkDown LinkState = "down"
	LinkUp   LinkState = "up"
)

// LinkEvent reports the source connection dropping or coming back.
type LinkEvent struct {
	State    LinkState
	Err      string        // LinkDown only
	Attempts int           // LinkUp only
	Gap      time.Duration // LinkUp only
	Port     string        // LinkUp only; set when the serial port name changed
}

// Frame is source data or, when Link is non-nil, a link event. At is the
// arrival (or event) time.
type Frame struct {
	Data []byte
	At   time.Time
	Link *LinkEvent
}
