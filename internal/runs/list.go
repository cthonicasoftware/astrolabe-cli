package runs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/core"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/storage"
)

// Summary provides a high-level view of a cached capture run.
type Summary struct {
	ID              string               `json:"run_id"`
	RunDir          string               `json:"run_dir"`
	Started         time.Time            `json:"started"`
	Completed       *time.Time           `json:"completed,omitempty"`
	DurationSeconds float64              `json:"duration_seconds"`
	Records         uint64               `json:"records_count"`
	Source          core.SourceMeta      `json:"source"`
	Manifest        core.Manifest        `json:"manifest"`
	Capture         core.CaptureSettings `json:"capture"`
	PrimaryData     string               `json:"primary_data_uri"`
	DataSizeBytes   int64                `json:"data_size_bytes,omitempty"`
}

// Warning captures non-fatal issues encountered while enumerating run directories.
type Warning struct {
	RunDir string `json:"run_dir"`
	Reason string `json:"reason"`
}

// Result is returned by List and aggregates cached runs and any warnings.
type Result struct {
	Root     string    `json:"root"`
	Runs     []Summary `json:"runs"`
	Warnings []Warning `json:"warnings,omitempty"`
}

// List enumerates run directories beneath root, parsing manifest files into run summaries.
func List(root string) (Result, error) {
	result := Result{Root: root}

	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("list runs: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		runDir := filepath.Join(root, entry.Name())
		manifestPath := filepath.Join(runDir, "manifest.json")
		doc, err := storage.LoadManifest(manifestPath)
		if errors.Is(err, os.ErrNotExist) {
			result.Warnings = append(result.Warnings, Warning{
				RunDir: runDir,
				Reason: "missing manifest.json",
			})
			continue
		}
		if err != nil {
			result.Warnings = append(result.Warnings, Warning{
				RunDir: runDir,
				Reason: err.Error(),
			})
			continue
		}

		summary := Summary{
			ID:          doc.RunID,
			RunDir:      runDir,
			Started:     doc.Started,
			Completed:   doc.Completed,
			Records:     doc.RecordsCount,
			Source:      doc.Source,
			Manifest:    doc.Manifest,
			Capture:     doc.Capture,
			PrimaryData: doc.PrimaryData,
		}

		if doc.Completed != nil && !doc.Completed.Before(doc.Started) {
			summary.DurationSeconds = doc.Completed.Sub(doc.Started).Seconds()
		}

		if summary.PrimaryData == "" {
			// Fallback to conventional location.
			summary.PrimaryData = filepath.Join(runDir, "data.jsonl")
		}

		if info, err := os.Stat(summary.PrimaryData); err == nil {
			summary.DataSizeBytes = info.Size()
		}

		result.Runs = append(result.Runs, summary)
	}

	sort.Slice(result.Runs, func(i, j int) bool {
		return result.Runs[i].Started.After(result.Runs[j].Started)
	})

	return result, nil
}
