package root

import (
	"fmt"

	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate <run_dir>",
	Short: "Check file/manifest consistency before upload",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		runDir := args[0]
		fmt.Printf("Validating run at %s ...\n", runDir)
		fmt.Println("TODO: checksum + schema validation.")
		return nil
	},
}
