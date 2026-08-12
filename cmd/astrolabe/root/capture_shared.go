package root

import (
	"context"
	"os"

	"github.com/cthonicasoftware/astrolabe-cli/internal/capture"
	"github.com/cthonicasoftware/astrolabe-cli/internal/config"
	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/spf13/pflag"
)

const captureManifestSchemaVersion = "v1alpha1"

type captureManagedSource interface {
	capture.Source
	Open(context.Context) error
	Close() error
}

type captureMetadataInput struct {
	Operator        string
	Location        string
	DeviceID        string
	DeviceSerial    string
	DeviceFirmware  string
	DeviceFWHash    string
	DeviceHWVersion string
	TestPlan        string
	TestVariant     string
	TestRun         string
	Tags            []string
	Attributes      map[string]string
}

func buildManifestOptions(in captureMetadataInput, savedMetadata config.Metadata, flags *pflag.FlagSet, applyDefaults bool) core.ManifestOptions {
	opts := core.ManifestOptions{
		Operator: in.Operator,
		Location: in.Location,
		Device: core.DeviceInfo{
			ID:              in.DeviceID,
			Serial:          in.DeviceSerial,
			Firmware:        in.DeviceFirmware,
			FirmwareHash:    in.DeviceFWHash,
			HardwareVersion: in.DeviceHWVersion,
		},
		Test: core.TestInfo{
			Plan:    in.TestPlan,
			Variant: in.TestVariant,
			Run:     in.TestRun,
		},
		Tags:       append([]string(nil), in.Tags...),
		Attributes: cloneStringMap(in.Attributes),
	}

	if applyDefaults {
		applyMetadataDefaults(&opts, savedMetadata, flags)
	}
	return opts
}

func buildCaptureManifest(opts core.ManifestOptions, attrs map[string]string) core.Manifest {
	cleanAttrs := make(map[string]string, len(attrs)+len(opts.Attributes))
	for k, v := range attrs {
		if k == "" || v == "" {
			continue
		}
		cleanAttrs[k] = v
	}
	for k, v := range opts.Attributes {
		if k == "" || v == "" {
			continue
		}
		cleanAttrs[k] = v
	}

	manifest := core.Manifest{
		SchemaVersion: captureManifestSchemaVersion,
		Device:        opts.Device,
		Test:          opts.Test,
		Operator:      opts.Operator,
		Location:      opts.Location,
		Tags:          append([]string(nil), opts.Tags...),
		Attributes:    cleanAttrs,
	}

	if manifest.Operator == "" {
		manifest.Operator = os.Getenv("USER")
	}
	if manifest.Test.Plan == "" {
		manifest.Test.Plan = "unspecified"
	}

	return manifest
}

// frameChannelSource wraps a frames channel and implements the capture.Source interface.
type frameChannelSource struct {
	framesCh <-chan []byte
	meta     core.SourceMeta
}

func (f *frameChannelSource) Frames() <-chan []byte {
	return f.framesCh
}

func (f *frameChannelSource) Meta() core.SourceMeta {
	return f.meta
}
