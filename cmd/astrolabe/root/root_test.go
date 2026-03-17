package root

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestInitConfig_SetsErrorForInvalidExplicitConfig(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "connection.yml")
	if err := os.WriteFile(configPath, []byte("api_url: [broken"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	previousCfgFile := cfgFile
	previousInitErr := initConfigErr
	t.Cleanup(func() {
		cfgFile = previousCfgFile
		initConfigErr = previousInitErr
	})

	cfgFile = configPath
	initConfigErr = nil

	initConfig()

	if initConfigErr == nil {
		t.Fatal("expected initConfigErr for invalid config")
	}
}

func TestInitConfig_AllowsMissingDefaultConfig(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	previousCfgFile := cfgFile
	previousInitErr := initConfigErr
	t.Cleanup(func() {
		cfgFile = previousCfgFile
		initConfigErr = previousInitErr
	})

	cfgFile = ""
	initConfigErr = nil

	initConfig()

	if initConfigErr != nil {
		t.Fatalf("expected no error for missing default config, got %v", initConfigErr)
	}
}
