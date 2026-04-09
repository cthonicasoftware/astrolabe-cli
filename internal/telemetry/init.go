package telemetry

import (
	"github.com/cthonicasoftware/astrolabe-cli/internal/config"
)

// InitFromConfig initializes the global telemetry based on config settings.
// Call this early in your main() or application setup.
func InitFromConfig(cfg config.Config) {
	if !cfg.Telemetry.Enabled {
		Init(&NoopMetrics{})
		return
	}

	switch cfg.Telemetry.Backend {
	case "expvar":
		Init(NewExpvarMetrics())
	case "prometheus":
		Init(NewPrometheusMetrics())
	case "none", "":
		Init(&NoopMetrics{})
	default:
		// Unknown backend, fallback to noop
		Init(&NoopMetrics{})
	}
}
