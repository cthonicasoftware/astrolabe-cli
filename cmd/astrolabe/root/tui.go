package root

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/capture"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/config"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/core"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/normalize"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/sources"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/storage"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the interactive Text UI",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Main TUI loop - keep showing welcome screen until user quits
		var status *tui.StatusMessage
		for {
			action, err := tui.RunWelcome(status)
			if err != nil {
				return err
			}
			status = nil
			// If no action selected (user quit), exit
			if action == "" {
				return nil
			}
			// Handle the selected action
			switch action {
			case "capture":
				serialCfg, launchTUI, err := tui.RunSerialPrompt()
				if err != nil {
					return err
				}
				savedMetadata, metaErr := config.LoadMetadata()
				if metaErr != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to load metadata: %v\n", metaErr)
				}
				if launchTUI {
					appCfg := config.Load()
					if err := os.MkdirAll(appCfg.OfflineCache, 0o755); err != nil {
						return fmt.Errorf("ensure offline cache: %w", err)
					}
					tempRoot, err := os.MkdirTemp(appCfg.OfflineCache, ".tmp-run-")
					if err != nil {
						return fmt.Errorf("create temp run dir: %w", err)
					}
					defer os.RemoveAll(tempRoot)
					store := storage.NewFS(tempRoot)
					normalizer := normalize.NewLineJSON()
					meta := serialManifestOptions{}
					if metaErr == nil {
						applyMetadataDefaults(&meta, savedMetadata, nil)
					}
					if meta.Operator == "" {
						meta.Operator = os.Getenv("USER")
					}
					if meta.Test.Plan == "" {
						meta.Test.Plan = "unspecified"
					}
					serial := sources.NewSerialWithConfig(*serialCfg)
					pipelineOpts := capture.Options{
						Source:     serial,
						Normalizer: normalizer,
						Store:      store,
						Manifest:   buildSerialManifest(*serialCfg, "", meta),
						Capture:    buildSerialCaptureSettings(*serialCfg),
					}
					pipeline, err := capture.NewPipeline(pipelineOpts)
					if err != nil {
						return fmt.Errorf("build pipeline: %w", err)
					}
					ctx, cancel := context.WithCancel(context.Background())
					if err := serial.Open(ctx); err != nil {
						cancel()
						return fmt.Errorf("failed to open serial port: %w", err)
					}
					stringCh := make(chan string, 16)
					pipelineFramesCh := make(chan []byte, 16)
					pipelineSource := &frameChannelSource{
						framesCh: pipelineFramesCh,
						meta:     serial.Meta(),
					}
					pipelineOpts.Source = pipelineSource
					pipeline, err = capture.NewPipeline(pipelineOpts)
					if err != nil {
						return fmt.Errorf("rebuild pipeline with wrapper source: %w", err)
					}
					go func() {
						defer close(stringCh)
						defer close(pipelineFramesCh)
						for frame := range serial.Frames() {
							if len(frame) == 0 {
								continue
							}
							select {
							case stringCh <- string(frame):
							case <-ctx.Done():
								return
							}
							select {
							case pipelineFramesCh <- frame:
							case <-ctx.Done():
								return
							}
						}
					}()
					pipelineResultCh := make(chan *core.Run, 1)
					pipelineErrCh := make(chan error, 1)
					go func() {
						run, err := pipeline.Run(ctx)
						pipelineResultCh <- run
						pipelineErrCh <- err
					}()
					title := fmt.Sprintf("Serial Capture - %s @ %d", serialCfg.Port, serialCfg.Baud)
					m := tui.NewApp(title, stringCh)
					p := tea.NewProgram(m, tea.WithAltScreen())
					finalModel, err := p.Run()
					if err != nil {
						cancel()
						return err
					}
					tuiApp, ok := finalModel.(*tui.App)
					if !ok {
						cancel()
						serial.Close()
						return fmt.Errorf("unexpected model type: %T", finalModel)
					}
					cancel()
					if tuiApp.SaveRequested() {
						fmt.Println("\nSaving capture data...")
						run := <-pipelineResultCh
						runErr := <-pipelineErrCh
						if runErr != nil && !errors.Is(runErr, context.Canceled) {
							serial.Close()
							return fmt.Errorf("capture pipeline: %w", runErr)
						}
						if run == nil {
							serial.Close()
							return fmt.Errorf("capture pipeline: run not returned")
						}
						if err := promoteRunArtifacts(run, tempRoot, appCfg.OfflineCache); err != nil {
							serial.Close()
							return fmt.Errorf("finalize run artifacts: %w", err)
						}
						fmt.Printf("Capture saved. Records: %d\n", run.RecordsCount)
						fmt.Printf("Run ID: %s\n", run.ID)
						fmt.Printf("Cache dir: %s\n", appCfg.OfflineCache)
						for _, artifact := range run.Artifacts {
							fmt.Printf(" - %s (%s)\n", artifact.Path, artifact.Role)
						}
						status = tui.NewStatusMessage(tui.StatusSuccess, "Run Saved", fmt.Sprintf("Run %s saved. Select 'View Runs' to inspect artifacts.", run.ID))
					} else {
						fmt.Println("\nExited without saving.")
						run := <-pipelineResultCh
						runErr := <-pipelineErrCh
						if runErr != nil && !errors.Is(runErr, context.Canceled) {
							fmt.Fprintf(os.Stderr, "capture pipeline error: %v\n", runErr)
						}
						if run != nil {
							runDir := filepath.Join(tempRoot, run.ID)
							_ = os.RemoveAll(runDir)
						}
						status = tui.NewStatusMessage(tui.StatusInfo, "Run Discarded", "Capture discarded. Start a new run when you are ready.")
					}
					if err := serial.Close(); err != nil {
						fmt.Fprintf(os.Stderr, "warning: failed to close serial port: %v\n", err)
					}
				}
				// Return to welcome screen (continue loop)
			case "list-ports":
				var err error
				status, err = tui.RunListPorts(status)
				if err != nil {
					return err
				}
			case "metadata":
				var err error
				status, err = tui.RunMetadataEditor(status)
				if err != nil {
					return err
				}
			case "view-runs":
				var err error
				status, err = tui.RunRunsViewer(status)
				if err != nil {
					return err
				}
			case "upload":
				if err := uploadCmd.RunE(cmd, args); err != nil {
					return err
				}
			case "config":
				if err := configCmd.RunE(cmd, args); err != nil {
					return err
				}
			default:
				// Unknown action, return to welcome screen
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
