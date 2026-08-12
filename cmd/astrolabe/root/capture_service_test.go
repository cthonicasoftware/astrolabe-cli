package root

import (
	"strings"
	"testing"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
	"github.com/cthonicasoftware/astrolabe-cli/internal/tui"
)

func TestCaptureServiceBuildComponentsSerial(t *testing.T) {
	svc := newCaptureService(nil, t.TempDir())
	serialCfg := &sources.Config{
		Port: "/dev/ttyUSB9",
		Baud: 230400,
	}

	components, err := svc.BuildComponents(CaptureRequest{
		Source:   *serialCfg,
		RunLabel: "bench-run",
		Meta: core.ManifestOptions{
			Operator:   "qa-operator",
			Attributes: map[string]string{"rack": "r2"},
		},
	})
	if err != nil {
		t.Fatalf("BuildComponents() error = %v", err)
	}

	if got := components.Manifest.Attributes["source_kind"]; got != "serial" {
		t.Fatalf("source_kind = %q, want serial", got)
	}
	if got := components.Manifest.Attributes["port"]; got != serialCfg.Port {
		t.Fatalf("port = %q, want %q", got, serialCfg.Port)
	}
	if got := components.Manifest.Attributes["baud"]; got != "230400" {
		t.Fatalf("baud = %q, want 230400", got)
	}
	if got := components.Manifest.Attributes["run_label"]; got != "bench-run" {
		t.Fatalf("run_label = %q, want bench-run", got)
	}
	if got := components.Manifest.Attributes["rack"]; got != "r2" {
		t.Fatalf("rack = %q, want r2", got)
	}
	if got := components.CaptureSettings.Notes; got != "serial capture from /dev/ttyUSB9 @ 230400 baud" {
		t.Fatalf("notes = %q", got)
	}
}

func TestCaptureServiceBuildComponentsTCP(t *testing.T) {
	svc := newCaptureService(nil, t.TempDir())
	tcpCfg := &sources.TCPConfig{
		Host:           "127.0.0.1",
		Port:           9100,
		ConnectTimeout: 3 * time.Second,
		ReadTimeout:    5 * time.Second,
		BufferSize:     8192,
	}

	components, err := svc.BuildComponents(CaptureRequest{
		Source: *tcpCfg,
	})
	if err != nil {
		t.Fatalf("BuildComponents() error = %v", err)
	}

	if got := components.Manifest.Attributes["source_kind"]; got != "tcp" {
		t.Fatalf("source_kind = %q, want tcp", got)
	}
	if got := components.Manifest.Attributes["host"]; got != "127.0.0.1" {
		t.Fatalf("host = %q, want 127.0.0.1", got)
	}
	if got := components.Manifest.Attributes["port"]; got != "9100" {
		t.Fatalf("port = %q, want 9100", got)
	}
	if got := components.Manifest.Attributes["connect_timeout"]; got != "3s" {
		t.Fatalf("connect_timeout = %q, want 3s", got)
	}
	if got := components.Manifest.Attributes["read_timeout"]; got != "5s" {
		t.Fatalf("read_timeout = %q, want 5s", got)
	}
	if got := components.Manifest.Attributes["buffer_size"]; got != "8192" {
		t.Fatalf("buffer_size = %q, want 8192", got)
	}
}

func TestCaptureServiceBuildComponentsRejectsInvalidRequests(t *testing.T) {
	svc := newCaptureService(nil, t.TempDir())

	_, err := svc.BuildComponents(CaptureRequest{})
	if err == nil {
		t.Fatal("BuildComponents() error = nil, want error for missing source")
	}

	// A source whose config cannot produce a usable source must surface the
	// failure, tagged with the source kind.
	_, err = svc.BuildComponents(CaptureRequest{
		Source: sources.TCPConfig{Host: "", Port: 9000},
	})
	if err == nil {
		t.Fatal("BuildComponents() error = nil, want error for invalid TCP config")
	}
	if !strings.Contains(err.Error(), "tcp") {
		t.Errorf("BuildComponents() error = %q, want it to name the source kind", err)
	}
}

func TestCaptureRequestFromConfig(t *testing.T) {
	req, err := captureRequestFromConfig(tui.CaptureConfig{
		SourceType: tui.SourceTypeTCP,
		TCPHost:    "localhost",
		TCPPort:    "1234",
	})
	if err != nil {
		t.Fatalf("captureRequestFromConfig() error = %v", err)
	}
	tcpCfg, ok := req.Source.(sources.TCPConfig)
	if !ok {
		t.Fatalf("req.Source = %T, want sources.TCPConfig", req.Source)
	}
	if tcpCfg.Port != 1234 {
		t.Fatalf("tcp request = %+v, want port 1234", tcpCfg)
	}
}
