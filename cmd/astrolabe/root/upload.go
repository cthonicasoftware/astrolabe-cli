package root

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/config"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/upload"
	"github.com/spf13/cobra"
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

		// Upload specific run or all pending runs
		if uploadRunID != "" {
			// Upload single run
			fmt.Printf("Uploading run %s...\n", uploadRunID)
			if err := client.UploadRun(ctx, uploadRunID); err != nil {
				return fmt.Errorf("upload run: %w", err)
			}
			fmt.Printf("✓ Successfully uploaded run %s\n", uploadRunID)
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

		return nil
	},
}

func init() {
	uploadCmd.Flags().StringVar(&uploadRunID, "run-id", "", "specific run ID to upload (default: all pending)")
	uploadCmd.Flags().BoolVar(&uploadForce, "force", false, "force upload even if offline flag set")
}

// findPendingRuns scans the cache directory for runs that haven't been uploaded yet.
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
		if _, err := os.Stat(statePath); err != nil {
			// No state file means never uploaded
			pending = append(pending, runID)
		}
		// If upload_state.json exists, skip it for now
		// In the future, we could check the status and retry failed uploads
	}

	return pending, nil
}
