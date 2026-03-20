package root

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cthonicasoftware/astrolabe-cli/internal/cliout"
	"github.com/cthonicasoftware/astrolabe-cli/internal/config"
	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/normalize"
	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	serialPort string
	serialBaud int
	serialName string
)

var captureSerialCmd = &cobra.Command{
	Use:   "serial",
	Short: "Capture from a serial port",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonMode, _ := cmd.Flags().GetBool("json")
		out := cliout.DefaultPrinter(jsonMode)

		portFlagSet := cmd.Flags().Changed("port")
		isInteractive := term.IsTerminal(int(os.Stdin.Fd())) && !portFlagSet

		if isInteractive {
			return runCaptureTUI(out)
		}

		if serialPort == "" {
			return fmt.Errorf("serial port is required (use --port flag or run interactively)")
		}
		defaults := sources.DefaultConfig()
		defaults.Port = serialPort
		defaults.Baud = serialBaud
		serialCfg := &defaults

		out.Step(fmt.Sprintf("Starting serial capture: %s @ %d baud", serialCfg.Port, serialCfg.Baud))

		savedMetadata, metaErr := config.LoadMetadata()
		if metaErr != nil {
			out.Warning(fmt.Sprintf("Failed to load metadata: %v", metaErr))
		}

		flagTags, err := parseTagFlags(captureTags)
		if err != nil {
			return fmt.Errorf("invalid --tag value: %w", err)
		}
		flagAttrs, err := parseAttributeFlags(captureAttributes)
		if err != nil {
			return fmt.Errorf("invalid --attr value: %w", err)
		}

		meta := buildManifestOptions(captureMetadataInput{
			Operator:        captureOperator,
			Location:        captureLocation,
			DeviceID:        captureDeviceID,
			DeviceSerial:    captureDeviceSerial,
			DeviceFirmware:  captureDeviceFirmware,
			DeviceFWHash:    captureDeviceFWHash,
			DeviceHWVersion: captureDeviceHWVersion,
			TestPlan:        captureTestPlan,
			TestVariant:     captureTestVariant,
			TestRun:         captureTestRun,
			Tags:            flagTags,
			Attributes:      flagAttrs,
		}, savedMetadata, cmd.Flags(), metaErr == nil)

		manifest := buildSerialManifest(*serialCfg, serialName, meta)
		captureSettings := buildSerialCaptureSettings(*serialCfg)
		serial := sources.NewSerialWithConfig(*serialCfg)
		appCfg, err := config.Load()
		if err != nil {
			return err
		}

		out.Info("Capturing... (press Ctrl+C to stop)")
		out.Blank()
		run, interrupted, err := runHeadlessCapture(out, serial, normalize.NewLineJSON(), appCfg.OfflineCache, manifest, captureSettings)
		if err != nil {
			return err
		}

		out.Blank()
		if interrupted {
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
	},
}

func init() {
	captureCmd.AddCommand(captureSerialCmd)
	captureSerialCmd.Flags().StringVarP(&serialPort, "port", "p", "/dev/ttyUSB0", "serial port path")
	captureSerialCmd.Flags().IntVarP(&serialBaud, "baud", "b", 115200, "baud rate")
	captureSerialCmd.Flags().StringVar(&serialName, "name", "", "optional run name")
}

func buildSerialManifest(cfg sources.Config, name string, opts core.ManifestOptions) core.Manifest {
	attrs := map[string]string{
		"source_kind": "serial",
		"port":        cfg.Port,
		"baud":        strconv.Itoa(cfg.Baud),
	}
	if name != "" {
		attrs["run_label"] = name
	}
	return buildCaptureManifest(opts, attrs)
}

func buildSerialCaptureSettings(cfg sources.Config) core.CaptureSettings {
	return core.CaptureSettings{
		Channels: []string{"serial"},
		Notes:    fmt.Sprintf("serial capture from %s @ %d baud", cfg.Port, cfg.Baud),
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
