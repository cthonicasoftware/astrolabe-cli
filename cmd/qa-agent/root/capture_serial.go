package root

import (
	"context"
	"fmt"
	"os"
	"time"

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

		if isInteractive {
			// Run interactive prompt
			config, err := tui.RunSerialPrompt()
			if err != nil {
				return fmt.Errorf("interactive prompt failed: %w", err)
			}

			// Use values from interactive prompt
			serialPort = config.Port
			serialBaud = config.Baud
			serialTUI = config.TUI
		}

		fmt.Printf("Starting serial capture: port=%s baud=%d name=%s tui=%v\n", serialPort, serialBaud, serialName, serialTUI)
		if serialTUI {
			// For now demo with a synthetic stream; replace with real source frames.
			ch := make(chan string, 16)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go func() {
				ticker := time.NewTicker(200 * time.Millisecond)
				defer ticker.Stop()
				i := 0
				for {
					select {
					case <-ctx.Done():
						close(ch)
						return
					case t := <-ticker.C:
						i++
						ch <- fmt.Sprintf("[serial:%s@%d] line %d @ %s", serialPort, serialBaud, i, t.UTC().Format(time.RFC3339Nano))
						if i >= 100 {
							cancel()
						}
					}
				}
			}()
			m := tui.NewApp("qa-agent capture", ch)
			p := tea.NewProgram(m, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				return err
			}
		}
		fmt.Println("TODO: wire Source→Normalizer→Store and enqueue Upload.")
		time.Sleep(150 * time.Millisecond)
		fmt.Println("Capture complete (stub).")
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
