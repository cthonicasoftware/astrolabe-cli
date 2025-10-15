package root

import (
	"encoding/json"
	"fmt"
	"runtime/debug"

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
		fmt.Printf("astrolabe %s (schema %s)\n", info.Agent, info.Schema)
		return nil
	},
}
