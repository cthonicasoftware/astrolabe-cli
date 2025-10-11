package root

import (
	"context"
	"fmt"
	"strings"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/sources"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
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
				config, launchTUI, err := tui.RunSerialPrompt()
				if err != nil {
					return err
				}

				// If user selected TUI mode, launch the live capture
				if launchTUI {
					// Create serial source with advanced settings
					serial := sources.NewSerialWithConfig(*config)

					// Open the serial port
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()

					if err := serial.Open(ctx); err != nil {
						return fmt.Errorf("failed to open serial port: %w", err)
					}
					defer serial.Close()

					// Create a string channel for the TUI
					stringCh := make(chan string, 16)

					// Convert byte frames to strings
					go func() {
						defer close(stringCh)
						for frame := range serial.Frames() {
							// Split on newlines and send each line
							lines := strings.Split(string(frame), "\n")
							for _, line := range lines {
								if line != "" {
									select {
									case stringCh <- line:
									case <-ctx.Done():
										return
									}
								}
							}
						}
					}()

					// Launch the TUI
					title := fmt.Sprintf("Serial Capture - %s @ %d", config.Port, config.Baud)
					m := tui.NewApp(title, stringCh)
					p := tea.NewProgram(m, tea.WithAltScreen())
					if _, err := p.Run(); err != nil {
						return err
					}

					// Cancel context to stop serial reading
					cancel()
				}
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
