package root

import (
	"encoding/json"
	"fmt"
	"runtime/debug"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/cliout"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var Version = "0.1.0"
var Schema = "1.0.0"

type versionInfo struct {
	Agent  string `json:"agent"`
	Schema string `json:"schema"`
	Commit string `json:"commit,omitempty"`
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Report agent and schema versions",
	RunE: func(cmd *cobra.Command, args []string) error {
		info := versionInfo{Agent: Version, Schema: Schema}
		if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Sum != "" {
			info.Commit = bi.Main.Sum
		}
		if viper.GetBool("json") {
			b, _ := json.Marshal(info)
			fmt.Println(string(b))
			return nil
		}

		// Create styled printer
		jsonMode, _ := cmd.Flags().GetBool("json")
		out := cliout.DefaultPrinter(jsonMode)

		out.Header("Astrolabe CLI")
		out.KeyValue("Version", info.Agent)
		out.KeyValue("Schema", info.Schema)
		if info.Commit != "" {
			out.KeyValue("Commit", info.Commit)
		}
		out.Blank()
		out.Muted("Data acquisition agent for test benches and QA systems")

		return nil
	},
}
