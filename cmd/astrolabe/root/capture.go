package root

import "github.com/spf13/cobra"

var captureCmd = &cobra.Command{
	Use:   "capture",
	Short: "Start a capture from a source",
}

func init() {
	// subcommands are defined in separate files (serial, tcp, file)
}
