package root

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/cthonicasoftware/astrolabe-cli/internal/cliout"
	"github.com/cthonicasoftware/astrolabe-cli/internal/config"
	"github.com/cthonicasoftware/astrolabe-cli/internal/normalize"
	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
	"github.com/spf13/cobra"
)

type fileFlags struct {
	format      string
	skipLines   int
	delimiter   string
	noHeaders   bool
	columnNames []string
}

func newCaptureFileCmd(meta *captureMetadataFlags) *cobra.Command {
	var flags fileFlags
	cmd := &cobra.Command{
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
			return runCaptureFile(cmd, args[0], flags, meta)
		},
	}
	cmd.Flags().StringVar(&flags.format, "format", "", "File format: csv, jsonl, or raw (auto-detected if not specified)")
	cmd.Flags().IntVar(&flags.skipLines, "skip-lines", 0, "Number of lines to skip at start of file")
	cmd.Flags().StringVar(&flags.delimiter, "delimiter", ",", "CSV field delimiter")
	cmd.Flags().BoolVar(&flags.noHeaders, "no-headers", false, "CSV has no header row (auto-generate column names)")
	cmd.Flags().StringSliceVar(&flags.columnNames, "columns", nil, "CSV column names (overrides header row)")
	return cmd
}

func runCaptureFile(cmd *cobra.Command, filePath string, flags fileFlags, meta *captureMetadataFlags) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	out := cliout.DefaultPrinter(jsonMode)

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

	format := flags.format
	if format == "" {
		format = detectFormat(absPath)
		out.Muted(fmt.Sprintf("Auto-detected format: %s", format))
	}

	if !isValidFormat(format) {
		return fmt.Errorf("invalid format: %s (must be csv, jsonl, or raw)", format)
	}

	out.Step(fmt.Sprintf("Ingesting file: %s (format: %s)", absPath, format))

	var savedMetadata config.Metadata
	if loaded, err := config.LoadMetadata(); err == nil {
		savedMetadata = loaded
	} else {
		out.Warning(fmt.Sprintf("Failed to load metadata: %v", err))
	}

	flagTags, err := parseTagFlags(meta.tags)
	if err != nil {
		return fmt.Errorf("invalid --tag value: %w", err)
	}
	flagAttrs, err := parseAttributeFlags(meta.attributes)
	if err != nil {
		return fmt.Errorf("invalid --attr value: %w", err)
	}

	fileCfg := sources.FileConfig{
		Path:      absPath,
		ChunkSize: 0, // line-by-line
		SkipLines: flags.skipLines,
		Follow:    false,
	}

	var normalizer normalize.Normalizer
	switch format {
	case "csv":
		csvCfg := normalize.DefaultCSVConfig()
		csvCfg.HasHeaders = !flags.noHeaders
		delimiter, err := parseDelimiter(flags.delimiter)
		if err != nil {
			return err
		}
		csvCfg.Delimiter = delimiter
		if len(flags.columnNames) > 0 {
			csvCfg.ColumnNames = flags.columnNames
		}
		normalizer = normalize.NewCSVWithConfig(csvCfg)
	case "jsonl":
		normalizer = normalize.NewLineJSON()
	case "raw":
		normalizer = normalize.NewRaw()
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	appCfg, err := configFromCmd(cmd)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(appCfg.OfflineCache, 0o755); err != nil {
		return fmt.Errorf("ensure offline cache: %w", err)
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
	}, savedMetadata, cmd.Flags(), true)

	out.Step("Processing file...")
	svc := newCaptureService(out, appCfg.OfflineCache)
	result, err := svc.Run(cmd.Context(), CaptureRequest{
		Source:     fileCfg,
		Normalizer: normalizer,
		Meta:       metaOpts,
	})
	if err != nil {
		return err
	}
	run, interrupted := result.Run, result.Interrupted

	out.Blank()
	if interrupted {
		out.Warning("File ingestion interrupted (partial run saved)")
	} else {
		out.Success("File ingestion complete")
	}
	out.KeyValue("Run ID", run.ID)
	out.KeyValue("Records", fmt.Sprintf("%d", run.RecordsCount))
	out.KeyValue("Location", filepath.Join(appCfg.OfflineCache, run.ID))
	out.Blank()
	out.Muted("Run 'astrolabe upload' to upload to server.")

	return nil
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

func parseDelimiter(value string) (rune, error) {
	if value == "" {
		return ',', nil
	}
	switch value {
	case `\t`:
		return '\t', nil
	case `\n`:
		return '\n', nil
	case `\r`:
		return '\r', nil
	}

	if strings.HasPrefix(value, `\u`) && len(value) == 6 {
		n, err := strconv.ParseInt(value[2:], 16, 32)
		if err != nil {
			return 0, fmt.Errorf("invalid unicode delimiter: %w", err)
		}
		return rune(n), nil
	}

	if utf8.RuneCountInString(value) != 1 {
		return 0, fmt.Errorf("delimiter must be a single character")
	}
	r, _ := utf8.DecodeRuneInString(value)
	return r, nil
}
