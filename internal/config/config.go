// Package config loads and exposes application configuration via Viper,
// including API connection settings, upload tuning, and telemetry options.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// UploadCfg controls batching and retry behaviour for the upload subsystem.
type UploadCfg struct {
	BatchBytes int64
	MaxRetries int
}

// TelemetryCfg controls the anonymous metrics collection backend.
type TelemetryCfg struct {
	Enabled bool   // opt-in for anonymous metrics
	Backend string // "expvar" (default), "prometheus", or "none"
}

// Config is the top-level application configuration populated from the
// connection.yml file, environment variables, and CLI flags.
type Config struct {
	APIURL       string
	ProjectID    string
	AuthToken    string
	OfflineCache string
	Upload       UploadCfg
	Telemetry    TelemetryCfg
}

// ReadInConfig configures Viper for Astrolabe and reads the active config file.
// Missing default config files are tolerated; explicit config paths must resolve cleanly.
func ReadInConfig(explicitPath string) error {
	if explicitPath != "" {
		viper.SetConfigFile(explicitPath)
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			viper.AddConfigPath(filepath.Join(home, ".astrolabe"))
		}
		viper.SetConfigName("connection")
		viper.SetConfigType("yaml")
	}
	viper.SetEnvPrefix("ASTROLABE")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if explicitPath == "" && errors.As(err, &notFound) {
			return nil
		}
		return fmt.Errorf("read config: %w", err)
	}
	return nil
}

// Load returns the active configuration from Viper.
// Missing values fall back to sensible defaults (e.g. ~/.astrolabe/runs for OfflineCache).
func Load() (Config, error) {
	home, _ := os.UserHomeDir()
	return Config{
		APIURL:       viper.GetString("api_url"),
		ProjectID:    viper.GetString("project_id"),
		AuthToken:    viper.GetString("auth_token"),
		OfflineCache: firstNonEmpty(viper.GetString("offline_cache"), filepath.Join(home, ".astrolabe", "runs")),
		Upload: UploadCfg{
			BatchBytes: viper.GetInt64("upload.batch_bytes"),
			MaxRetries: viper.GetInt("upload.max_retries"),
		},
		Telemetry: TelemetryCfg{
			Enabled: viper.GetBool("telemetry.enabled"),
			Backend: firstNonEmpty(viper.GetString("telemetry.backend"), "expvar"),
		},
	}, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
