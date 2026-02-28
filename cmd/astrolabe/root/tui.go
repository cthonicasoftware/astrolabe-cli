package root

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/capture"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/cliout"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/config"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/core"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/normalize"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/sources"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/storage"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/tui"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/upload"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the interactive Text UI",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonMode, _ := cmd.Flags().GetBool("json")
		out := cliout.DefaultPrinter(jsonMode)

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
				// Show unified capture configuration screen
				captureConfig, err := tui.RunCaptureTabs()
				if err != nil {
					return err
				}
				if captureConfig == nil {
					// User canceled - return to welcome menu
					continue
				}

				// Handle based on source type
				switch captureConfig.SourceType {
				case "serial":
					if captureConfig.SerialConfig == nil {
						continue
					}
					serialCfg := captureConfig.SerialConfig
						savedMetadata, metaErr := config.LoadMetadata()
						if metaErr != nil {
							out.Warning(fmt.Sprintf("Failed to load metadata: %v", metaErr))
						}
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
						cancel()
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
							out.Blank()
							out.Step("Saving capture data...")
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
							out.Success("Serial capture saved")
							out.KeyValue("Records", fmt.Sprintf("%d", run.RecordsCount))
							out.KeyValue("Run ID", run.ID)
							out.KeyValue("Cache dir", appCfg.OfflineCache)
							for _, artifact := range run.Artifacts {
								out.Muted(fmt.Sprintf("  - %s (%s)", artifact.Path, artifact.Role))
							}
							status = tui.NewStatusMessage(tui.StatusSuccess, "Run Saved", fmt.Sprintf("Run %s saved. Select 'View Runs' to inspect artifacts.", run.ID))
						} else {
							out.Blank()
							out.Muted("Exited without saving.")
							run := <-pipelineResultCh
							runErr := <-pipelineErrCh
							if runErr != nil && !errors.Is(runErr, context.Canceled) {
								out.Error(fmt.Sprintf("Capture pipeline error: %v", runErr))
							}
						if run != nil {
							runDir := filepath.Join(tempRoot, run.ID)
							_ = os.RemoveAll(runDir)
						}
						status = tui.NewStatusMessage(tui.StatusInfo, "Run Discarded", "Capture discarded. Start a new run when you are ready.")
					}
						if err := serial.Close(); err != nil {
							out.Warning(fmt.Sprintf("Failed to close serial port: %v", err))
						}
				case "tcp":
					if captureConfig.TCPHost == "" || captureConfig.TCPPort == "" {
						continue
					}
					// Parse port string to int
					tcpPort, err := strconv.Atoi(captureConfig.TCPPort)
					if err != nil {
						status = tui.NewStatusMessage(tui.StatusError, "Invalid Port", fmt.Sprintf("TCP port must be a number: %v", err))
						continue
					}

					tcpCfg := &sources.TCPConfig{
						Host: captureConfig.TCPHost,
						Port: tcpPort,
					}

						savedMetadata, metaErr := config.LoadMetadata()
						if metaErr != nil {
							out.Warning(fmt.Sprintf("Failed to load metadata: %v", metaErr))
						}
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

					tcpSource, err := sources.NewTCPWithConfig(*tcpCfg)
					if err != nil {
						return fmt.Errorf("failed to create TCP source: %w", err)
					}

					// Build manifest with defaults
					meta := serialManifestOptions{} // Reuse serial manifest options
					if metaErr == nil {
						applyMetadataDefaults(&meta, savedMetadata, nil)
					}
					if meta.Operator == "" {
						meta.Operator = os.Getenv("USER")
					}
					if meta.Test.Plan == "" {
						meta.Test.Plan = "unspecified"
					}

					// Create manifest and capture settings
					manifest := buildTCPManifestFromOptions(tcpCfg.Host, tcpCfg.Port, meta)
					captureSettings := core.CaptureSettings{
						Channels: []string{"tcp"},
						Notes:    fmt.Sprintf("TCP capture from %s:%d", tcpCfg.Host, tcpCfg.Port),
					}

					pipelineOpts := capture.Options{
						Source:     tcpSource,
						Normalizer: normalizer,
						Store:      store,
						Manifest:   manifest,
						Capture:    captureSettings,
					}
					pipeline, err := capture.NewPipeline(pipelineOpts)
					if err != nil {
						return fmt.Errorf("build pipeline: %w", err)
					}

					ctx, cancel := context.WithCancel(context.Background())

					// Setup data channels
					stringCh := make(chan string, 16)
					pipelineFramesCh := make(chan []byte, 16)
					pipelineSource := &frameChannelSource{
						framesCh: pipelineFramesCh,
						meta:     tcpSource.Meta(),
					}
					pipelineOpts.Source = pipelineSource
					pipeline, err = capture.NewPipeline(pipelineOpts)
					if err != nil {
						cancel()
						return fmt.Errorf("rebuild pipeline with wrapper source: %w", err)
					}

					// Launch TUI first
					title := fmt.Sprintf("TCP Capture - %s:%d", tcpCfg.Host, tcpCfg.Port)
					m := tui.NewApp(title, stringCh)
					p := tea.NewProgram(m, tea.WithAltScreen())

					// Attempt connection in background and tee the data
					connectionErrCh := make(chan error, 1)
					go func() {
						// Send connecting message
						stringCh <- fmt.Sprintf("Connecting to %s:%d...\n", tcpCfg.Host, tcpCfg.Port)

						// Try to open connection
						if err := tcpSource.Open(ctx); err != nil {
							stringCh <- fmt.Sprintf("ERROR: Failed to connect: %v\n", err)
							connectionErrCh <- err
							close(stringCh)
							close(pipelineFramesCh)
							return
						}

						stringCh <- fmt.Sprintf("Connected to %s:%d\n", tcpCfg.Host, tcpCfg.Port)
						connectionErrCh <- nil

						// Tee the data
						defer close(stringCh)
						defer close(pipelineFramesCh)
						for frame := range tcpSource.Frames() {
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

					// Run pipeline in background
					pipelineResultCh := make(chan *core.Run, 1)
					pipelineErrCh := make(chan error, 1)
					go func() {
						run, err := pipeline.Run(ctx)
						pipelineResultCh <- run
						pipelineErrCh <- err
					}()

					// Run TUI
					finalModel, err := p.Run()
					if err != nil {
						cancel()
						return err
					}

					tuiApp, ok := finalModel.(*tui.App)
					if !ok {
						cancel()
						tcpSource.Close()
						return fmt.Errorf("unexpected model type: %T", finalModel)
					}

						cancel()
						if tuiApp.SaveRequested() {
							out.Blank()
							out.Step("Saving capture data...")
							run := <-pipelineResultCh
							runErr := <-pipelineErrCh
							if runErr != nil && !errors.Is(runErr, context.Canceled) {
								tcpSource.Close()
								return fmt.Errorf("capture pipeline: %w", runErr)
						}
						if run == nil {
							tcpSource.Close()
							return fmt.Errorf("capture pipeline: run not returned")
						}
							if err := promoteRunArtifacts(run, tempRoot, appCfg.OfflineCache); err != nil {
								tcpSource.Close()
								return fmt.Errorf("finalize run artifacts: %w", err)
							}
							out.Success("TCP capture saved")
							out.KeyValue("Records", fmt.Sprintf("%d", run.RecordsCount))
							out.KeyValue("Run ID", run.ID)
							out.KeyValue("Cache dir", appCfg.OfflineCache)
							for _, artifact := range run.Artifacts {
								out.Muted(fmt.Sprintf("  - %s (%s)", artifact.Path, artifact.Role))
							}
							status = tui.NewStatusMessage(tui.StatusSuccess, "Run Saved", fmt.Sprintf("Run %s saved. Select 'View Runs' to inspect artifacts.", run.ID))
						} else {
							out.Blank()
							out.Muted("Exited without saving.")
							run := <-pipelineResultCh
							runErr := <-pipelineErrCh
							if runErr != nil && !errors.Is(runErr, context.Canceled) {
								out.Error(fmt.Sprintf("Capture pipeline error: %v", runErr))
							}
						if run != nil {
							runDir := filepath.Join(tempRoot, run.ID)
							_ = os.RemoveAll(runDir)
						}
						status = tui.NewStatusMessage(tui.StatusInfo, "Run Discarded", "Capture discarded. Start a new run when you are ready.")
					}
						if err := tcpSource.Close(); err != nil {
							out.Warning(fmt.Sprintf("Failed to close TCP connection: %v", err))
						}
				case "file":
					// TODO: Implement file source handling
					status = tui.NewStatusMessage(tui.StatusInfo, "Not Implemented", "File source capture is not yet implemented.")
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
				// Load configuration and create upload client
				appCfg := config.Load()

				// Validate configuration
				if appCfg.APIURL == "" || appCfg.AuthToken == "" || appCfg.ProjectID == "" {
					status = tui.NewStatusMessage(
						tui.StatusError,
						"Upload Configuration Missing",
						"Configure connection settings before uploading. Select 'Configure Connection'.",
					)
					continue
				}

				maxRetries := appCfg.Upload.MaxRetries
				if maxRetries == 0 {
					maxRetries = 3
				}

				// Create upload client
				client := upload.NewClient(upload.Config{
					APIURL:     appCfg.APIURL,
					AuthToken:  appCfg.AuthToken,
					ProjectID:  appCfg.ProjectID,
					CacheRoot:  appCfg.OfflineCache,
					MaxRetries: maxRetries,
				})

				// Find pending runs
				runs, err := findPendingRuns(appCfg.OfflineCache)
				if err != nil {
					return fmt.Errorf("find pending runs: %w", err)
				}

				// Run upload with status
				status, err = tui.RunUploadWithStatus(client, runs, status)
				if err != nil {
					return err
				}
			case "config-connection":
				var err error
				status, err = tui.RunConfigEditor(status)
				if err != nil {
					return err
				}
			default:
				// Unknown action, return to welcome screen
			}
		}
	},
}

func buildTCPManifestFromOptions(host string, port int, opts serialManifestOptions) core.Manifest {
	attrs := map[string]string{
		"source_kind": "tcp",
		"tcp_host":    host,
		"tcp_port":    fmt.Sprintf("%d", port),
		"tcp_addr":    fmt.Sprintf("%s:%d", host, port),
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

func init() {
	rootCmd.AddCommand(tuiCmd)
}
