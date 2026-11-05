package sources

import (
	"errors"
	"testing"

	"go.bug.st/serial/enumerator"
)

func fakePorts(t *testing.T, ports ...*enumerator.PortDetails) {
	t.Helper()
	old := listPorts
	listPorts = func(...func(string, string) bool) ([]*enumerator.PortDetails, error) { return ports, nil }
	t.Cleanup(func() { listPorts = old })
}

func TestConfig_IdentityFromSerialNumber(t *testing.T) {
	fakePorts(t,
		&enumerator.PortDetails{Name: "/dev/ttyACM0", IsUSB: true, SerialNumber: "E66138"},
		&enumerator.PortDetails{Name: "/dev/ttyACM2", IsUSB: true},
	)
	if got := (Config{Port: "/dev/ttyACM0"}).Identity(); got != "E66138" {
		t.Errorf("Identity = %q, want E66138", got)
	}
	if got := (Config{Port: "/dev/ttyACM9"}).Identity(); got != "" {
		t.Errorf("unlisted port Identity = %q, want empty", got)
	}
	if got := (Config{Port: "/dev/ttyACM2"}).Identity(); got != "" {
		t.Errorf("no-serial port Identity = %q, want empty", got)
	}
}

func TestConfig_IdentityListError(t *testing.T) {
	old := listPorts
	listPorts = func(...func(string, string) bool) ([]*enumerator.PortDetails, error) { return nil, errors.New("boom") }
	t.Cleanup(func() { listPorts = old })
	if got := (Config{Port: "/dev/ttyACM0"}).Identity(); got != "" {
		t.Errorf("Identity = %q, want empty", got)
	}
	if _, ok := (Config{Port: "/dev/ttyACM0"}).Relocate("X"); ok {
		t.Error("Relocate ok on list error")
	}
}

func TestConfig_RelocateFindsNewName(t *testing.T) {
	fakePorts(t, &enumerator.PortDetails{Name: "/dev/ttyACM1", IsUSB: true, SerialNumber: "E66138"})
	c := Config{Port: "/dev/ttyACM0", Baud: 57600, Parity: "E", DataBits: 7, StopBits: "2", FlowControl: "hardware"}
	spec, ok := c.Relocate("E66138")
	if !ok {
		t.Fatal("Relocate = false, want true")
	}
	want := c
	want.Port = "/dev/ttyACM1"
	if got, _ := spec.(Config); got != want {
		t.Errorf("relocated = %+v, want %+v", spec, want)
	}
	if _, ok := c.Relocate("OTHER"); ok {
		t.Error("Relocate matched an unknown serial")
	}
	if _, ok := c.Relocate(""); ok {
		t.Error("Relocate matched an empty id")
	}
}

func TestConfig_RelocateIgnoresVIDPIDOnlyMatch(t *testing.T) {
	fakePorts(t,
		&enumerator.PortDetails{Name: "/dev/ttyACM1", IsUSB: true, VID: "2341", PID: "0043", SerialNumber: "OTHER"},
		&enumerator.PortDetails{Name: "/dev/ttyACM2", IsUSB: true, VID: "2341", PID: "0043"},
	)
	if _, ok := (Config{Port: "/dev/ttyACM0"}).Relocate("E66138"); ok {
		t.Error("Relocate matched on VID/PID alone")
	}
}

func TestConfig_RelocateAmbiguousSerialDeclines(t *testing.T) {
	fakePorts(t,
		&enumerator.PortDetails{Name: "/dev/ttyACM1", IsUSB: true, SerialNumber: "E66138"},
		&enumerator.PortDetails{Name: "/dev/ttyACM3", IsUSB: true, SerialNumber: "E66138"},
	)
	if _, ok := (Config{Port: "/dev/ttyACM0"}).Relocate("E66138"); ok {
		t.Error("Relocate followed an ambiguous serial")
	}
}

func TestConfig_IdentityAmbiguousSerialIsEmpty(t *testing.T) {
	fakePorts(t,
		&enumerator.PortDetails{Name: "/dev/ttyACM0", IsUSB: true, SerialNumber: "E66138"},
		&enumerator.PortDetails{Name: "/dev/ttyACM1", IsUSB: true, SerialNumber: "E66138"},
	)
	if got := (Config{Port: "/dev/ttyACM0"}).Identity(); got != "" {
		t.Errorf("Identity = %q, want empty", got)
	}
}
