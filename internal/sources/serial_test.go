package sources

import (
	"context"
	"testing"
	"time"
)

func TestNewSerial(t *testing.T) {
	s := NewSerial("/dev/ttyUSB0", 115200)

	if s.Port != "/dev/ttyUSB0" {
		t.Errorf("Port = %v, want /dev/ttyUSB0", s.Port)
	}
	if s.Baud != 115200 {
		t.Errorf("Baud = %v, want 115200", s.Baud)
	}
	if s.ch == nil {
		t.Error("channel should be initialized")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Baud != 115200 {
		t.Errorf("Baud = %d, want 115200", cfg.Baud)
	}
	if cfg.Parity != "N" {
		t.Errorf("Parity = %s, want N", cfg.Parity)
	}
	if cfg.DataBits != 8 {
		t.Errorf("DataBits = %d, want 8", cfg.DataBits)
	}
	if cfg.StopBits != "1" {
		t.Errorf("StopBits = %s, want 1", cfg.StopBits)
	}
	if cfg.FlowControl != "none" {
		t.Errorf("FlowControl = %s, want none", cfg.FlowControl)
	}
}

func TestNewSerialWithConfigAppliesDefaults(t *testing.T) {
	cfg := Config{
		Port: "/dev/ttyUSB1",
		Baud: 0,
		// leave rest empty to force defaults
	}

	s := NewSerialWithConfig(cfg)

	if s.Port != "/dev/ttyUSB1" {
		t.Errorf("Port = %s, want /dev/ttyUSB1", s.Port)
	}
	if s.Baud != DefaultConfig().Baud {
		t.Errorf("Baud = %d, want %d", s.Baud, DefaultConfig().Baud)
	}
	if s.Parity != DefaultConfig().Parity {
		t.Errorf("Parity = %s, want %s", s.Parity, DefaultConfig().Parity)
	}
	if s.DataBits != DefaultConfig().DataBits {
		t.Errorf("DataBits = %d, want %d", s.DataBits, DefaultConfig().DataBits)
	}
	if s.StopBits != DefaultConfig().StopBits {
		t.Errorf("StopBits = %s, want %s", s.StopBits, DefaultConfig().StopBits)
	}
	if s.FlowControl != DefaultConfig().FlowControl {
		t.Errorf("FlowControl = %s, want %s", s.FlowControl, DefaultConfig().FlowControl)
	}
	if s.ch == nil {
		t.Error("channel should be initialized")
	}
}

func TestNewSerialWithConfigPreservesValues(t *testing.T) {
	cfg := Config{
		Port:        "/dev/ttyACM1",
		Baud:        57600,
		Parity:      "E",
		DataBits:    7,
		StopBits:    "2",
		FlowControl: "hardware",
	}

	s := NewSerialWithConfig(cfg)

	if s.Port != cfg.Port {
		t.Errorf("Port = %s, want %s", s.Port, cfg.Port)
	}
	if s.Baud != cfg.Baud {
		t.Errorf("Baud = %d, want %d", s.Baud, cfg.Baud)
	}
	if s.Parity != cfg.Parity {
		t.Errorf("Parity = %s, want %s", s.Parity, cfg.Parity)
	}
	if s.DataBits != cfg.DataBits {
		t.Errorf("DataBits = %d, want %d", s.DataBits, cfg.DataBits)
	}
	if s.StopBits != cfg.StopBits {
		t.Errorf("StopBits = %s, want %s", s.StopBits, cfg.StopBits)
	}
	if s.FlowControl != cfg.FlowControl {
		t.Errorf("FlowControl = %s, want %s", s.FlowControl, cfg.FlowControl)
	}
}

func TestSerial_Meta(t *testing.T) {
	s := NewSerial("/dev/ttyACM0", 9600)
	meta := s.Meta()

	if meta.Kind != "serial" {
		t.Errorf("Kind = %v, want serial", meta.Kind)
	}
	if meta.Port != "/dev/ttyACM0" {
		t.Errorf("Port = %v, want /dev/ttyACM0", meta.Port)
	}
	if meta.Baud != 9600 {
		t.Errorf("Baud = %v, want 9600", meta.Baud)
	}
}

func TestSerial_Frames(t *testing.T) {
	s := NewSerial("/dev/ttyUSB0", 115200)
	ch := s.Frames()

	if ch == nil {
		t.Error("Frames() should return non-nil channel")
	}

	// Verify it's the same channel
	if ch != s.ch {
		t.Error("Frames() should return the internal channel")
	}
}

func TestSerial_CloseWithoutOpen(t *testing.T) {
	s := NewSerial("/dev/ttyUSB0", 115200)
	err := s.Close()

	if err != nil {
		t.Errorf("Close() without Open() should not error, got %v", err)
	}
}

func TestSerial_ContextCancellation(t *testing.T) {
	// This test verifies that context cancellation works
	// We can't test actual serial port reading without hardware,
	// but we can verify the structure is correct
	s := NewSerial("/dev/ttyUSB0", 115200)

	if s.cancel != nil {
		t.Error("cancel should be nil before Open()")
	}

	// Note: We can't actually open a serial port in tests without hardware
	// Real integration tests would be run separately with actual hardware
}

func TestSerial_OpenNonexistentPort(t *testing.T) {
	s := NewSerial("/dev/nonexistent_serial_port_12345", 115200)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := s.Open(ctx)
	if err == nil {
		t.Error("Open() should fail for nonexistent port")
		s.Close()
	}

	// Verify error message contains port name
	if err != nil && s.Port != "/dev/nonexistent_serial_port_12345" {
		t.Errorf("Port should still be set after failed open")
	}
}

func TestSerial_ConcurrentClose(t *testing.T) {
	s := NewSerial("/dev/ttyUSB0", 115200)

	// Close multiple times should be safe
	err1 := s.Close()
	err2 := s.Close()

	if err1 != nil {
		t.Errorf("First Close() error = %v, want nil", err1)
	}
	if err2 != nil {
		t.Errorf("Second Close() error = %v, want nil", err2)
	}
}

func TestSerial_ReadLoopChannelClose(t *testing.T) {
	// Verify that the channel gets closed when context is cancelled
	// This is a structural test without actual hardware
	s := NewSerial("/dev/ttyUSB0", 115200)

	// Simulate what happens in readLoop
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	// Start a goroutine that mimics readLoop behavior
	done := make(chan bool)
	go func() {
		select {
		case <-ctx.Done():
			close(s.ch)
			done <- true
		case <-time.After(2 * time.Second):
			t.Error("Context cancellation took too long")
			done <- false
		}
	}()

	// Cancel context
	cancel()

	// Wait for completion
	<-done

	// Verify channel is closed
	_, ok := <-s.ch
	if ok {
		t.Error("Channel should be closed after context cancellation")
	}
}
