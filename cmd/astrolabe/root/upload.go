package root

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cthonicasoftware/astrolabe-cli/internal/cliout"
	"github.com/cthonicasoftware/astrolabe-cli/internal/config"
	"github.com/cthonicasoftware/astrolabe-cli/internal/core"
	"github.com/cthonicasoftware/astrolabe-cli/internal/tui"
	"github.com/cthonicasoftware/astrolabe-cli/internal/upload"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	uploadRunID string
	uploadForce bool
)

const defaultUploadTimeout = 30 * time.Minute

var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload cached runs to the server",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create styled printer
		jsonMode, _ := cmd.Flags().GetBool("json")
		out := cliout.DefaultPrinter(jsonMode)

		cfg, err := config.Load()
		if err != nil {
			return err
		}

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
				out.Step(fmt.Sprintf("Uploading run: %s", uploadRunID))
				if err := uploadRunWithTimeout(context.Background(), defaultUploadTimeout, uploadRunID, client.UploadRun); err != nil {
					return fmt.Errorf("upload run: %w", err)
				}
				out.Blank()
				out.Success("Run uploaded successfully")
				out.KeyValue("Run ID", uploadRunID)
			}
		} else {
			// Upload all pending runs
			runs, err := findPendingRuns(cfg.OfflineCache)
			if err != nil {
				return fmt.Errorf("find pending runs: %w", err)
			}

			if len(runs) == 0 {
				out.Info("No pending runs to upload")
				out.Blank()
				out.Muted("Capture data with 'astrolabe capture' commands to create runs.")
				return nil
			}

			if isInteractive {
				// Use TUI for interactive mode
				if err := tui.RunUploadTUI(client, runs); err != nil {
					return err
				}
			} else {
				// Use text output for non-interactive (CI/scripts)
				out.Step(fmt.Sprintf("Found %d pending run(s) to upload", len(runs)))
				out.Blank()

				succeeded := 0
				failed := 0

				for i, runID := range runs {
					out.Step(fmt.Sprintf("[%d/%d] Uploading %s", i+1, len(runs), runID))
					if err := uploadRunWithTimeout(context.Background(), defaultUploadTimeout, runID, client.UploadRun); err != nil {
						out.Error(fmt.Sprintf("Failed: %v", err))
						failed++
					} else {
						succeeded++
						out.Success("Uploaded successfully")
					}
				}

				out.Blank()
				if failed == 0 {
					out.Success(fmt.Sprintf("All %d run(s) uploaded successfully", succeeded))
				} else {
					out.Warning(fmt.Sprintf("Upload complete: %d succeeded, %d failed", succeeded, failed))
					return fmt.Errorf("%d run(s) failed to upload", failed)
				}
			}
		}

		return nil
	},
}

func uploadRunWithTimeout(parent context.Context, timeout time.Duration, runID string, fn func(context.Context, string) error) error {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	return fn(ctx, runID)
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
