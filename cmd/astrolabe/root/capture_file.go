package root

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/capture"
	"github.com/cthonicasoftware/astrolabe-cli/internal/cliout"
	"github.com/cthonicasoftware/astrolabe-cli/internal/config"
	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/normalize"
	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
	"github.com/cthonicasoftware/astrolabe-cli/internal/storage"
	"github.com/spf13/cobra"
)

var (
	fileFormat      string
	fileSkipLines   int
	fileDelimiter   string
	fileNoHeaders   bool
	fileColumnNames []string

	// Metadata flags (same as serial)
	fileOperator        string
	fileLocation        string
	fileDeviceID        string
	fileDeviceSerial    string
	fileDeviceFirmware  string
	fileDeviceFWHash    string
	fileDeviceHWVersion string
	fileTestPlan        string
	fileTestVariant     string
	fileTestRun         string
	fileTags            []string
	fileAttributes      = map[string]string{}
)

var captureFileCmd = &cobra.Command{
	Use:   "file <path>",
	Short: "Ingest data from a file (CSV, JSONL, or raw logs)",
	Long: `Import existing data files into the Astrolabe system.

Supported formats:
  - csv:    Comma-separated values with optional headers
  - jsonl:  Newline-delimited JSON (one JSON object per line)
  - raw:    Plain text logs (one record per line)

Examples:
  # Import CSV with headers
  astrolabe capture file data.csv --format csv

  # Import CSV without headers (auto-generate column names)
  astrolabe capture file data.csv --format csv --no-headers

  # Import CSV with custom delimiter
  astrolabe capture file data.tsv --format csv --delimiter '\t'

  # Import raw log file
  astrolabe capture file app.log --format raw

  # Import JSONL file
  astrolabe capture file events.jsonl --format jsonl

  # Skip first 2 lines (e.g., comments in file)
  astrolabe capture file data.csv --format csv --skip-lines 2`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]

		// Create styled printer (check for --json flag from root command)
		jsonMode, _ := cmd.Flags().GetBool("json")
		out := cliout.DefaultPrinter(jsonMode)

		// Validate file exists
		absPath, err := filepath.Abs(filePath)
		if err != nil {
			return fmt.Errorf("invalid file path: %w", err)
		}

		info, err := os.Stat(absPath)
		if err != nil {
			return fmt.Errorf("cannot access file: %w", err)
		}

		if info.IsDir() {
			return fmt.Errorf("path is a directory, not a file: %s", absPath)
		}

		// Auto-detect format if not specified
		if fileFormat == "" {
			fileFormat = detectFormat(absPath)
			out.Muted(fmt.Sprintf("Auto-detected format: %s", fileFormat))
		}

		// Validate format
		if !isValidFormat(fileFormat) {
			return fmt.Errorf("invalid format: %s (must be csv, jsonl, or raw)", fileFormat)
		}

		out.Step(fmt.Sprintf("Ingesting file: %s (format: %s)", absPath, fileFormat))

		// Load metadata defaults
		var savedMetadata config.Metadata
		if meta, err := config.LoadMetadata(); err == nil {
			savedMetadata = meta
		} else {
			out.Warning(fmt.Sprintf("Failed to load metadata: %v", err))
		}

		// Parse flags
		flagTags, err := parseTagFlags(fileTags)
		if err != nil {
			return fmt.Errorf("invalid --tag value: %w", err)
		}
		flagAttrs, err := parseAttributeFlags(fileAttributes)
		if err != nil {
			return fmt.Errorf("invalid --attr value: %w", err)
		}

		// Create file source
		fileCfg := sources.FileConfig{
			Path:      absPath,
			ChunkSize: 0, // line-by-line
			SkipLines: fileSkipLines,
			Follow:    false,
		}

		fileSource, err := sources.NewFileWithConfig(fileCfg)
		if err != nil {
			return fmt.Errorf("failed to create file source: %w", err)
		}

		// Create normalizer based on format
		var normalizer normalize.Normalizer
		switch fileFormat {
		case "csv":
			csvCfg := normalize.DefaultCSVConfig()
			csvCfg.HasHeaders = !fileNoHeaders
			if fileDelimiter != "" {
				if len(fileDelimiter) != 1 {
					return fmt.Errorf("delimiter must be a single character")
				}
				csvCfg.Delimiter = rune(fileDelimiter[0])
			}
			if len(fileColumnNames) > 0 {
				csvCfg.ColumnNames = fileColumnNames
			}
			normalizer = normalize.NewCSVWithConfig(csvCfg)
		case "jsonl":
			normalizer = normalize.NewLineJSON()
		case "raw":
			normalizer = normalize.NewRaw()
		default:
			return fmt.Errorf("unsupported format: %s", fileFormat)
		}

		// Setup storage
		appCfg := config.Load()
		if err := os.MkdirAll(appCfg.OfflineCache, 0o755); err != nil {
			return fmt.Errorf("ensure offline cache: %w", err)
		}

		// Create permanent run directory
		runID := generateRunID()
		runRoot := filepath.Join(appCfg.OfflineCache, runID)
		if err := os.MkdirAll(runRoot, 0o755); err != nil {
			return fmt.Errorf("create run dir: %w", err)
		}

		store := storage.NewFS(runRoot)

		// Build manifest
		meta := core.ManifestOptions{
			Operator: fileOperator,
			Location: fileLocation,
			Device: core.DeviceInfo{
				ID:              fileDeviceID,
				Serial:          fileDeviceSerial,
				Firmware:        fileDeviceFirmware,
				FirmwareHash:    fileDeviceFWHash,
				HardwareVersion: fileDeviceHWVersion,
			},
			Test: core.TestInfo{
				Plan:    fileTestPlan,
				Variant: fileTestVariant,
				Run:     fileTestRun,
			},
			Tags:       append([]string(nil), flagTags...),
			Attributes: cloneStringMap(flagAttrs),
		}
		applyMetadataDefaults(&meta, savedMetadata, cmd.Flags())

		manifest := buildFileManifest(absPath, fileFormat, meta)
		captureSettings := buildFileCaptureSettings(absPath, fileFormat)

		// Create pipeline
		pipelineOpts := capture.Options{
			Source:     fileSource,
			Normalizer: normalizer,
			Store:      store,
			Manifest:   manifest,
			Capture:    captureSettings,
			RunID:      runID,
		}

		pipeline, err := capture.NewPipeline(pipelineOpts)
		if err != nil {
			return fmt.Errorf("build pipeline: %w", err)
		}

		// Open file source
		ctx := context.Background()
		if err := fileSource.Open(ctx); err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		defer fileSource.Close()

		// Run capture pipeline
		out.Step("Processing file...")
		run, err := pipeline.Run(ctx)
		if err != nil {
			return fmt.Errorf("capture failed: %w", err)
		}

		// Success
		out.Blank()
		out.Success("File ingestion complete")
		out.KeyValue("Run ID", run.ID)
		out.KeyValue("Records", fmt.Sprintf("%d", run.RecordsCount))
		out.KeyValue("Location", runRoot)
		out.Blank()
		out.Muted("Run 'astrolabe upload' to upload to server.")

		return nil
	},
}

func buildFileManifest(filePath, format string, opts core.ManifestOptions) core.Manifest {
	attrs := map[string]string{
		"source_kind":   "file",
		"source_file":   filepath.Base(filePath),
		"source_format": format,
		"source_path":   filePath,
	}

	for k, v := range opts.Attributes {
		if k == "" || v == "" {
			continue
		}
		attrs[k] = v
	}

	manifest := core.Manifest{
		SchemaVersion: Schema,
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

func buildFileCaptureSettings(filePath, format string) core.CaptureSettings {
	return core.CaptureSettings{
		Notes: fmt.Sprintf("File ingestion: %s (format: %s)", filepath.Base(filePath), format),
	}
}

func detectFormat(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".csv":
		return "csv"
	case ".jsonl", ".ndjson":
		return "jsonl"
	case ".log", ".txt":
		return "raw"
	default:
		// Default to raw for unknown extensions
		return "raw"
	}
}

func isValidFormat(format string) bool {
	switch format {
	case "csv", "jsonl", "raw":
		return true
	default:
		return false
	}
}

func generateRunID() string {
	// Format: run-YYYYMMDD-HHMMSS
	// This matches the default from capture/pipeline.go
	now := time.Now().UTC()
	return fmt.Sprintf("run-%s", now.Format("20060102-150405"))
}

func init() {
	captureCmd.AddCommand(captureFileCmd)

	// File-specific flags
	captureFileCmd.Flags().StringVar(&fileFormat, "format", "", "File format: csv, jsonl, or raw (auto-detected if not specified)")
	captureFileCmd.Flags().IntVar(&fileSkipLines, "skip-lines", 0, "Number of lines to skip at start of file")

	// CSV-specific flags
	captureFileCmd.Flags().StringVar(&fileDelimiter, "delimiter", ",", "CSV field delimiter")
	captureFileCmd.Flags().BoolVar(&fileNoHeaders, "no-headers", false, "CSV has no header row (auto-generate column names)")
	captureFileCmd.Flags().StringSliceVar(&fileColumnNames, "columns", nil, "CSV column names (overrides header row)")

	// Metadata flags (mirror serial command)
	captureFileCmd.Flags().StringVar(&fileOperator, "operator", "", "Operator name")
	captureFileCmd.Flags().StringVar(&fileLocation, "location", "", "Test location/bench identifier")
	captureFileCmd.Flags().StringVar(&fileDeviceID, "device-id", "", "Device identifier")
	captureFileCmd.Flags().StringVar(&fileDeviceSerial, "device-serial", "", "Device serial number")
	captureFileCmd.Flags().StringVar(&fileDeviceFirmware, "device-firmware", "", "Device firmware version")
	captureFileCmd.Flags().StringVar(&fileDeviceFWHash, "device-firmware-hash", "", "Device firmware hash")
	captureFileCmd.Flags().StringVar(&fileDeviceHWVersion, "device-hardware-version", "", "Device hardware version")
	captureFileCmd.Flags().StringVar(&fileTestPlan, "test-plan", "", "Test plan name")
	captureFileCmd.Flags().StringVar(&fileTestVariant, "test-variant", "", "Test variant")
	captureFileCmd.Flags().StringVar(&fileTestRun, "test-run", "", "Test run identifier")
	captureFileCmd.Flags().StringSliceVar(&fileTags, "tag", nil, "Tag (can be repeated)")
	captureFileCmd.Flags().StringToStringVar(&fileAttributes, "attr", nil, "Custom attribute key=value (can be repeated)")
}
