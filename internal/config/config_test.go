package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func resetViper(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
}

func TestLoadDefaults(t *testing.T) {
	resetViper(t)
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	home, _ := os.UserHomeDir()
	wantCache := filepath.Join(home, ".astrolabe", "runs")
	if cfg.OfflineCache != wantCache {
		t.Fatalf("OfflineCache = %q, want %q", cfg.OfflineCache, wantCache)
	}
	if cfg.Telemetry.Backend != "expvar" {
		t.Fatalf("Telemetry.Backend = %q, want %q", cfg.Telemetry.Backend, "expvar")
	}
	if cfg.APIURL != "" {
		t.Fatalf("APIURL = %q, want empty", cfg.APIURL)
	}
	if cfg.Telemetry.Enabled {
		t.Fatal("Telemetry.Enabled should default to false")
	}
}

func TestLoadFromViperValues(t *testing.T) {
	resetViper(t)
	viper.Set("api_url", "https://api.example.com")
	viper.Set("project_id", "proj-1")
	viper.Set("auth_token", "tok-abc")
	viper.Set("offline_cache", "/tmp/cache")
	viper.Set("upload.batch_bytes", int64(1024))
	viper.Set("upload.max_retries", 5)
	viper.Set("telemetry.enabled", true)
	viper.Set("telemetry.backend", "prometheus")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.APIURL != "https://api.example.com" {
		t.Fatalf("APIURL = %q", cfg.APIURL)
	}
	if cfg.ProjectID != "proj-1" {
		t.Fatalf("ProjectID = %q", cfg.ProjectID)
	}
	if cfg.AuthToken != "tok-abc" {
		t.Fatalf("AuthToken = %q", cfg.AuthToken)
	}
	if cfg.OfflineCache != "/tmp/cache" {
		t.Fatalf("OfflineCache = %q", cfg.OfflineCache)
	}
	if cfg.Upload.BatchBytes != 1024 {
		t.Fatalf("Upload.BatchBytes = %d", cfg.Upload.BatchBytes)
	}
	if cfg.Upload.MaxRetries != 5 {
		t.Fatalf("Upload.MaxRetries = %d", cfg.Upload.MaxRetries)
	}
	if !cfg.Telemetry.Enabled {
		t.Fatal("Telemetry.Enabled should be true")
	}
	if cfg.Telemetry.Backend != "prometheus" {
		t.Fatalf("Telemetry.Backend = %q", cfg.Telemetry.Backend)
	}
}

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		vals []string
		want string
	}{
		{[]string{"a", "b"}, "a"},
		{[]string{"", "b"}, "b"},
		{[]string{"", ""}, ""},
		{[]string{}, ""},
		{[]string{"only"}, "only"},
	}
	for _, tc := range tests {
		got := firstNonEmpty(tc.vals...)
		if got != tc.want {
			t.Fatalf("firstNonEmpty(%v) = %q, want %q", tc.vals, got, tc.want)
		}
	}
}

func TestReadInConfig_InvalidExplicitFile(t *testing.T) {
	resetViper(t)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "connection.yml")
	if err := os.WriteFile(configPath, []byte("api_url: [broken"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := ReadInConfig(configPath); err == nil {
		t.Fatal("expected invalid config error, got nil")
	}
}

func TestReadInConfig_MissingDefaultFileAllowed(t *testing.T) {
	resetViper(t)
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	if err := ReadInConfig(""); err != nil {
		t.Fatalf("ReadInConfig() error = %v", err)
	}
}
