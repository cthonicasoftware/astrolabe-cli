package root

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/config"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/core"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/tui"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/upload"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	uploadRunID string
	uploadForce bool
)

var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload cached runs to the server",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Load()

		// Validate configuration
		if cfg.APIURL == "" {
			return fmt.Errorf("API URL not configured (set via config file or ASTROLABE_API_URL env var)")
		}
		if cfg.AuthToken == "" {
			return fmt.Errorf("auth token not configured (set via config file or ASTROLABE_AUTH_TOKEN env var)")
		}
		if cfg.ProjectID == "" {
			return fmt.Errorf("project ID not configured (set via config file or ASTROLABE_PROJECT_ID env var)")
		}

		maxRetries := cfg.Upload.MaxRetries
		if maxRetries == 0 {
			maxRetries = 3 // Default
		}

		// Create upload client
		client := upload.NewClient(upload.Config{
			APIURL:     cfg.APIURL,
			AuthToken:  cfg.AuthToken,
			ProjectID:  cfg.ProjectID,
			CacheRoot:  cfg.OfflineCache,
			MaxRetries: maxRetries,
		})

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		// Check if we should use TUI or text mode
		isInteractive := term.IsTerminal(int(os.Stdout.Fd()))

		// Upload specific run or all pending runs
		if uploadRunID != "" {
			// Upload single run
			if isInteractive {
				if err := tui.RunUploadTUI(client, []string{uploadRunID}); err != nil {
					return fmt.Errorf("upload run: %w", err)
				}
			} else {
				fmt.Printf("Uploading run %s...\n", uploadRunID)
				if err := client.UploadRun(ctx, uploadRunID); err != nil {
					return fmt.Errorf("upload run: %w", err)
				}
				fmt.Printf("✓ Successfully uploaded run %s\n", uploadRunID)
			}
		} else {
			// Upload all pending runs
			runs, err := findPendingRuns(cfg.OfflineCache)
			if err != nil {
				return fmt.Errorf("find pending runs: %w", err)
			}

			if len(runs) == 0 {
				fmt.Println("No pending runs to upload")
				return nil
			}

			if isInteractive {
				// Use TUI for interactive mode
				if err := tui.RunUploadTUI(client, runs); err != nil {
					return err
				}
			} else {
				// Use text output for non-interactive (CI/scripts)
				fmt.Printf("Found %d pending run(s) to upload\n", len(runs))
				succeeded := 0
				failed := 0

				for i, runID := range runs {
					fmt.Printf("[%d/%d] Uploading %s...\n", i+1, len(runs), runID)
					if err := client.UploadRun(ctx, runID); err != nil {
						fmt.Fprintf(os.Stderr, "  ✗ Failed: %v\n", err)
						failed++
					} else {
						fmt.Printf("  ✓ Success\n")
						succeeded++
					}
				}

				fmt.Printf("\nUpload complete: %d succeeded, %d failed\n", succeeded, failed)
				if failed > 0 {
					return fmt.Errorf("%d run(s) failed to upload", failed)
				}
			}
		}

		return nil
	},
}

func init() {
	uploadCmd.Flags().StringVar(&uploadRunID, "run-id", "", "specific run ID to upload (default: all pending)")
	uploadCmd.Flags().BoolVar(&uploadForce, "force", false, "force upload even if offline flag set")
}

// findPendingRuns scans the cache directory for runs that haven't been uploaded yet.
// Runs are considered pending if:
// - No upload_state.json exists (never attempted)
// - upload_state.json exists but status != "succeeded" (failed or incomplete)
func findPendingRuns(cacheRoot string) ([]string, error) {
	entries, err := os.ReadDir(cacheRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var pending []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		runID := entry.Name()

		// Check if manifest exists (validates it's a run directory)
		manifestPath := filepath.Join(cacheRoot, runID, "manifest.json")
		if _, err := os.Stat(manifestPath); err != nil {
			continue
		}

		// Check upload state
		statePath := filepath.Join(cacheRoot, runID, "upload_state.json")
		stateData, err := os.ReadFile(statePath)
		if err != nil {
			// No state file means never uploaded
			pending = append(pending, runID)
			continue
		}

		// Parse upload state to check status
		var state core.UploadState
		if err := json.Unmarshal(stateData, &state); err != nil {
			// If we can't parse the state, treat as pending
			pending = append(pending, runID)
			continue
		}

		// Only skip runs that have successfully uploaded
		// Retry runs that failed, are in-flight, queued, or pending
		if state.Status != core.UploadStatusSucceeded {
			pending = append(pending, runID)
		}
	}

	return pending, nil
}
