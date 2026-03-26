package runs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
)

// FindPending returns run IDs that have not successfully uploaded yet.
func FindPending(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list pending runs: %w", err)
	}

	var pending []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		runID := entry.Name()
		manifestPath := filepath.Join(root, runID, "manifest.json")
		if _, err := os.Stat(manifestPath); err != nil {
			continue
		}

		statePath := filepath.Join(root, runID, "upload_state.json")
		stateData, err := os.ReadFile(statePath)
		if errors.Is(err, os.ErrNotExist) {
			pending = append(pending, runID)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read upload state for %s: %w", runID, err)
		}

		var state core.UploadState
		if err := json.Unmarshal(stateData, &state); err != nil {
			pending = append(pending, runID)
			continue
		}
		if state.Status != core.UploadStatusSucceeded {
			pending = append(pending, runID)
		}
	}

	return pending, nil
}
