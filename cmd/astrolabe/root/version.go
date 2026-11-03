package root

import (
	"encoding/json"
	"fmt"

	"github.com/cthonicasoftware/astrolabe-cli/internal/cliout"
	"github.com/cthonicasoftware/astrolabe-cli/internal/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type versionInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit,omitempty"`
	BuildDate string `json:"build_date,omitempty"`
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Report the CLI version",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		info := versionInfo{
			Version:   version.Version,
			Commit:    version.Commit,
			BuildDate: version.BuildDate,
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
		out.KeyValue("Version", info.Version)
		if info.Commit != "unknown" {
			out.KeyValue("Commit", info.Commit)
		}
		if info.BuildDate != "unknown" {
			out.KeyValue("Built", info.BuildDate)
		}
		out.Blank()
		// TODO: should really come up with a better tagline
		out.Muted("Data acquisition instrument")

		return nil
	},
}
