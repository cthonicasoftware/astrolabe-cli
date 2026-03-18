package root

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cthonicasoftware/astrolabe-cli/internal/tui"
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
					// TODO: wire in next slice
					return tui.NewWelcome(tui.NewStatusMessage(tui.StatusInfo, "Not yet wired", "Metadata coming soon")), nil, nil
				},

				tui.ScreenRuns: func(ctx tui.ScreenContext) (tea.Model, func(), error) {
					// TODO: wire in next slice
					return tui.NewWelcome(tui.NewStatusMessage(tui.StatusInfo, "Not yet wired", "View Runs coming soon")), nil, nil
				},

				tui.ScreenUpload: func(ctx tui.ScreenContext) (tea.Model, func(), error) {
					// TODO: wire in next slice
					return tui.NewWelcome(tui.NewStatusMessage(tui.StatusInfo, "Not yet wired", "Upload coming soon")), nil, nil
				},
			},
		})
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
