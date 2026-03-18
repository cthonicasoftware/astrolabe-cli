package root

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cthonicasoftware/astrolabe-cli/internal/config"
	"github.com/cthonicasoftware/astrolabe-cli/internal/tui"
	"github.com/cthonicasoftware/astrolabe-cli/internal/upload"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the interactive Text UI",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.RunTUI(tui.RouterConfig{
			InitialScreen: tui.ScreenWelcome,
			Factories: map[tui.ScreenID]tui.ScreenFactory{
				tui.ScreenWelcome: func(ctx tui.ScreenContext) (tea.Model, func(), error) {
					return tui.NewWelcome(ctx.Status), nil, nil
				},

				tui.ScreenConfig: func(ctx tui.ScreenContext) (tea.Model, func(), error) {
					return tui.NewConfigEditor(), nil, nil
				},

				tui.ScreenCaptureTabs: func(ctx tui.ScreenContext) (tea.Model, func(), error) {
					// TODO: wire in next slice
					return tui.NewWelcome(tui.NewStatusMessage(tui.StatusInfo, "Not yet wired", "Capture coming soon")), nil, nil
				},

				tui.ScreenCaptureLive: func(ctx tui.ScreenContext) (tea.Model, func(), error) {
					// TODO: wire in next slice
					return tui.NewWelcome(tui.NewStatusMessage(tui.StatusInfo, "Not yet wired", "Capture live coming soon")), nil, nil
				},

				tui.ScreenMetadata: func(ctx tui.ScreenContext) (tea.Model, func(), error) {
					repo := config.NewFileMetadataRepository(nil)
					meta, loadErr := repo.Load()
					var path string
					if p, perr := repo.Path(); perr == nil {
						path = p
					}
					return tui.NewMetadataEditor(repo, meta, path, loadErr), nil, nil
				},

				tui.ScreenRuns: func(ctx tui.ScreenContext) (tea.Model, func(), error) {
					cfg, err := config.Load()
					if err != nil {
						return nil, nil, err
					}
					return tui.NewRunsViewer(cfg.OfflineCache), nil, nil
				},

				tui.ScreenUpload: func(ctx tui.ScreenContext) (tea.Model, func(), error) {
					// Load and validate configuration
					appCfg, err := config.Load()
					if err != nil {
						return nil, nil, fmt.Errorf("load config: %w", err)
					}

					if appCfg.APIURL == "" || appCfg.AuthToken == "" || appCfg.ProjectID == "" {
						return nil, nil, fmt.Errorf("upload configuration incomplete: configure connection settings before uploading")
					}

					maxRetries := appCfg.Upload.MaxRetries
					if maxRetries == 0 {
						maxRetries = 3
					}

					// Build upload client
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
						return nil, nil, fmt.Errorf("find pending runs: %w", err)
					}

					if len(runs) == 0 {
						return nil, nil, fmt.Errorf("no pending runs: all runs have been uploaded")
					}

					return tui.NewUploadModel(client, runs), nil, nil
				},
			},
		})
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
