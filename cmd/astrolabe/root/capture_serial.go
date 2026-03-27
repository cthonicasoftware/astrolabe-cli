package root

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/cthonicasoftware/astrolabe-cli/internal/cliout"
	"github.com/cthonicasoftware/astrolabe-cli/internal/config"
	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type serialFlags struct {
	port string
	baud int
	name string
}

func newCaptureSerialCmd(meta *captureMetadataFlags) *cobra.Command {
	var flags serialFlags
	cmd := &cobra.Command{
		Use:   "serial",
		Short: "Capture from a serial port",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCaptureSerial(cmd, flags, meta)
		},
	}
	cmd.Flags().StringVarP(&flags.port, "port", "p", "/dev/ttyUSB0", "serial port path")
	cmd.Flags().IntVarP(&flags.baud, "baud", "b", 115200, "baud rate")
	cmd.Flags().StringVar(&flags.name, "name", "", "optional run name")
	return cmd
}

func runCaptureSerial(cmd *cobra.Command, flags serialFlags, meta *captureMetadataFlags) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	out := cliout.DefaultPrinter(jsonMode)

	portFlagSet := cmd.Flags().Changed("port")
	isInteractive := term.IsTerminal(int(os.Stdin.Fd())) && !portFlagSet

	if isInteractive {
		return runCaptureTUI(cmd, out)
	}

	if flags.port == "" {
		return fmt.Errorf("serial port is required (use --port flag or run interactively)")
	}
	defaults := sources.DefaultConfig()
	defaults.Port = flags.port
	defaults.Baud = flags.baud
	serialCfg := &defaults

	out.Step(fmt.Sprintf("Starting serial capture: %s @ %d baud", serialCfg.Port, serialCfg.Baud))

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
		SerialConfig: serialCfg,
		RunLabel:     flags.name,
		Meta:         metaOpts,
	})
	if err != nil {
		return err
	}
	run := result.Run

	out.Blank()
	if result.Interrupted {
		out.Warning("Serial capture interrupted (partial run saved)")
	} else {
		out.Success("Serial capture complete")
	}
	out.KeyValue("Run ID", run.ID)
	out.KeyValue("Records", fmt.Sprintf("%d", run.RecordsCount))
	out.KeyValue("Location", filepath.Join(appCfg.OfflineCache, run.ID))
	out.Blank()
	out.Muted("Run 'astrolabe upload' to upload to server.")
	return nil
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
		fmt.Fprintf(&b, "%s=%s", k, v)
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
	maps.Copy(dst, src)
	return dst
}
