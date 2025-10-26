package root

import (
	"fmt"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/cliout"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate <run_dir>",
	Short: "Check file/manifest consistency before upload",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create styled printer
		jsonMode, _ := cmd.Flags().GetBool("json")
		out := cliout.DefaultPrinter(jsonMode)

		runDir := args[0]
		out.Step(fmt.Sprintf("Validating run directory: %s", runDir))
		out.Blank()
		out.Warning("Validation not yet implemented")
		out.Muted("TODO: checksum + schema validation")
		return nil
	},
}
