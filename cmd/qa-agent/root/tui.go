package root

import (
    "context"
    "fmt"
    "time"

    tea "github.com/charmbracelet/bubbletea"
    "github.com/LostinTimeandspaceYT/qa_cli_agent/internal/tui"
    "github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
    Use:   "tui",
    Short: "Launch the interactive Text UI",
    RunE: func(cmd *cobra.Command, args []string) error {
        // Demo channel that emits sample lines; you can wire real capture later.
        ch := make(chan string, 16)
        ctx, cancel := context.WithCancel(context.Background())
        defer cancel()

        go func() {
            ticker := time.NewTicker(300 * time.Millisecond)
            defer ticker.Stop()
            i := 0
            for {
                select {
                case <-ctx.Done():
                    close(ch)
                    return
                case t := <-ticker.C:
                    i++
                    ch <- fmt.Sprintf("sample %03d at %s", i, t.UTC().Format(time.RFC3339Nano))
                    if i >= 50 {
                        cancel()
                    }
                }
            }
        }()

        m := tui.NewApp("qa-agent", ch)
        p := tea.NewProgram(m, tea.WithAltScreen())
        if _, err := p.Run(); err != nil {
            return err
        }
        return nil
    },
}

func init() {
    rootCmd.AddCommand(tuiCmd)
}
