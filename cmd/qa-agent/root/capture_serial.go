package root

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
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
			serialCfg *sources.Config
			launchTUI bool = serialTUI
		)
		if isInteractive {
			// Run interactive prompt
			var err error
			serialCfg, launchTUI, err = tui.RunSerialPrompt()
			if err != nil {
				return fmt.Errorf("interactive prompt failed: %w", err)
			}

			// Use values from interactive prompt
			serialPort = serialCfg.Port
			serialBaud = serialCfg.Baud
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

		if launchTUI {
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

			// Create a string channel for the TUI
			stringCh := make(chan string, 16)

			// Convert byte frames to strings
			go func() {
				defer close(stringCh)
				for frame := range serial.Frames() {
					// Send the entire frame (including newlines) to the TUI
					// The TUI will handle splitting on newlines and buffering partial lines
					if len(frame) > 0 {
						select {
						case stringCh <- string(frame):
						case <-ctx.Done():
							return
						}
					}
				}
			}()

			// Launch the TUI
			title := fmt.Sprintf("Serial Capture - %s @ %d", serialCfg.Port, serialCfg.Baud)
			m := tui.NewApp(title, stringCh)
			p := tea.NewProgram(m, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				return err
			}

			// Cancel context to stop serial reading
			cancel()
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
				Tags:       append([]string(nil), serialTags...),
				Attributes: cloneStringMap(serialAttributes),
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
