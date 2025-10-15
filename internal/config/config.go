package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type UploadCfg struct {
	BatchBytes int64
	MaxRetries int
}

type TelemetryCfg struct {
	Enabled bool   // opt-in for anonymous metrics
	Backend string // "expvar" (default), "prometheus", or "none"
}

type Config struct {
	APIURL       string
	ProjectID    string
	AuthToken    string
	OfflineCache string
	Upload       UploadCfg
	Telemetry    TelemetryCfg
}

func Load() Config {
	_ = viper.ReadInConfig()
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
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
