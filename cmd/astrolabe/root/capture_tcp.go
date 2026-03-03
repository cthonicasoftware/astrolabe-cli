package root

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cthonicasoftware/astrolabe-cli/internal/capture"
	"github.com/cthonicasoftware/astrolabe-cli/internal/cliout"
	"github.com/cthonicasoftware/astrolabe-cli/internal/config"
	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/normalize"
	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
	"github.com/cthonicasoftware/astrolabe-cli/internal/storage"
	"github.com/cthonicasoftware/astrolabe-cli/internal/tui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	tcpHost           string
	tcpPort           int
	tcpConnectTimeout time.Duration
	tcpReadTimeout    time.Duration
	tcpBufferSize     int
	tcpName           string
	tcpTUI            bool

	tcpOperator        string
	tcpLocation        string
	tcpDeviceID        string
	tcpDeviceSerial    string
	tcpDeviceFirmware  string
	tcpDeviceFWHash    string
	tcpDeviceHWVersion string
	tcpTestPlan        string
	tcpTestVariant     string
	tcpTestRun         string
	tcpTags            []string
	tcpAttributes      = map[string]string{}
)

var captureTCPCmd = &cobra.Command{
	Use:   "tcp",
	Short: "Capture from a TCP network source",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create styled printer (check for --json flag from root command)
		jsonMode, _ := cmd.Flags().GetBool("json")
		out := cliout.DefaultPrinter(jsonMode)

		// Check if we should run in interactive mode
		// Interactive mode runs when:
		// 1. We have a TTY (not in CI/pipe)
		// 2. Host flag wasn't explicitly set
		hostFlagSet := cmd.Flags().Changed("host")
		isInteractive := term.IsTerminal(int(os.Stdin.Fd())) && !hostFlagSet

		var (
			savedMetadata config.Metadata
			metaErr       error
		)
		if savedMetadata, metaErr = config.LoadMetadata(); metaErr != nil {
			out.Warning(fmt.Sprintf("Failed to load metadata: %v", metaErr))
		}

		var (
			tcpCfg    sources.TCPConfig
			launchTUI bool = tcpTUI
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
			if captureConfig.SourceType != "tcp" {
				return fmt.Errorf("tcp source required for this command, got: %s", captureConfig.SourceType)
			}

			// Use values from interactive prompt
			// Convert TCP port from string to int
			portInt, err := strconv.Atoi(captureConfig.TCPPort)
			if err != nil {
				return fmt.Errorf("invalid TCP port from interactive config: %w", err)
			}

			tcpCfg = sources.TCPConfig{
				Host:           captureConfig.TCPHost,
				Port:           portInt,
				ConnectTimeout: tcpConnectTimeout,
				ReadTimeout:    tcpReadTimeout,
				BufferSize:     tcpBufferSize,
			}
			tcpHost = captureConfig.TCPHost
			tcpPort = portInt
			launchTUI = true // Always launch TUI when using interactive mode
			tcpTUI = launchTUI
		} else {
			// Use command-line flags with defaults
			if tcpHost == "" {
				return fmt.Errorf("tcp host is required (use --host flag or run interactively)")
			}
			if tcpPort <= 0 || tcpPort > 65535 {
				return fmt.Errorf("tcp port must be between 1 and 65535, got %d", tcpPort)
			}

			tcpCfg = sources.TCPConfig{
				Host:           tcpHost,
				Port:           tcpPort,
				ConnectTimeout: tcpConnectTimeout,
				ReadTimeout:    tcpReadTimeout,
				BufferSize:     tcpBufferSize,
			}
		}

		if !launchTUI {
			out.Step(fmt.Sprintf("Starting TCP capture: %s:%d", tcpCfg.Host, tcpCfg.Port))
		}

		tcp, err := sources.NewTCPWithConfig(tcpCfg)
		if err != nil {
			return fmt.Errorf("create TCP source: %w", err)
		}

		flagTags, err := parseTagFlags(tcpTags)
		if err != nil {
			return fmt.Errorf("invalid --tag value: %w", err)
		}
		flagAttrs, err := parseAttributeFlags(tcpAttributes)
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

			meta := core.ManifestOptions{
				Operator: tcpOperator,
				Location: tcpLocation,
				Device: core.DeviceInfo{
					ID:              tcpDeviceID,
					Serial:          tcpDeviceSerial,
					Firmware:        tcpDeviceFirmware,
					FirmwareHash:    tcpDeviceFWHash,
					HardwareVersion: tcpDeviceHWVersion,
				},
				Test: core.TestInfo{
					Plan:    tcpTestPlan,
					Variant: tcpTestVariant,
					Run:     tcpTestRun,
				},
				Tags:       append([]string(nil), flagTags...),
				Attributes: cloneStringMap(flagAttrs),
			}
			if metaErr == nil {
				applyMetadataDefaults(&meta, savedMetadata, cmd.Flags())
			}

			pipelineOpts := capture.Options{
				Source:     tcp,
				Normalizer: normalizer,
				Store:      store,
				Manifest:   buildTCPManifest(tcpCfg, tcpName, meta),
				Capture:    buildTCPCaptureSettings(tcpCfg),
			}

			pipeline, err := capture.NewPipeline(pipelineOpts)
			if err != nil {
				return fmt.Errorf("build pipeline: %w", err)
			}

			// Open the TCP connection
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if err := tcp.Open(ctx); err != nil {
				return fmt.Errorf("failed to open TCP connection to %s:%d: %w", tcpCfg.Host, tcpCfg.Port, err)
			}
			defer func() {
				if err := tcp.Close(); err != nil {
					out.Warning(fmt.Sprintf("Failed to close TCP connection: %v", err))
				}
			}()

			// Create channels for TUI display and pipeline data
			stringCh := make(chan string, 16)
			pipelineFramesCh := make(chan []byte, 16)

			// Create a wrapper source for the pipeline that reads from our tee'd channel
			pipelineSource := &frameChannelSource{
				framesCh: pipelineFramesCh,
				meta:     tcp.Meta(),
			}
			pipelineOpts.Source = pipelineSource
			pipeline, err = capture.NewPipeline(pipelineOpts)
			if err != nil {
				return fmt.Errorf("rebuild pipeline with wrapper source: %w", err)
			}

			// Tee the data: read from TCP and send to both pipeline and TUI
			go func() {
				defer close(stringCh)
				defer close(pipelineFramesCh)
				for frame := range tcp.Frames() {
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
			title := fmt.Sprintf("TCP Capture - %s:%d", tcpCfg.Host, tcpCfg.Port)
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

			// Cancel context to stop TCP reading and pipeline
			cancel()

			if tuiApp.SaveRequested() {
				out.Blank()
				out.Step("Saving capture data...")

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

				out.Success("TCP capture saved")
				out.KeyValue("Run ID", run.ID)
				out.KeyValue("Records", fmt.Sprintf("%d", run.RecordsCount))
				out.KeyValue("Location", filepath.Join(appCfg.OfflineCache, run.ID))
				out.Blank()
				out.Muted("Run 'astrolabe upload' to upload to server.")
			} else {
				out.Blank()
				out.Muted("Exited without saving.")
				// Wait for pipeline to finish but discard results
				run := <-pipelineResultCh
				runErr := <-pipelineErrCh
				if runErr != nil && !errors.Is(runErr, context.Canceled) {
					out.Error(fmt.Sprintf("Capture pipeline error: %v", runErr))
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

			meta := core.ManifestOptions{
				Operator: tcpOperator,
				Location: tcpLocation,
				Device: core.DeviceInfo{
					ID:              tcpDeviceID,
					Serial:          tcpDeviceSerial,
					Firmware:        tcpDeviceFirmware,
					FirmwareHash:    tcpDeviceFWHash,
					HardwareVersion: tcpDeviceHWVersion,
				},
				Test: core.TestInfo{
					Plan:    tcpTestPlan,
					Variant: tcpTestVariant,
					Run:     tcpTestRun,
				},
				Tags:       append([]string(nil), flagTags...),
				Attributes: cloneStringMap(flagAttrs),
			}
			if metaErr == nil {
				applyMetadataDefaults(&meta, savedMetadata, cmd.Flags())
			}

			opts := capture.Options{
				Source:     tcp,
				Normalizer: normalizer,
				Store:      store,
				Manifest:   buildTCPManifest(tcpCfg, tcpName, meta),
				Capture:    buildTCPCaptureSettings(tcpCfg),
			}

			pipeline, err := capture.NewPipeline(opts)
			if err != nil {
				return fmt.Errorf("build pipeline: %w", err)
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			if err := tcp.Open(ctx); err != nil {
				return fmt.Errorf("failed to open TCP connection to %s:%d: %w", tcpCfg.Host, tcpCfg.Port, err)
			}
			defer func() {
				if err := tcp.Close(); err != nil {
					out.Warning(fmt.Sprintf("Failed to close TCP connection: %v", err))
				}
			}()

			out.Info("Capturing... (press Ctrl+C to stop)")
			out.Blank()

			run, runErr := pipeline.Run(ctx)
			if runErr != nil && !errors.Is(runErr, context.Canceled) {
				return fmt.Errorf("capture pipeline: %w", runErr)
			}
			if run == nil {
				return fmt.Errorf("capture pipeline: run not returned")
			}

			out.Blank()
			if runErr == nil {
				out.Success("TCP capture complete")
			} else {
				out.Warning("TCP capture interrupted (partial run saved)")
			}
			out.KeyValue("Run ID", run.ID)
			out.KeyValue("Records", fmt.Sprintf("%d", run.RecordsCount))
			out.KeyValue("Location", filepath.Join(appCfg.OfflineCache, run.ID))
			out.Blank()
			out.Muted("Run 'astrolabe upload' to upload to server.")
		}
		return nil
	},
}

func init() {
	captureCmd.AddCommand(captureTCPCmd)

	// TCP connection flags
	captureTCPCmd.Flags().StringVarP(&tcpHost, "host", "H", "localhost", "TCP host (hostname or IP address)")
	captureTCPCmd.Flags().IntVarP(&tcpPort, "port", "p", 9000, "TCP port")
	captureTCPCmd.Flags().DurationVar(&tcpConnectTimeout, "connect-timeout", 10*time.Second, "connection timeout")
	captureTCPCmd.Flags().DurationVar(&tcpReadTimeout, "read-timeout", 0, "read timeout (0 = no timeout)")
	captureTCPCmd.Flags().IntVar(&tcpBufferSize, "buffer-size", 4096, "read buffer size in bytes")

	// Run metadata flags
	captureTCPCmd.Flags().StringVar(&tcpName, "name", "", "optional run name")
	captureTCPCmd.Flags().BoolVar(&tcpTUI, "tui", false, "launch a live TUI")

	// Operator and location flags
	captureTCPCmd.Flags().StringVar(&tcpOperator, "operator", "", "operator assigned to this run")
	captureTCPCmd.Flags().StringVar(&tcpLocation, "location", "", "physical location or bench identifier")

	// Device metadata flags
	captureTCPCmd.Flags().StringVar(&tcpDeviceID, "device-id", "", "device identifier")
	captureTCPCmd.Flags().StringVar(&tcpDeviceSerial, "device-serial", "", "device serial number")
	captureTCPCmd.Flags().StringVar(&tcpDeviceFirmware, "device-firmware", "", "device firmware version")
	captureTCPCmd.Flags().StringVar(&tcpDeviceFWHash, "device-firmware-hash", "", "device firmware hash or build id")
	captureTCPCmd.Flags().StringVar(&tcpDeviceHWVersion, "device-hardware-version", "", "device hardware revision")

	// Test metadata flags
	captureTCPCmd.Flags().StringVar(&tcpTestPlan, "test-plan", "unspecified", "test plan identifier")
	captureTCPCmd.Flags().StringVar(&tcpTestVariant, "test-variant", "", "test plan variant")
	captureTCPCmd.Flags().StringVar(&tcpTestRun, "test-run", "", "test plan run identifier")

	// Tags and attributes
	captureTCPCmd.Flags().StringSliceVar(&tcpTags, "tag", nil, "tag to apply to this run (repeatable)")
	captureTCPCmd.Flags().StringToStringVar(&tcpAttributes, "attr", map[string]string{}, "additional manifest attribute (key=value, repeatable)")
}

const tcpSchemaVersion = "v1alpha1"

func buildTCPManifest(cfg sources.TCPConfig, name string, opts core.ManifestOptions) core.Manifest {
	attrs := map[string]string{
		"source_kind":     "tcp",
		"host":            cfg.Host,
		"port":            strconv.Itoa(cfg.Port),
		"connect_timeout": cfg.ConnectTimeout.String(),
		"read_timeout":    cfg.ReadTimeout.String(),
		"buffer_size":     strconv.Itoa(cfg.BufferSize),
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
		SchemaVersion: tcpSchemaVersion,
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

func buildTCPCaptureSettings(cfg sources.TCPConfig) core.CaptureSettings {
	return core.CaptureSettings{
		Channels: []string{"tcp"},
		Notes:    fmt.Sprintf("tcp capture from %s:%d", cfg.Host, cfg.Port),
	}
}
