package root

import (
	"testing"

	"github.com/cthonicasoftware/astrolabe-cli/internal/config"
	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/spf13/pflag"
)

var _ = captureSerialCmd // ensure subcommands are referenced

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
	cmds := map[string]interface{ LocalFlags() *pflag.FlagSet }{
		"serial": captureSerialCmd,
		"tcp":    captureTCPCmd,
		"file":   captureFileCmd,
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
