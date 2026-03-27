package root

import (
	"fmt"

	"github.com/cthonicasoftware/astrolabe-cli/internal/cliout"
	"github.com/cthonicasoftware/astrolabe-cli/internal/config"
	"github.com/cthonicasoftware/astrolabe-cli/internal/tui"
	"github.com/cthonicasoftware/astrolabe-cli/internal/upload"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the interactive Text UI",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		appCfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		out := cliout.DefaultPrinter(viper.GetBool("json"))
		return runTUI(out, appCfg, tui.ScreenWelcome)
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}

// runCaptureTUI launches the screen router starting at the capture configuration screen.
func runCaptureTUI(out *cliout.Printer) error {
	appCfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	return runTUI(out, appCfg, tui.ScreenCaptureTabs)
}

// runTUI launches the screen router starting at initialScreen.
func runTUI(out *cliout.Printer, appCfg config.Config, initialScreen tui.ScreenID) error {
	capturePort := newCaptureAdapter(out, appCfg.OfflineCache)
	metadataPort := newMetadataAdapter(config.NewFileMetadataRepository(nil))

	var uploadPort tui.UploadPort
	if appCfg.APIURL != "" && appCfg.AuthToken != "" && appCfg.ProjectID != "" {
		maxRetries := appCfg.Upload.MaxRetries
		if maxRetries == 0 {
			maxRetries = 3
		}

		uploadPort = upload.NewClient(upload.Config{
			APIURL:     appCfg.APIURL,
			AuthToken:  appCfg.AuthToken,
			ProjectID:  appCfg.ProjectID,
			CacheRoot:  appCfg.OfflineCache,
			MaxRetries: maxRetries,
		})
	}

	return tui.RunTUI(tui.RouterConfig{
		InitialScreen: initialScreen,
		CapturePort:   capturePort,
		MetadataPort:  metadataPort,
		UploadPort:    uploadPort,
		RunsCacheRoot: appCfg.OfflineCache,
	})
}
