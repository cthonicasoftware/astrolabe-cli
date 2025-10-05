package telemetry

import (
	"testing"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/config"
)

func TestInitFromConfig_Disabled(t *testing.T) {
	cfg := config.Config{
		Telemetry: config.TelemetryCfg{
			Enabled: false,
			Backend: "expvar",
		},
	}

	InitFromConfig(cfg)

	if _, ok := Current().(*NoopMetrics); !ok {
		t.Error("InitFromConfig with disabled telemetry should set NoopMetrics")
	}
}

func TestInitFromConfig_ExpvarBackend(t *testing.T) {
	cfg := config.Config{
		Telemetry: config.TelemetryCfg{
			Enabled: true,
			Backend: "expvar",
		},
	}

	InitFromConfig(cfg)

	if _, ok := Current().(*ExpvarMetrics); !ok {
		t.Error("InitFromConfig with expvar backend should set ExpvarMetrics")
	}
}

func TestInitFromConfig_PrometheusBackend(t *testing.T) {
	cfg := config.Config{
		Telemetry: config.TelemetryCfg{
			Enabled: true,
			Backend: "prometheus",
		},
	}

	InitFromConfig(cfg)

	if _, ok := Current().(*PrometheusMetrics); !ok {
		t.Error("InitFromConfig with prometheus backend should set PrometheusMetrics")
	}
}

func TestInitFromConfig_NoneBackend(t *testing.T) {
	cfg := config.Config{
		Telemetry: config.TelemetryCfg{
			Enabled: true,
			Backend: "none",
		},
	}

	InitFromConfig(cfg)

	if _, ok := Current().(*NoopMetrics); !ok {
		t.Error("InitFromConfig with none backend should set NoopMetrics")
	}
}

func TestInitFromConfig_EmptyBackend(t *testing.T) {
	cfg := config.Config{
		Telemetry: config.TelemetryCfg{
			Enabled: true,
			Backend: "",
		},
	}

	InitFromConfig(cfg)

	if _, ok := Current().(*NoopMetrics); !ok {
		t.Error("InitFromConfig with empty backend should set NoopMetrics")
	}
}

func TestInitFromConfig_UnknownBackend(t *testing.T) {
	cfg := config.Config{
		Telemetry: config.TelemetryCfg{
			Enabled: true,
			Backend: "unknown-backend",
		},
	}

	InitFromConfig(cfg)

	if _, ok := Current().(*NoopMetrics); !ok {
		t.Error("InitFromConfig with unknown backend should fallback to NoopMetrics")
	}
}

func TestInitFromConfig_Integration(t *testing.T) {
	// Test that metrics work after initialization
	cfg := config.Config{
		Telemetry: config.TelemetryCfg{
			Enabled: true,
			Backend: "expvar",
		},
	}

	InitFromConfig(cfg)

	before := Snapshot()
	beforeCaptures := before["captures"].(map[string]interface{})["total"].(int64)

	// Record some metrics
	RecordCapture("test")
	RecordBytes("ingested", 100)
	RecordUpload(true, 50)
	RecordError("test_error")

	snapshot := Snapshot()
	if snapshot == nil {
		t.Error("Snapshot should not be nil after InitFromConfig")
	}

	captures := snapshot["captures"].(map[string]interface{})
	if captures["total"].(int64)-beforeCaptures != 1 {
		t.Errorf("captures delta after init = %v, want 1", captures["total"].(int64)-beforeCaptures)
	}
}
