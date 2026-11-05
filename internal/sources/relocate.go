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

// serialCount counts listed USB ports reporting serial id.
func serialCount(ports []*enumerator.PortDetails, id string) int {
	n := 0
	for _, p := range ports {
		if p != nil && p.IsUSB && p.SerialNumber == id {
			n++
		}
	}
	return n
}

// Identity implements Relocator using the port's USB serial number. It
// returns "" when the port is not listed, has no serial number, or shares its
// serial with other ports (composite devices), where following is ambiguous.
func (c Config) Identity() string {
	ports, err := listPorts()
	if err != nil {
		return ""
	}
	for _, p := range ports {
		if p != nil && p.Name == c.Port && p.IsUSB {
			if p.SerialNumber == "" || serialCount(ports, p.SerialNumber) > 1 {
				return ""
			}
			return p.SerialNumber
		}
	}
	return ""
}

// Relocate implements Relocator, matching on USB serial number only (never
// VID/PID, which identical devices share). It declines when more than one
// port reports id, since a wrong-device run is worse than one that ends.
func (c Config) Relocate(id string) (Spec, bool) {
	if id == "" {
		return nil, false
	}
	ports, err := listPorts()
	if err != nil || serialCount(ports, id) != 1 {
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
