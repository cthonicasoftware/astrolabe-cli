package root

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/cthonicasoftware/astrolabe-cli/internal/capture"
	"github.com/cthonicasoftware/astrolabe-cli/internal/cliout"
	"github.com/cthonicasoftware/astrolabe-cli/internal/config"
	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/normalize"
	"github.com/cthonicasoftware/astrolabe-cli/internal/storage"
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

func runHeadlessCapture(
	out *cliout.Printer,
	source captureManagedSource,
	normalizer normalize.Normalizer,
	storeRoot string,
	manifest core.Manifest,
	captureSettings core.CaptureSettings,
) (*core.Run, bool, error) {
	store := storage.NewFS(storeRoot)
	opts := capture.Options{
		Source:     source,
		Normalizer: normalizer,
		Store:      store,
		Manifest:   manifest,
		Capture:    captureSettings,
	}

	pipeline, err := capture.NewPipeline(opts)
	if err != nil {
		return nil, false, fmt.Errorf("build pipeline: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := source.Open(ctx); err != nil {
		return nil, false, fmt.Errorf("open source: %w", err)
	}
	defer func() {
		if err := source.Close(); err != nil {
			out.Warning(fmt.Sprintf("Failed to close source: %v", err))
		}
	}()

	run, runErr := pipeline.Run(ctx)
	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		return nil, false, fmt.Errorf("capture pipeline: %w", runErr)
	}
	if run == nil {
		return nil, false, fmt.Errorf("capture pipeline: run not returned")
	}

	return run, runErr != nil, nil
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
