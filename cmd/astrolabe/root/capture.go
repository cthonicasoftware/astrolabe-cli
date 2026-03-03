package root

import (
	"strings"

	"github.com/cthonicasoftware/astrolabe-cli/internal/config"
	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var captureCmd = &cobra.Command{
	Use:   "capture",
	Short: "Start a capture from a source",
	Args:  cobra.NoArgs,
}

func init() {
	// subcommands are defined in separate files (serial, tcp, file)
}

// applyMetadataDefaults fills in fields on opts from saved config for any flag
// that was not explicitly set on the command line.
func applyMetadataDefaults(opts *core.ManifestOptions, saved config.Metadata, flags *pflag.FlagSet) {
	isChanged := func(name string) bool {
		if flags == nil {
			return false
		}
		return flags.Changed(name)
	}

	if saved.Operator != "" && !isChanged("operator") && opts.Operator == "" {
		opts.Operator = saved.Operator
	}
	if saved.Location != "" && !isChanged("location") && opts.Location == "" {
		opts.Location = saved.Location
	}

	if saved.Device.ID != "" && !isChanged("device-id") && opts.Device.ID == "" {
		opts.Device.ID = saved.Device.ID
	}
	if saved.Device.Serial != "" && !isChanged("device-serial") && opts.Device.Serial == "" {
		opts.Device.Serial = saved.Device.Serial
	}
	if saved.Device.Firmware != "" && !isChanged("device-firmware") && opts.Device.Firmware == "" {
		opts.Device.Firmware = saved.Device.Firmware
	}
	if saved.Device.FirmwareHash != "" && !isChanged("device-firmware-hash") && opts.Device.FirmwareHash == "" {
		opts.Device.FirmwareHash = saved.Device.FirmwareHash
	}
	if saved.Device.HardwareVersion != "" && !isChanged("device-hardware-version") && opts.Device.HardwareVersion == "" {
		opts.Device.HardwareVersion = saved.Device.HardwareVersion
	}

	if saved.Test.Plan != "" && !isChanged("test-plan") {
		current := strings.TrimSpace(opts.Test.Plan)
		if current == "" || strings.EqualFold(current, "unspecified") {
			opts.Test.Plan = saved.Test.Plan
		}
	}
	if saved.Test.Variant != "" && !isChanged("test-variant") && opts.Test.Variant == "" {
		opts.Test.Variant = saved.Test.Variant
	}
	if saved.Test.Run != "" && !isChanged("test-run") && opts.Test.Run == "" {
		opts.Test.Run = saved.Test.Run
	}

	if len(saved.Tags) > 0 && !isChanged("tag") && len(opts.Tags) == 0 {
		opts.Tags = append([]string(nil), saved.Tags...)
	}
	if len(saved.Attributes) > 0 && !isChanged("attr") && len(opts.Attributes) == 0 {
		opts.Attributes = cloneStringMap(saved.Attributes)
	}
}
