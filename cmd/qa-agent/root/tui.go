package root

import (
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the interactive Text UI",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Main TUI loop - keep showing welcome screen until user quits
		for {
			action, err := tui.RunWelcome()
			if err != nil {
				return err
			}

			// If no action selected (user quit), exit
			if action == "" {
				return nil
			}

			// Handle the selected action
			switch action {
			case "capture":
				// Run the serial capture prompt
				config, err := tui.RunSerialPrompt()
				if err != nil {
					return err
				}
				// TODO: Execute capture with config
				_ = config
				// Return to welcome screen (continue loop)

			case "list-ports":
				// Show TUI list-ports view
				err := tui.RunListPorts()
				if err != nil {
					return err
				}
				// Return to welcome screen (continue loop)

			case "view-runs":
				// TODO: Implement view runs
				// Return to welcome screen (continue loop)

			case "upload":
				// Execute upload command
				err := uploadCmd.RunE(cmd, args)
				if err != nil {
					return err
				}
				// Return to welcome screen (continue loop)

			case "config":
				// Execute config command
				err := configCmd.RunE(cmd, args)
				if err != nil {
					return err
				}
				// Return to welcome screen (continue loop)

			default:
				// Unknown action, return to welcome screen
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
