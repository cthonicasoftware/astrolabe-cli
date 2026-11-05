package sources

import "go.bug.st/serial/enumerator"

// listPorts enumerates serial ports; tests replace it.
var listPorts = enumerator.GetDetailedPortsList

// Relocator is implemented by specs whose device can return under a new name.
type Relocator interface {
	// Identity returns a stable device ID for the current port, or "" if none.
	Identity() string
	// Relocate returns a spec for whichever port now reports id.
	Relocate(id string) (Spec, bool)
}

// Identity implements Relocator using the port's USB serial number. It
// returns "" when the port is not listed or has no serial number.
func (c Config) Identity() string {
	ports, err := listPorts()
	if err != nil {
		return ""
	}
	for _, p := range ports {
		if p != nil && p.Name == c.Port && p.IsUSB {
			return p.SerialNumber
		}
	}
	return ""
}

// Relocate implements Relocator, matching on USB serial number only (never
// VID/PID, which identical devices share).
func (c Config) Relocate(id string) (Spec, bool) {
	if id == "" {
		return nil, false
	}
	ports, err := listPorts()
	if err != nil {
		return nil, false
	}
	for _, p := range ports {
		if p != nil && p.IsUSB && p.SerialNumber == id {
			c.Port = p.Name
			return c, true
		}
	}
	return nil, false
}
