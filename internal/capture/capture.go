package capture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"go.bug.st/serial"
)

const DefaultOutputDir = "captures"

// Config holds the parameters required to execute a capture run.
type Config struct {
	Driver       string
	Port         string
	Baud         int
	RunID        string
	FirmwareHash string
	OutputFormat string
	UploadMode   string
	AutoUpload   bool
	OutputDir    string
}

// Result describes the files produced by a capture run.
type Result struct {
	RunID        string
	DataPath     string
	MetadataPath string
}

// Run executes the capture pipeline and returns the generated artifact paths.
func Run(cfg Config) (Result, error) {
	if cfg.Driver == "" {
		return Result{}, fmt.Errorf("capture driver is required")
	}

	// Probe the serial port early so we can surface errors before writing files.
	if strings.EqualFold(cfg.Driver, "serial") {
		mode := &serial.Mode{
			BaudRate: cfg.Baud,
			DataBits: 8,
			Parity:   serial.NoParity,
			StopBits: serial.OneStopBit,
		}
		port, err := serial.Open(cfg.Port, mode)
		if err != nil {
			return Result{}, fmt.Errorf("open serial port: %w", err)
		}
		port.Close()
	}

	outputDir := cfg.OutputDir
	if outputDir == "" {
		outputDir = DefaultOutputDir
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create capture dir: %w", err)
	}

	base := buildCaptureFileName(cfg.RunID)
	dataPath := filepath.Join(outputDir, base+"."+formatExtension(cfg.OutputFormat))
	metaPath := filepath.Join(outputDir, base+".meta.json")

	if err := writeMetadata(metaPath, cfg); err != nil {
		return Result{}, err
	}

	if err := writeSampleData(dataPath, cfg); err != nil {
		return Result{}, err
	}

	return Result{RunID: cfg.RunID, DataPath: dataPath, MetadataPath: metaPath}, nil
}

func writeMetadata(path string, cfg Config) error {
	meta := map[string]any{
		"driver":        strings.ToLower(cfg.Driver),
		"run_id":        cfg.RunID,
		"firmware_hash": cfg.FirmwareHash,
		"port":          cfg.Port,
		"baud":          cfg.Baud,
		"upload_mode":   strings.ToLower(cfg.UploadMode),
		"auto_upload":   cfg.AutoUpload,
		"output_format": strings.ToLower(cfg.OutputFormat),
		"created_at":    time.Now().UTC().Format(time.RFC3339Nano),
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create metadata: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(meta); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}

	return nil
}

func writeSampleData(path string, cfg Config) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create capture data: %w", err)
	}
	defer file.Close()

	record := map[string]any{
		"ts":      time.Now().UTC().Format(time.RFC3339Nano),
		"event":   "capture-started",
		"driver":  strings.ToLower(cfg.Driver),
		"run_id":  cfg.RunID,
		"message": "capture pipeline stub; connect hardware to stream data",
	}

	if strings.EqualFold(cfg.Driver, "serial") {
		record["port"] = cfg.Port
		record["baud"] = cfg.Baud
	}

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(record); err != nil {
		return fmt.Errorf("write capture data: %w", err)
	}

	return nil
}

func formatExtension(format string) string {
	switch strings.ToLower(format) {
	case "protobuf":
		return "pb"
	default:
		return "jsonl"
	}
}

func buildCaptureFileName(runID string) string {
	ts := time.Now().UTC().Format("20060102T150405Z")
	slug := sanitize(runID)
	if slug == "" {
		return fmt.Sprintf("capture_%s", ts)
	}
	return fmt.Sprintf("%s_%s", ts, slug)
}

func sanitize(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return ""
	}

	var b strings.Builder
	var lastDash bool

	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case unicode.IsSpace(r) || r == '-' || r == '_' || r == '.':
			if !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}

	return strings.Trim(b.String(), "-")
}
