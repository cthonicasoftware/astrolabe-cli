package root

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/sources"
	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/tui"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	serialPort string
	serialBaud int
	serialName string
	serialTUI  bool
)

var captureSerialCmd = &cobra.Command{
	Use:   "serial",
	Short: "Capture from a serial port",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if we should run in interactive mode
		// Interactive mode runs when:
		// 1. We have a TTY (not in CI/pipe)
		// 2. Port flag wasn't explicitly set
		portFlagSet := cmd.Flags().Changed("port")
		isInteractive := term.IsTerminal(int(os.Stdin.Fd())) && !portFlagSet

		var (
			config    *sources.Config
			launchTUI bool = serialTUI
		)
		if isInteractive {
			// Run interactive prompt
			var err error
			config, launchTUI, err = tui.RunSerialPrompt()
			if err != nil {
				return fmt.Errorf("interactive prompt failed: %w", err)
			}

			// Use values from interactive prompt
			serialPort = config.Port
			serialBaud = config.Baud
			serialTUI = launchTUI
		} else {
			// Use command-line flags with defaults
			defaults := sources.DefaultConfig()
			defaults.Port = serialPort
			defaults.Baud = serialBaud
			config = &defaults
		}

		fmt.Printf("Starting serial capture: port=%s baud=%d parity=%s data=%d stop=%s flow=%s name=%s tui=%v\n",
			config.Port, config.Baud, config.Parity, config.DataBits, config.StopBits, config.FlowControl, serialName, launchTUI)

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
					// Send the entire frame (including newlines) to the TUI
					// The TUI will handle splitting on newlines and buffering partial lines
					if len(frame) > 0 {
						select {
						case stringCh <- string(frame):
						case <-ctx.Done():
							return
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
		} else {
			fmt.Println("TODO: wire Source→Normalizer→Store and enqueue Upload.")
			time.Sleep(150 * time.Millisecond)
			fmt.Println("Capture complete (stub).")
		}
		return nil
	},
}

func init() {
	captureCmd.AddCommand(captureSerialCmd)
	captureSerialCmd.Flags().StringVarP(&serialPort, "port", "p", "/dev/ttyUSB0", "serial port path")
	captureSerialCmd.Flags().IntVarP(&serialBaud, "baud", "b", 115200, "baud rate")
	captureSerialCmd.Flags().StringVar(&serialName, "name", "", "optional run name")
	captureSerialCmd.Flags().BoolVar(&serialTUI, "tui", false, "launch a live TUI")
}
