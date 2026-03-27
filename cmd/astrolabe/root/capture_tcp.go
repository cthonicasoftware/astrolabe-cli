package root

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/cliout"
	"github.com/cthonicasoftware/astrolabe-cli/internal/config"
	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type tcpFlags struct {
	host           string
	port           int
	connectTimeout time.Duration
	readTimeout    time.Duration
	bufferSize     int
	name           string
}

func newCaptureTCPCmd(meta *captureMetadataFlags) *cobra.Command {
	var flags tcpFlags
	cmd := &cobra.Command{
		Use:   "tcp",
		Short: "Capture from a TCP network source",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCaptureTCP(cmd, flags, meta)
		},
	}
	cmd.Flags().StringVarP(&flags.host, "host", "H", "localhost", "TCP host (hostname or IP address)")
	cmd.Flags().IntVarP(&flags.port, "port", "p", 9000, "TCP port")
	cmd.Flags().DurationVar(&flags.connectTimeout, "connect-timeout", 10*time.Second, "connection timeout")
	cmd.Flags().DurationVar(&flags.readTimeout, "read-timeout", 0, "read timeout (0 = no timeout)")
	cmd.Flags().IntVar(&flags.bufferSize, "buffer-size", 4096, "read buffer size in bytes")
	cmd.Flags().StringVar(&flags.name, "name", "", "optional run name")
	return cmd
}

func runCaptureTCP(cmd *cobra.Command, flags tcpFlags, meta *captureMetadataFlags) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	out := cliout.DefaultPrinter(jsonMode)

	hostFlagSet := cmd.Flags().Changed("host")
	isInteractive := term.IsTerminal(int(os.Stdin.Fd())) && !hostFlagSet

	if isInteractive {
		return runCaptureTUI(cmd, out)
	}

	if flags.host == "" {
		return fmt.Errorf("tcp host is required (use --host flag or run interactively)")
	}
	if flags.port <= 0 || flags.port > 65535 {
		return fmt.Errorf("tcp port must be between 1 and 65535, got %d", flags.port)
	}
	tcpCfg := sources.TCPConfig{
		Host:           flags.host,
		Port:           flags.port,
		ConnectTimeout: flags.connectTimeout,
		ReadTimeout:    flags.readTimeout,
		BufferSize:     flags.bufferSize,
	}

	out.Step(fmt.Sprintf("Starting TCP capture: %s:%d", tcpCfg.Host, tcpCfg.Port))

	savedMetadata, metaErr := config.LoadMetadata()
	if metaErr != nil {
		out.Warning(fmt.Sprintf("Failed to load metadata: %v", metaErr))
	}

	flagTags, err := parseTagFlags(meta.tags)
	if err != nil {
		return fmt.Errorf("invalid --tag value: %w", err)
	}
	flagAttrs, err := parseAttributeFlags(meta.attributes)
	if err != nil {
		return fmt.Errorf("invalid --attr value: %w", err)
	}

	metaOpts := buildManifestOptions(captureMetadataInput{
		Operator:        meta.operator,
		Location:        meta.location,
		DeviceID:        meta.deviceID,
		DeviceSerial:    meta.deviceSerial,
		DeviceFirmware:  meta.deviceFirmware,
		DeviceFWHash:    meta.deviceFWHash,
		DeviceHWVersion: meta.deviceHWVersion,
		TestPlan:        meta.testPlan,
		TestVariant:     meta.testVariant,
		TestRun:         meta.testRun,
		Tags:            flagTags,
		Attributes:      flagAttrs,
	}, savedMetadata, cmd.Flags(), metaErr == nil)

	appCfg, err := configFromCmd(cmd)
	if err != nil {
		return err
	}
	svc := newCaptureService(out, appCfg.OfflineCache)

	out.Info("Capturing... (press Ctrl+C to stop)")
	out.Blank()
	result, err := svc.Run(cmd.Context(), CaptureRequest{
		TCPConfig: &tcpCfg,
		RunLabel:  flags.name,
		Meta:      metaOpts,
	})
	if err != nil {
		return err
	}
	run := result.Run

	out.Blank()
	if result.Interrupted {
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
}
