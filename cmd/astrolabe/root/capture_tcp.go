package root

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/cliout"
	"github.com/cthonicasoftware/astrolabe-cli/internal/config"
	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
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
		jsonMode, _ := cmd.Flags().GetBool("json")
		out := cliout.DefaultPrinter(jsonMode)

		hostFlagSet := cmd.Flags().Changed("host")
		isInteractive := term.IsTerminal(int(os.Stdin.Fd())) && !hostFlagSet

		savedMetadata, metaErr := config.LoadMetadata()
		if metaErr != nil {
			out.Warning(fmt.Sprintf("Failed to load metadata: %v", metaErr))
		}

		var (
			tcpCfg    sources.TCPConfig
			launchTUI = tcpTUI
		)
		if isInteractive {
			captureConfig, err := runCaptureTabsForSource("tcp")
			if err != nil {
				return err
			}

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
			launchTUI = true
			tcpTUI = true
		} else {
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

		tcpSource, err := sources.NewTCPWithConfig(tcpCfg)
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

		meta := buildManifestOptions(captureMetadataInput{
			Operator:        tcpOperator,
			Location:        tcpLocation,
			DeviceID:        tcpDeviceID,
			DeviceSerial:    tcpDeviceSerial,
			DeviceFirmware:  tcpDeviceFirmware,
			DeviceFWHash:    tcpDeviceFWHash,
			DeviceHWVersion: tcpDeviceHWVersion,
			TestPlan:        tcpTestPlan,
			TestVariant:     tcpTestVariant,
			TestRun:         tcpTestRun,
			Tags:            flagTags,
			Attributes:      flagAttrs,
		}, savedMetadata, cmd.Flags(), metaErr == nil)

		manifest := buildTCPManifest(tcpCfg, tcpName, meta)
		captureSettings := buildTCPCaptureSettings(tcpCfg)
		appCfg := config.Load()

		if launchTUI {
			title := fmt.Sprintf("TCP Capture - %s:%d", tcpCfg.Host, tcpCfg.Port)
			run, saved, err := runInteractiveCapture(out, tcpSource, title, appCfg.OfflineCache, manifest, captureSettings)
			if err != nil {
				return err
			}

			out.Blank()
			if saved {
				out.Success("TCP capture saved")
				out.KeyValue("Run ID", run.ID)
				out.KeyValue("Records", fmt.Sprintf("%d", run.RecordsCount))
				out.KeyValue("Location", filepath.Join(appCfg.OfflineCache, run.ID))
				out.Blank()
				out.Muted("Run 'astrolabe upload' to upload to server.")
				return nil
			}

			out.Muted("Exited without saving.")
			return nil
		}

		out.Info("Capturing... (press Ctrl+C to stop)")
		out.Blank()
		run, interrupted, err := runHeadlessCapture(out, tcpSource, appCfg.OfflineCache, manifest, captureSettings)
		if err != nil {
			return err
		}

		out.Blank()
		if interrupted {
			out.Warning("TCP capture interrupted (partial run saved)")
		} else {
			out.Success("TCP capture complete")
		}
		out.KeyValue("Run ID", run.ID)
		out.KeyValue("Records", fmt.Sprintf("%d", run.RecordsCount))
		out.KeyValue("Location", filepath.Join(appCfg.OfflineCache, run.ID))
		out.Blank()
		out.Muted("Run 'astrolabe upload' to upload to server.")
		return nil
	},
}

func init() {
	captureCmd.AddCommand(captureTCPCmd)

	captureTCPCmd.Flags().StringVarP(&tcpHost, "host", "H", "localhost", "TCP host (hostname or IP address)")
	captureTCPCmd.Flags().IntVarP(&tcpPort, "port", "p", 9000, "TCP port")
	captureTCPCmd.Flags().DurationVar(&tcpConnectTimeout, "connect-timeout", 10*time.Second, "connection timeout")
	captureTCPCmd.Flags().DurationVar(&tcpReadTimeout, "read-timeout", 0, "read timeout (0 = no timeout)")
	captureTCPCmd.Flags().IntVar(&tcpBufferSize, "buffer-size", 4096, "read buffer size in bytes")

	captureTCPCmd.Flags().StringVar(&tcpName, "name", "", "optional run name")
	captureTCPCmd.Flags().BoolVar(&tcpTUI, "tui", false, "launch a live TUI")

	captureTCPCmd.Flags().StringVar(&tcpOperator, "operator", "", "operator assigned to this run")
	captureTCPCmd.Flags().StringVar(&tcpLocation, "location", "", "physical location or bench identifier")
	captureTCPCmd.Flags().StringVar(&tcpDeviceID, "device-id", "", "device identifier")
	captureTCPCmd.Flags().StringVar(&tcpDeviceSerial, "device-serial", "", "device serial number")
	captureTCPCmd.Flags().StringVar(&tcpDeviceFirmware, "device-firmware", "", "device firmware version")
	captureTCPCmd.Flags().StringVar(&tcpDeviceFWHash, "device-firmware-hash", "", "device firmware hash or build id")
	captureTCPCmd.Flags().StringVar(&tcpDeviceHWVersion, "device-hardware-version", "", "device hardware revision")
	captureTCPCmd.Flags().StringVar(&tcpTestPlan, "test-plan", "unspecified", "test plan identifier")
	captureTCPCmd.Flags().StringVar(&tcpTestVariant, "test-variant", "", "test plan variant")
	captureTCPCmd.Flags().StringVar(&tcpTestRun, "test-run", "", "test plan run identifier")
	captureTCPCmd.Flags().StringSliceVar(&tcpTags, "tag", nil, "tag to apply to this run (repeatable)")
	captureTCPCmd.Flags().StringToStringVar(&tcpAttributes, "attr", map[string]string{}, "additional manifest attribute (key=value, repeatable)")
}

func buildTCPManifest(cfg sources.TCPConfig, name string, opts core.ManifestOptions) core.Manifest {
	attrs := map[string]string{
		"source_kind":     "tcp",
		"host":            cfg.Host,
		"port":            strconv.Itoa(cfg.Port),
		"connect_timeout": cfg.ConnectTimeout.String(),
		"read_timeout":    cfg.ReadTimeout.String(),
		"buffer_size":     strconv.Itoa(cfg.BufferSize),
	}
	if name != "" {
		attrs["run_label"] = name
	}
	return buildCaptureManifest(opts, attrs)
}

func buildTCPCaptureSettings(cfg sources.TCPConfig) core.CaptureSettings {
	return core.CaptureSettings{
		Channels: []string{"tcp"},
		Notes:    fmt.Sprintf("tcp capture from %s:%d", cfg.Host, cfg.Port),
	}
}
