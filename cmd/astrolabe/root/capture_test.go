package root

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cthonicasoftware/astrolabe-cli/internal/config"
	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// TestCaptureCmd_PersistentMetadataFlags verifies that all shared metadata flags
// are registered on captureCmd.PersistentFlags() and thus inherited by all subcommands.
func TestCaptureCmd_PersistentMetadataFlags(t *testing.T) {
	expected := []string{
		"operator", "location",
		"device-id", "device-serial", "device-firmware", "device-firmware-hash", "device-hardware-version",
		"test-plan", "test-variant", "test-run",
		"tag", "attr",
	}
	for _, name := range expected {
		if captureCmd.PersistentFlags().Lookup(name) == nil {
			t.Errorf("persistent flag %q not found on captureCmd", name)
		}
	}
}

// TestCaptureSubcmds_NoLocalMetadataFlags verifies that metadata flags are not
// duplicated as local flags on each subcommand.
func TestCaptureSubcmds_NoLocalMetadataFlags(t *testing.T) {
	metadataFlags := []string{
		"operator", "location",
		"device-id", "device-serial", "device-firmware", "device-firmware-hash", "device-hardware-version",
		"test-plan", "test-variant", "test-run",
		"tag", "attr",
	}
	meta := &captureMetadataFlags{}
	cmds := map[string]interface{ LocalFlags() *pflag.FlagSet }{
		"serial": newCaptureSerialCmd(meta),
		"tcp":    newCaptureTCPCmd(meta),
		"file":   newCaptureFileCmd(meta),
	}
	for cmdName, cmd := range cmds {
		for _, flagName := range metadataFlags {
			if cmd.LocalFlags().Lookup(flagName) != nil {
				t.Errorf("%s: flag %q should not be a local flag (must be inherited from captureCmd)", cmdName, flagName)
			}
		}
	}
}

// TestApplyMetadataDefaults_CLIWins verifies that an explicitly set CLI flag takes
// precedence over the saved config value.
func TestApplyMetadataDefaults_CLIWins(t *testing.T) {
	saved := config.Metadata{
		Operator: "saved-operator",
		Location: "saved-location",
		Device: core.DeviceInfo{
			ID:              "saved-id",
			Serial:          "saved-serial",
			Firmware:        "saved-fw",
			FirmwareHash:    "saved-hash",
			HardwareVersion: "saved-hw",
		},
		Test: core.TestInfo{
			Plan:    "saved-plan",
			Variant: "saved-variant",
			Run:     "saved-run",
		},
	}

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	var op string
	fs.StringVar(&op, "operator", "", "")
	if err := fs.Parse([]string{"--operator", "cli-operator"}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	opts := core.ManifestOptions{Operator: "cli-operator"}
	applyMetadataDefaults(&opts, saved, fs)

	if opts.Operator != "cli-operator" {
		t.Errorf("operator: got %q, want %q", opts.Operator, "cli-operator")
	}
	// Unset fields should still get saved defaults
	if opts.Location != "saved-location" {
		t.Errorf("location: got %q, want %q", opts.Location, "saved-location")
	}
}

// TestApplyMetadataDefaults_SavedConfigFillsIn verifies that saved config fills in
// fields that were not set on the CLI.
func TestApplyMetadataDefaults_SavedConfigFillsIn(t *testing.T) {
	saved := config.Metadata{
		Operator: "saved-operator",
		Location: "saved-location",
		Device: core.DeviceInfo{
			ID:              "saved-id",
			Serial:          "saved-serial",
			Firmware:        "saved-fw",
			FirmwareHash:    "saved-hash",
			HardwareVersion: "saved-hw",
		},
		Test: core.TestInfo{
			Plan:    "saved-plan",
			Variant: "saved-variant",
			Run:     "saved-run",
		},
		Tags:       []string{"t1", "t2"},
		Attributes: map[string]string{"k": "v"},
	}

	// No flags parsed — nothing marked as Changed
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	var op, loc, devID, devSerial, devFW, devFWHash, devHW, plan, variant, run string
	var tags []string
	var attrs map[string]string
	fs.StringVar(&op, "operator", "", "")
	fs.StringVar(&loc, "location", "", "")
	fs.StringVar(&devID, "device-id", "", "")
	fs.StringVar(&devSerial, "device-serial", "", "")
	fs.StringVar(&devFW, "device-firmware", "", "")
	fs.StringVar(&devFWHash, "device-firmware-hash", "", "")
	fs.StringVar(&devHW, "device-hardware-version", "", "")
	fs.StringVar(&plan, "test-plan", "", "")
	fs.StringVar(&variant, "test-variant", "", "")
	fs.StringVar(&run, "test-run", "", "")
	fs.StringSliceVar(&tags, "tag", nil, "")
	fs.StringToStringVar(&attrs, "attr", nil, "")
	if err := fs.Parse([]string{}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	opts := core.ManifestOptions{} // all fields empty
	applyMetadataDefaults(&opts, saved, fs)

	checks := []struct{ got, want, field string }{
		{opts.Operator, "saved-operator", "Operator"},
		{opts.Location, "saved-location", "Location"},
		{opts.Device.ID, "saved-id", "Device.ID"},
		{opts.Device.Serial, "saved-serial", "Device.Serial"},
		{opts.Device.Firmware, "saved-fw", "Device.Firmware"},
		{opts.Device.FirmwareHash, "saved-hash", "Device.FirmwareHash"},
		{opts.Device.HardwareVersion, "saved-hw", "Device.HardwareVersion"},
		{opts.Test.Plan, "saved-plan", "Test.Plan"},
		{opts.Test.Variant, "saved-variant", "Test.Variant"},
		{opts.Test.Run, "saved-run", "Test.Run"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.field, c.got, c.want)
		}
	}
	if len(opts.Tags) != 2 || opts.Tags[0] != "t1" || opts.Tags[1] != "t2" {
		t.Errorf("Tags: got %v, want [t1 t2]", opts.Tags)
	}
	if opts.Attributes["k"] != "v" {
		t.Errorf("Attributes: got %v, want map[k:v]", opts.Attributes)
	}
}

// TestApplyMetadataDefaults_FlagChangedPreventsSavedOverride verifies that a flag
// marked as Changed (even with an empty value) prevents the saved config from overriding it.
func TestApplyMetadataDefaults_FlagChangedPreventsSavedOverride(t *testing.T) {
	saved := config.Metadata{Operator: "saved-operator"}

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	var op string
	fs.StringVar(&op, "operator", "", "")
	// Explicitly pass an empty string — flag is Changed but value is ""
	if err := fs.Parse([]string{"--operator", ""}); err != nil {
		t.Fatalf("parse flags: %v", err)
	}

	opts := core.ManifestOptions{Operator: ""}
	applyMetadataDefaults(&opts, saved, fs)

	// Because the flag was explicitly set (Changed), saved config must not win
	if opts.Operator != "" {
		t.Errorf("operator: got %q, want empty string (CLI explicit empty wins)", opts.Operator)
	}
}

// TestRunCaptureSerial_RequiresPort verifies that runCaptureSerial returns an error
// when no port is specified and not running interactively.
func TestRunCaptureSerial_RequiresPort(t *testing.T) {
	meta := &captureMetadataFlags{}
	cmd := newCaptureSerialCmd(meta)
	// port is empty string (zero value), term.IsTerminal returns false in test context
	err := runCaptureSerial(cmd, serialFlags{}, meta)
	if err == nil || !strings.Contains(err.Error(), "serial port is required") {
		t.Errorf("expected 'serial port is required' error, got %v", err)
	}
}

// TestRunCaptureTCP_RequiresHost verifies that runCaptureTCP returns an error
// when no host is specified and not running interactively.
func TestRunCaptureTCP_RequiresHost(t *testing.T) {
	meta := &captureMetadataFlags{}
	cmd := newCaptureTCPCmd(meta)
	err := runCaptureTCP(cmd, tcpFlags{}, meta)
	if err == nil || !strings.Contains(err.Error(), "tcp host is required") {
		t.Errorf("expected 'tcp host is required' error, got %v", err)
	}
}

// TestRunCaptureTCP_InvalidPort verifies that runCaptureTCP rejects out-of-range ports.
func TestRunCaptureTCP_InvalidPort(t *testing.T) {
	meta := &captureMetadataFlags{}
	cmd := newCaptureTCPCmd(meta)
	for _, port := range []int{0, -1, 65536, 99999} {
		err := runCaptureTCP(cmd, tcpFlags{host: "localhost", port: port}, meta)
		if err == nil || !strings.Contains(err.Error(), "tcp port must be between") {
			t.Errorf("port %d: expected port range error, got %v", port, err)
		}
	}
}

// TestRunCaptureFile_InvalidPath verifies that runCaptureFile returns an error
// when the file path does not exist.
func TestRunCaptureFile_InvalidPath(t *testing.T) {
	meta := &captureMetadataFlags{}
	cmd := newCaptureFileCmd(meta)
	err := runCaptureFile(cmd, "/nonexistent/path/to/file.csv", fileFlags{}, meta)
	if err == nil || !strings.Contains(err.Error(), "cannot access file") {
		t.Errorf("expected 'cannot access file' error, got %v", err)
	}
}

func TestConfigFromCmd_RequiresInjectedConfig(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	_, err := configFromCmd(cmd)
	if err == nil || !strings.Contains(err.Error(), "config not found in context") {
		t.Fatalf("expected missing config error, got %v", err)
	}
}

func TestRunCaptureFile_UsesOfflineCacheFromContext(t *testing.T) {
	meta := &captureMetadataFlags{}
	cmd := newCaptureFileCmd(meta)

	cacheDir := t.TempDir()
	cmd.SetContext(withConfig(context.Background(), config.Config{
		OfflineCache: cacheDir,
	}))

	inputPath := filepath.Join(t.TempDir(), "events.log")
	if err := os.WriteFile(inputPath, []byte("first\nsecond\n"), 0o644); err != nil {
		t.Fatalf("write input file: %v", err)
	}

	err := runCaptureFile(cmd, inputPath, fileFlags{format: "raw"}, meta)
	if err != nil {
		t.Fatalf("runCaptureFile: %v", err)
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("read cache dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 captured run in cache, got %d", len(entries))
	}

	runDir := filepath.Join(cacheDir, entries[0].Name())
	if _, err := os.Stat(filepath.Join(runDir, "manifest.json")); err != nil {
		t.Fatalf("expected manifest in offline cache, stat manifest.json: %v", err)
	}
	if _, err := os.Stat(filepath.Join(runDir, "data.jsonl")); err != nil {
		t.Fatalf("expected data output in offline cache, stat data.jsonl: %v", err)
	}
}
