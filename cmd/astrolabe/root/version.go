package root

import (
	"encoding/json"
	"fmt"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/cliout"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Version information - set via ldflags at build time
// Example: go build -ldflags "-X github.com/LostinTimeandspaceYT/qa_cli_agent/cmd/astrolabe/root.Version=1.0.0"
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
	Schema    = "1.0.0"
)

type versionInfo struct {
	Agent     string `json:"agent"`
	Schema    string `json:"schema"`
	Commit    string `json:"commit,omitempty"`
	BuildDate string `json:"build_date,omitempty"`
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Report agent and schema versions",
	RunE: func(cmd *cobra.Command, args []string) error {
		info := versionInfo{
			Agent:     Version,
			Schema:    Schema,
			Commit:    Commit,
			BuildDate: BuildDate,
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
		if info.Commit != "unknown" {
			out.KeyValue("Commit", info.Commit)
		}
		if info.BuildDate != "unknown" {
			out.KeyValue("Built", info.BuildDate)
		}
		out.Blank()
		out.Muted("Data acquisition agent for test benches and QA systems")

		return nil
	},
}
