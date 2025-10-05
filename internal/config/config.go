package config

import (
    "os"
    "path/filepath"

    "github.com/spf13/viper"
)

type UploadCfg struct {
    BatchBytes  int64
    MaxRetries  int
}

type Config struct {
    APIURL       string
    ProjectID    string
    AuthToken    string
    OfflineCache string
    Upload       UploadCfg
}

func Load() Config {
    _ = viper.ReadInConfig()
    home, _ := os.UserHomeDir()
    return Config{
        APIURL:       viper.GetString("api_url"),
        ProjectID:    viper.GetString("project_id"),
        AuthToken:    viper.GetString("auth_token"),
        OfflineCache: firstNonEmpty(viper.GetString("offline_cache"), filepath.Join(home, ".qa-agent", "runs")),
        Upload: UploadCfg{
            BatchBytes: viper.GetInt64("upload.batch_bytes"),
            MaxRetries: viper.GetInt("upload.max_retries"),
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
