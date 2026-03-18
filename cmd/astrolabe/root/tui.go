package root

import (
	"fmt"
	"strconv"

	"github.com/cthonicasoftware/astrolabe-cli/internal/cliout"
	"github.com/cthonicasoftware/astrolabe-cli/internal/config"
	"github.com/cthonicasoftware/astrolabe-cli/internal/sources"
	"github.com/cthonicasoftware/astrolabe-cli/internal/tui"
	"github.com/cthonicasoftware/astrolabe-cli/internal/upload"
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
				var err error
				status, err = runTUICaptureAction(out)
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
				// Load configuration and create upload client
				appCfg, err := config.Load()
				if err != nil {
					return err
				}

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

func runTUICaptureAction(out *cliout.Printer) (*tui.StatusMessage, error) {
	captureConfig, err := tui.RunCaptureTabs()
	if err != nil {
		return nil, err
	}
	if captureConfig == nil {
		return nil, nil
	}

	savedMetadata, metaErr := config.LoadMetadata()
	if metaErr != nil {
		out.Warning(fmt.Sprintf("Failed to load metadata: %v", metaErr))
	}
	meta := buildManifestOptions(captureMetadataInput{}, savedMetadata, nil, metaErr == nil)
	appCfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	switch captureConfig.SourceType {
	case "serial":
		if captureConfig.SerialConfig == nil {
			return nil, nil
		}
		serialCfg := captureConfig.SerialConfig
		serial := sources.NewSerialWithConfig(*serialCfg)
		run, saved, err := runInteractiveCapture(
			out,
			serial,
			fmt.Sprintf("Serial Capture - %s @ %d", serialCfg.Port, serialCfg.Baud),
			appCfg.OfflineCache,
			buildSerialManifest(*serialCfg, "", meta),
			buildSerialCaptureSettings(*serialCfg),
		)
		if err != nil {
			return nil, err
		}
		if !saved {
			out.Blank()
			out.Muted("Exited without saving.")
			return tui.NewStatusMessage(tui.StatusInfo, "Run Discarded", "Capture discarded. Start a new run when you are ready."), nil
		}

		out.Blank()
		out.Success("Serial capture saved")
		out.KeyValue("Records", fmt.Sprintf("%d", run.RecordsCount))
		out.KeyValue("Run ID", run.ID)
		out.KeyValue("Cache dir", appCfg.OfflineCache)
		for _, artifact := range run.Artifacts {
			out.Muted(fmt.Sprintf("  - %s (%s)", artifact.Path, artifact.Role))
		}
		return tui.NewStatusMessage(tui.StatusSuccess, "Run Saved", fmt.Sprintf("Run %s saved. Select 'View Runs' to inspect artifacts.", run.ID)), nil

	case "tcp":
		if captureConfig.TCPHost == "" || captureConfig.TCPPort == "" {
			return nil, nil
		}
		tcpPort, err := strconv.Atoi(captureConfig.TCPPort)
		if err != nil {
			return tui.NewStatusMessage(tui.StatusError, "Invalid Port", fmt.Sprintf("TCP port must be a number: %v", err)), nil
		}

		tcpCfg := sources.DefaultTCPConfig()
		tcpCfg.Host = captureConfig.TCPHost
		tcpCfg.Port = tcpPort
		tcpSource, err := sources.NewTCPWithConfig(tcpCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create TCP source: %w", err)
		}

		run, saved, err := runInteractiveCapture(
			out,
			tcpSource,
			fmt.Sprintf("TCP Capture - %s:%d", tcpCfg.Host, tcpCfg.Port),
			appCfg.OfflineCache,
			buildTCPManifest(tcpCfg, "", meta),
			buildTCPCaptureSettings(tcpCfg),
		)
		if err != nil {
			return nil, err
		}
		if !saved {
			out.Blank()
			out.Muted("Exited without saving.")
			return tui.NewStatusMessage(tui.StatusInfo, "Run Discarded", "Capture discarded. Start a new run when you are ready."), nil
		}

		out.Blank()
		out.Success("TCP capture saved")
		out.KeyValue("Records", fmt.Sprintf("%d", run.RecordsCount))
		out.KeyValue("Run ID", run.ID)
		out.KeyValue("Cache dir", appCfg.OfflineCache)
		for _, artifact := range run.Artifacts {
			out.Muted(fmt.Sprintf("  - %s (%s)", artifact.Path, artifact.Role))
		}
		return tui.NewStatusMessage(tui.StatusSuccess, "Run Saved", fmt.Sprintf("Run %s saved. Select 'View Runs' to inspect artifacts.", run.ID)), nil

	case "file":
		return tui.NewStatusMessage(tui.StatusInfo, "Not Implemented", "File source capture is not yet implemented."), nil
	default:
		return nil, nil
	}
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
