package root

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/capture"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/config"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/core"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/normalize"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/sources"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/storage"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/term"
)

var (
	serialPort string
	serialBaud int
	serialName string
	serialTUI  bool

	serialOperator        string
	serialLocation        string
	serialDeviceID        string
	serialDeviceSerial    string
	serialDeviceFirmware  string
	serialDeviceFWHash    string
	serialDeviceHWVersion string
	serialTestPlan        string
	serialTestVariant     string
	serialTestRun         string
	serialTags            []string
	serialAttributes      = map[string]string{}
)

var captureSerialCmd = &cobra.Command{
	Use:   "serial",
	Short: "Capture from a serial port",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if we should run in interactive mode
		// Interactive mode runs when:
		// 1. We have a TTY (not in CI/pipe)
		// 2. Port flag wasn't explicitly set
		portFlagSet := cmd.Flags().Changed("port")
		isInteractive := term.IsTerminal(int(os.Stdin.Fd())) && !portFlagSet

		var (
			savedMetadata config.Metadata
			metaErr       error
		)
		if savedMetadata, metaErr = config.LoadMetadata(); metaErr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to load metadata: %v\n", metaErr)
		}

		var (
			serialCfg *sources.Config
			launchTUI bool = serialTUI
		)
		if isInteractive {
			// Run interactive prompt with tabbed capture interface
			var err error
			captureConfig, err := tui.RunCaptureTabs()
			if err != nil {
				return fmt.Errorf("interactive prompt failed: %w", err)
			}
			if captureConfig == nil {
				return fmt.Errorf("capture configuration cancelled")
			}
			if captureConfig.SourceType != "serial" {
				return fmt.Errorf("serial source required for this command, got: %s", captureConfig.SourceType)
			}
			if captureConfig.SerialConfig == nil {
				return fmt.Errorf("serial configuration missing")
			}

			// Use values from interactive prompt
			serialCfg = captureConfig.SerialConfig
			serialPort = serialCfg.Port
			serialBaud = serialCfg.Baud
			launchTUI = true // Always launch TUI when using interactive mode
			serialTUI = launchTUI
		} else {
			// Use command-line flags with defaults
			if serialPort == "" {
				return fmt.Errorf("serial port is required (use --port flag or run interactively)")
			}
			defaults := sources.DefaultConfig()
			defaults.Port = serialPort
			defaults.Baud = serialBaud
			serialCfg = &defaults
		}

		fmt.Printf("Starting serial capture: port=%s baud=%d parity=%s data=%d stop=%s flow=%s name=%s tui=%v\n",
			serialCfg.Port, serialCfg.Baud, serialCfg.Parity, serialCfg.DataBits, serialCfg.StopBits, serialCfg.FlowControl, serialName, launchTUI)

		serial := sources.NewSerialWithConfig(*serialCfg)

		flagTags, err := parseTagFlags(serialTags)
		if err != nil {
			return fmt.Errorf("invalid --tag value: %w", err)
		}
		flagAttrs, err := parseAttributeFlags(serialAttributes)
		if err != nil {
			return fmt.Errorf("invalid --attr value: %w", err)
		}

		if launchTUI {
			// Setup for TUI mode with optional save capability
			appCfg := config.Load()
			if err := os.MkdirAll(appCfg.OfflineCache, 0o755); err != nil {
				return fmt.Errorf("ensure offline cache: %w", err)
			}

			tempRoot, err := os.MkdirTemp(appCfg.OfflineCache, ".tmp-run-")
			if err != nil {
				return fmt.Errorf("create temp run dir: %w", err)
			}
			defer os.RemoveAll(tempRoot)

			store := storage.NewFS(tempRoot)
			normalizer := normalize.NewLineJSON()

			meta := serialManifestOptions{
				Operator: serialOperator,
				Location: serialLocation,
				Device: core.DeviceInfo{
					ID:              serialDeviceID,
					Serial:          serialDeviceSerial,
					Firmware:        serialDeviceFirmware,
					FirmwareHash:    serialDeviceFWHash,
					HardwareVersion: serialDeviceHWVersion,
				},
				Test: core.TestInfo{
					Plan:    serialTestPlan,
					Variant: serialTestVariant,
					Run:     serialTestRun,
				},
				Tags:       append([]string(nil), flagTags...),
				Attributes: cloneStringMap(flagAttrs),
			}
			if metaErr == nil {
				applyMetadataDefaults(&meta, savedMetadata, cmd.Flags())
			}

			pipelineOpts := capture.Options{
				Source:     serial,
				Normalizer: normalizer,
				Store:      store,
				Manifest:   buildSerialManifest(*serialCfg, serialName, meta),
				Capture:    buildSerialCaptureSettings(*serialCfg),
			}

			pipeline, err := capture.NewPipeline(pipelineOpts)
			if err != nil {
				return fmt.Errorf("build pipeline: %w", err)
			}

			// Open the serial port
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := serial.Open(ctx); err != nil {
				return fmt.Errorf("failed to open serial port: %w", err)
			}
			defer func() {
				if err := serial.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to close serial port: %v\n", err)
				}
			}()

			// Create channels for TUI display and pipeline data
			stringCh := make(chan string, 16)
			pipelineFramesCh := make(chan []byte, 16)

			// Create a wrapper source for the pipeline that reads from our tee'd channel
			pipelineSource := &frameChannelSource{
				framesCh: pipelineFramesCh,
				meta:     serial.Meta(),
			}
			pipelineOpts.Source = pipelineSource
			pipeline, err = capture.NewPipeline(pipelineOpts)
			if err != nil {
				return fmt.Errorf("rebuild pipeline with wrapper source: %w", err)
			}

			// Tee the data: read from serial and send to both pipeline and TUI
			go func() {
				defer close(stringCh)
				defer close(pipelineFramesCh)
				for frame := range serial.Frames() {
					if len(frame) == 0 {
						continue
					}
					// Send to TUI display
					select {
					case stringCh <- string(frame):
					case <-ctx.Done():
						return
					}
					// Send to pipeline
					select {
					case pipelineFramesCh <- frame:
					case <-ctx.Done():
						return
					}
				}
			}()

			// Run the capture pipeline in the background
			pipelineResultCh := make(chan *core.Run, 1)
			pipelineErrCh := make(chan error, 1)
			go func() {
				run, err := pipeline.Run(ctx)
				pipelineResultCh <- run
				pipelineErrCh <- err
			}()

			// Launch the TUI
			title := fmt.Sprintf("Serial Capture - %s @ %d", serialCfg.Port, serialCfg.Baud)
			m := tui.NewApp(title, stringCh)
			p := tea.NewProgram(m, tea.WithAltScreen())
			finalModel, err := p.Run()
			if err != nil {
				cancel()
				return err
			}

			// Check if user requested to save
			tuiApp, ok := finalModel.(*tui.App)
			if !ok {
				cancel()
				return fmt.Errorf("unexpected model type: %T", finalModel)
			}

			// Cancel context to stop serial reading and pipeline
			cancel()

			if tuiApp.SaveRequested() {
				fmt.Println("\nSaving capture data...")
				run := <-pipelineResultCh
				runErr := <-pipelineErrCh

				if runErr != nil && !errors.Is(runErr, context.Canceled) {
					return fmt.Errorf("capture pipeline: %w", runErr)
				}
				if run == nil {
					return fmt.Errorf("capture pipeline: run not returned")
				}

				if err := promoteRunArtifacts(run, tempRoot, appCfg.OfflineCache); err != nil {
					return fmt.Errorf("finalize run artifacts: %w", err)
				}

				fmt.Printf("Capture saved. Records: %d\n", run.RecordsCount)
				fmt.Printf("Run ID: %s\n", run.ID)
				fmt.Printf("Cache dir: %s\n", appCfg.OfflineCache)
				for _, artifact := range run.Artifacts {
					fmt.Printf(" - %s (%s)\n", artifact.Path, artifact.Role)
				}
			} else {
				fmt.Println("\nExited without saving.")
				// Wait for pipeline to finish but discard results
				run := <-pipelineResultCh
				runErr := <-pipelineErrCh
				if runErr != nil && !errors.Is(runErr, context.Canceled) {
					fmt.Fprintf(os.Stderr, "capture pipeline error: %v\n", runErr)
				}
				if run != nil {
					runDir := filepath.Join(tempRoot, run.ID)
					_ = os.RemoveAll(runDir)
				}
			}
		} else {
			appCfg := config.Load()
			store := storage.NewFS(appCfg.OfflineCache)
			normalizer := normalize.NewLineJSON()

			meta := serialManifestOptions{
				Operator: serialOperator,
				Location: serialLocation,
				Device: core.DeviceInfo{
					ID:              serialDeviceID,
					Serial:          serialDeviceSerial,
					Firmware:        serialDeviceFirmware,
					FirmwareHash:    serialDeviceFWHash,
					HardwareVersion: serialDeviceHWVersion,
				},
				Test: core.TestInfo{
					Plan:    serialTestPlan,
					Variant: serialTestVariant,
					Run:     serialTestRun,
				},
				Tags:       append([]string(nil), flagTags...),
				Attributes: cloneStringMap(flagAttrs),
			}
			if metaErr == nil {
				applyMetadataDefaults(&meta, savedMetadata, cmd.Flags())
			}

			opts := capture.Options{
				Source:     serial,
				Normalizer: normalizer,
				Store:      store,
				Manifest:   buildSerialManifest(*serialCfg, serialName, meta),
				Capture:    buildSerialCaptureSettings(*serialCfg),
			}

			pipeline, err := capture.NewPipeline(opts)
			if err != nil {
				return fmt.Errorf("build pipeline: %w", err)
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			if err := serial.Open(ctx); err != nil {
				return fmt.Errorf("failed to open serial port: %w", err)
			}
			defer func() {
				if err := serial.Close(); err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to close serial port: %v\n", err)
				}
			}()

			fmt.Println("Press Ctrl+C to stop capture and flush artifacts.")
			run, runErr := pipeline.Run(ctx)
			if runErr != nil && !errors.Is(runErr, context.Canceled) {
				return fmt.Errorf("capture pipeline: %w", runErr)
			}
			if run == nil {
				return fmt.Errorf("capture pipeline: run not returned")
			}
			if runErr == nil {
				fmt.Printf("Capture complete. Records: %d\n", run.RecordsCount)
			} else {
				fmt.Printf("Capture interrupted. Partial run saved. Records: %d\n", run.RecordsCount)
			}

			fmt.Printf("Run ID: %s\n", run.ID)
			fmt.Printf("Cache dir: %s\n", appCfg.OfflineCache)
			for _, artifact := range run.Artifacts {
				fmt.Printf(" - %s (%s)\n", artifact.Path, artifact.Role)
			}
		}
		return nil
	},
}

func init() {
	captureCmd.AddCommand(captureSerialCmd)
	captureSerialCmd.Flags().StringVarP(&serialPort, "port", "p", "/dev/ttyUSB0", "serial port path")
	captureSerialCmd.Flags().IntVarP(&serialBaud, "baud", "b", 115200, "baud rate")
	captureSerialCmd.Flags().StringVar(&serialName, "name", "", "optional run name")
	captureSerialCmd.Flags().BoolVar(&serialTUI, "tui", false, "launch a live TUI")
	captureSerialCmd.Flags().StringVar(&serialOperator, "operator", "", "operator assigned to this run")
	captureSerialCmd.Flags().StringVar(&serialLocation, "location", "", "physical location or bench identifier")
	captureSerialCmd.Flags().StringVar(&serialDeviceID, "device-id", "", "device identifier")
	captureSerialCmd.Flags().StringVar(&serialDeviceSerial, "device-serial", "", "device serial number")
	captureSerialCmd.Flags().StringVar(&serialDeviceFirmware, "device-firmware", "", "device firmware version")
	captureSerialCmd.Flags().StringVar(&serialDeviceFWHash, "device-firmware-hash", "", "device firmware hash or build id")
	captureSerialCmd.Flags().StringVar(&serialDeviceHWVersion, "device-hw", "", "device hardware revision")
	captureSerialCmd.Flags().StringVar(&serialTestPlan, "test-plan", "unspecified", "test plan identifier")
	captureSerialCmd.Flags().StringVar(&serialTestVariant, "test-variant", "", "test plan variant")
	captureSerialCmd.Flags().StringVar(&serialTestRun, "test-run", "", "test plan run identifier")
	captureSerialCmd.Flags().StringSliceVar(&serialTags, "tag", nil, "tag to apply to this run (repeatable)")
	captureSerialCmd.Flags().StringToStringVar(&serialAttributes, "attr", map[string]string{}, "additional manifest attribute (key=value, repeatable)")
}

const serialSchemaVersion = "v1alpha1"

type serialManifestOptions struct {
	Operator   string
	Location   string
	Device     core.DeviceInfo
	Test       core.TestInfo
	Tags       []string
	Attributes map[string]string
}

func buildSerialManifest(cfg sources.Config, name string, opts serialManifestOptions) core.Manifest {
	attrs := map[string]string{
		"source_kind": "serial",
		"port":        cfg.Port,
		"baud":        strconv.Itoa(cfg.Baud),
	}

	for k, v := range opts.Attributes {
		if k == "" || v == "" {
			continue
		}
		attrs[k] = v
	}
	if name != "" {
		attrs["run_label"] = name
	}

	manifest := core.Manifest{
		SchemaVersion: serialSchemaVersion,
		Device:        opts.Device,
		Test:          opts.Test,
		Operator:      opts.Operator,
		Location:      opts.Location,
		Tags:          append([]string(nil), opts.Tags...),
		Attributes:    attrs,
	}

	if manifest.Operator == "" {
		manifest.Operator = os.Getenv("USER")
	}
	if manifest.Test.Plan == "" {
		manifest.Test.Plan = "unspecified"
	}

	return manifest
}

func buildSerialCaptureSettings(cfg sources.Config) core.CaptureSettings {
	return core.CaptureSettings{
		Channels: []string{"serial"},
		Notes:    fmt.Sprintf("serial capture from %s @ %d baud", cfg.Port, cfg.Baud),
	}
}

func applyMetadataDefaults(opts *serialManifestOptions, saved config.Metadata, flags *pflag.FlagSet) {
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
	if saved.Device.HardwareVersion != "" && !isChanged("device-hw") && opts.Device.HardwareVersion == "" {
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

func parseTagFlags(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	return config.ParseTags(strings.Join(values, ","))
}

func parseAttributeFlags(values map[string]string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	var b strings.Builder
	first := true
	for k, v := range values {
		if !first {
			b.WriteRune('\n')
		}
		first = false
		b.WriteString(fmt.Sprintf("%s=%s", k, v))
	}
	return config.ParseAttributes(b.String())
}

func promoteRunArtifacts(run *core.Run, tempRoot, cacheRoot string) error {
	if run == nil {
		return fmt.Errorf("run is nil")
	}

	tempDir := filepath.Join(tempRoot, run.ID)
	finalDir := filepath.Join(cacheRoot, run.ID)

	if _, err := os.Stat(tempDir); err != nil {
		return fmt.Errorf("temporary run artifacts missing: %w", err)
	}
	if _, err := os.Stat(finalDir); err == nil {
		return fmt.Errorf("run directory already exists: %s", finalDir)
	}

	if err := os.Rename(tempDir, finalDir); err != nil {
		return fmt.Errorf("move run artifacts: %w", err)
	}

	relocate := func(path string) (string, error) {
		if path == "" {
			return "", nil
		}
		rel, err := filepath.Rel(tempDir, path)
		if err != nil {
			return "", err
		}
		return filepath.Join(finalDir, rel), nil
	}

	if run.PrimaryDataURI != "" {
		newPrimary, err := relocate(run.PrimaryDataURI)
		if err != nil {
			return fmt.Errorf("update primary data path: %w", err)
		}
		run.PrimaryDataURI = newPrimary
	}

	for i := range run.Artifacts {
		newPath, err := relocate(run.Artifacts[i].Path)
		if err != nil {
			return fmt.Errorf("update artifact %q path: %w", run.Artifacts[i].Name, err)
		}
		run.Artifacts[i].Path = newPath
	}

	return nil
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// frameChannelSource wraps a frames channel and implements the capture.Source interface
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
