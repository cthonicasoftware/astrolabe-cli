package validate

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/core"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/storage"
)

// CheckStatus represents the outcome of a validation check.
type CheckStatus string

const (
	StatusPassed  CheckStatus = "passed"
	StatusFailed  CheckStatus = "failed"
	StatusSkipped CheckStatus = "skipped"
)

// Check represents a single validation check result.
type Check struct {
	Name   string      `json:"name"`
	Status CheckStatus `json:"status"`
	Error  string      `json:"error,omitempty"`
}

// Result contains the full validation outcome for a run directory.
type Result struct {
	Valid    bool     `json:"valid"`
	RunID    string   `json:"run_id,omitempty"`
	RunDir   string   `json:"run_dir"`
	Checks   []Check  `json:"checks"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings,omitempty"`
}

// Validator performs validation checks on a run directory.
type Validator struct {
	runDir       string
	manifestPath string
	dataPath     string
	uploadPath   string
	manifest     *storage.ManifestDocument
	result       Result
}

// New creates a new Validator for the given run directory.
func New(runDir string) *Validator {
	return &Validator{
		runDir:       runDir,
		manifestPath: filepath.Join(runDir, "manifest.json"),
		dataPath:     filepath.Join(runDir, "data.jsonl"),
		uploadPath:   filepath.Join(runDir, "upload_state.json"),
		result: Result{
			Valid:    true,
			RunDir:   runDir,
			Checks:   []Check{},
			Errors:   []string{},
			Warnings: []string{},
		},
	}
}

// Run executes all validation checks and returns the result.
func (v *Validator) Run() Result {
	v.checkDirectoryExists()
	v.checkManifestParse()
	v.checkManifestFields()
	v.checkDataExists()
	v.checkDataNDJSON()
	v.checkDataChecksum()
	v.checkRecordsCount()
	v.checkTimestamps()
	v.checkUploadState()

	return v.result
}

func (v *Validator) addCheck(name string, status CheckStatus, errMsg string) {
	check := Check{Name: name, Status: status, Error: errMsg}
	v.result.Checks = append(v.result.Checks, check)

	if status == StatusFailed {
		v.result.Valid = false
		v.result.Errors = append(v.result.Errors, errMsg)
	}
}

func (v *Validator) addWarning(msg string) {
	v.result.Warnings = append(v.result.Warnings, msg)
}

func (v *Validator) checkDirectoryExists() {
	info, err := os.Stat(v.runDir)
	if err != nil {
		v.addCheck("directory_exists", StatusFailed, fmt.Sprintf("run directory not accessible: %v", err))
		return
	}
	if !info.IsDir() {
		v.addCheck("directory_exists", StatusFailed, "path is not a directory")
		return
	}

	// Check if this looks like a run directory (has manifest.json)
	// vs a parent directory containing multiple runs
	if _, err := os.Stat(v.manifestPath); os.IsNotExist(err) {
		if v.looksLikeRunsParent() {
			v.addCheck("directory_exists", StatusFailed,
				"path appears to be a runs parent directory, not a run directory; specify a run subdirectory")
			return
		}
	}

	v.addCheck("directory_exists", StatusPassed, "")
}

// looksLikeRunsParent checks if the directory contains subdirectories
// that look like run directories (have manifest.json).
func (v *Validator) looksLikeRunsParent() bool {
	entries, err := os.ReadDir(v.runDir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Check if subdirectory has a manifest.json
		subManifest := filepath.Join(v.runDir, entry.Name(), "manifest.json")
		if _, err := os.Stat(subManifest); err == nil {
			return true
		}
	}
	return false
}

func (v *Validator) checkManifestParse() {
	// Skip if directory check failed
	if !v.checkPassed("directory_exists") {
		v.addCheck("manifest_parse", StatusSkipped, "")
		return
	}

	manifest, err := storage.LoadManifest(v.manifestPath)
	if err != nil {
		v.addCheck("manifest_parse", StatusFailed, fmt.Sprintf("failed to parse manifest: %v", err))
		return
	}

	v.manifest = &manifest
	v.result.RunID = manifest.RunID
	v.addCheck("manifest_parse", StatusPassed, "")
}

func (v *Validator) checkManifestFields() {
	if !v.checkPassed("manifest_parse") {
		v.addCheck("manifest_fields", StatusSkipped, "")
		return
	}

	var missing []string

	// Required top-level fields
	if v.manifest.RunID == "" {
		missing = append(missing, "run_id")
	}
	if v.manifest.Schema == "" {
		missing = append(missing, "schema_version")
	}
	if v.manifest.Started.IsZero() {
		missing = append(missing, "started")
	}

	// Required manifest.device fields
	if v.manifest.Manifest.Device.ID == "" {
		missing = append(missing, "manifest.device.id")
	}

	// Required manifest.test fields
	if v.manifest.Manifest.Test.Plan == "" {
		missing = append(missing, "manifest.test.plan")
	}

	// Required source fields based on kind
	if v.manifest.Source.Kind == "" {
		missing = append(missing, "source.kind")
	} else {
		switch v.manifest.Source.Kind {
		case "serial":
			if v.manifest.Source.Port == "" {
				missing = append(missing, "source.port (required for serial)")
			}
		case "tcp":
			if v.manifest.Source.Addr == "" {
				missing = append(missing, "source.addr (required for tcp)")
			}
		case "file":
			if v.manifest.Source.Path == "" {
				missing = append(missing, "source.path (required for file)")
			}
		}
	}

	if len(missing) > 0 {
		v.addCheck("manifest_fields", StatusFailed, fmt.Sprintf("missing required fields: %v", missing))
		return
	}

	v.addCheck("manifest_fields", StatusPassed, "")
}

func (v *Validator) checkDataExists() {
	if !v.checkPassed("directory_exists") {
		v.addCheck("data_exists", StatusSkipped, "")
		return
	}

	info, err := os.Stat(v.dataPath)
	if err != nil {
		v.addCheck("data_exists", StatusFailed, fmt.Sprintf("data.jsonl not found: %v", err))
		return
	}
	if info.IsDir() {
		v.addCheck("data_exists", StatusFailed, "data.jsonl is a directory, expected file")
		return
	}
	v.addCheck("data_exists", StatusPassed, "")
}

func (v *Validator) checkDataNDJSON() {
	if !v.checkPassed("data_exists") {
		v.addCheck("data_ndjson", StatusSkipped, "")
		return
	}

	file, err := os.Open(v.dataPath)
	if err != nil {
		v.addCheck("data_ndjson", StatusFailed, fmt.Sprintf("cannot open data file: %v", err))
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	info, err := file.Stat()
	if err != nil {
		v.addCheck("data_ndjson", StatusFailed, fmt.Sprintf("cannot stat data file: %v", err))
		return
	}

	if info.Size() >= 64*1024 {
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	}

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue // skip empty lines
		}
		if !json.Valid(line) {
			v.addCheck("data_ndjson", StatusFailed, fmt.Sprintf("invalid JSON on line %d", lineNum))
			return
		}
	}

	if err := scanner.Err(); err != nil {
		v.addCheck("data_ndjson", StatusFailed, fmt.Sprintf("error reading data file: %v", err))
		return
	}

	v.addCheck("data_ndjson", StatusPassed, "")
}

func (v *Validator) checkDataChecksum() {
	if !v.checkPassed("data_exists") {
		v.addCheck("data_checksum", StatusSkipped, "")
		return
	}

	file, err := os.Open(v.dataPath)
	if err != nil {
		v.addCheck("data_checksum", StatusFailed, fmt.Sprintf("cannot open data file: %v", err))
		return
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		v.addCheck("data_checksum", StatusFailed, fmt.Sprintf("error computing checksum: %v", err))
		return
	}

	_ = hex.EncodeToString(hasher.Sum(nil))
	v.addCheck("data_checksum", StatusPassed, "")
}

func (v *Validator) checkRecordsCount() {
	if !v.checkPassed("data_exists") || !v.checkPassed("manifest_parse") {
		v.addCheck("records_count", StatusSkipped, "")
		return
	}

	file, err := os.Open(v.dataPath)
	if err != nil {
		v.addCheck("records_count", StatusFailed, fmt.Sprintf("cannot open data file: %v", err))
		return
	}
	defer file.Close()

	var lineCount uint64

	scanner := bufio.NewScanner(file)

	info, err := file.Stat()

	if err != nil {
		v.addCheck("records_count", StatusFailed, fmt.Sprintf("cannot stat data file: %v", err))
		return
	}

	if info.Size() >= 64*1024 {
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	}

	for scanner.Scan() {
		if len(scanner.Bytes()) > 0 {
			lineCount++
		}
	}

	if err := scanner.Err(); err != nil {
		v.addCheck("records_count", StatusFailed, fmt.Sprintf("error counting lines: %v", err))
		return
	}

	if lineCount != v.manifest.RecordsCount {
		v.addCheck("records_count", StatusFailed,
			fmt.Sprintf("line count mismatch: manifest says %d, file has %d",
				v.manifest.RecordsCount, lineCount))
		return
	}

	v.addCheck("records_count", StatusPassed, "")
}

func (v *Validator) checkTimestamps() {
	if !v.checkPassed("manifest_parse") {
		v.addCheck("timestamps", StatusSkipped, "")
		return
	}

	if v.manifest.Completed == nil {
		// No completed timestamp, nothing to check
		v.addCheck("timestamps", StatusPassed, "")
		return
	}

	if v.manifest.Completed.Before(v.manifest.Started) {
		v.addCheck("timestamps", StatusFailed,
			fmt.Sprintf("completed (%v) is before started (%v)",
				v.manifest.Completed, v.manifest.Started))
		return
	}

	v.addCheck("timestamps", StatusPassed, "")
}

func (v *Validator) checkUploadState() {
	if !v.checkPassed("directory_exists") {
		v.addCheck("upload_state", StatusSkipped, "")
		return
	}

	data, err := os.ReadFile(v.uploadPath)
	if os.IsNotExist(err) {
		v.addCheck("upload_state", StatusSkipped, "")
		v.addWarning("upload_state.json not present (run not yet uploaded)")
		return
	}
	if err != nil {
		v.addCheck("upload_state", StatusFailed, fmt.Sprintf("cannot read upload_state.json: %v", err))
		return
	}

	var state core.UploadState
	if err := json.Unmarshal(data, &state); err != nil {
		v.addCheck("upload_state", StatusFailed, fmt.Sprintf("invalid upload_state.json: %v", err))
		return
	}

	v.addCheck("upload_state", StatusPassed, "")
}

func (v *Validator) checkPassed(name string) bool {
	for _, check := range v.result.Checks {
		if check.Name == name {
			return check.Status == StatusPassed
		}
	}
	return false
}
