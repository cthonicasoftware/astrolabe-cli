package root

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "astrolabe",
	Short: "Standardized capture & upload of QA artifacts",
	Long:  "Astrolabe captures, normalizes, caches, and uploads QA test run data.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If no subcommand provided, launch TUI
		return tuiCmd.RunE(cmd, args)
	},
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.astrolabe/connection.yml)")
	rootCmd.PersistentFlags().Bool("json", false, "emit machine-readable JSON output")
	if err := viper.BindPFlag("json", rootCmd.PersistentFlags().Lookup("json")); err != nil {
		panic(fmt.Sprintf("bind json flag: %v", err))
	}

	// attach subcommands
	rootCmd.AddCommand(captureCmd)
	rootCmd.AddCommand(uploadCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(versionCmd)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			viper.AddConfigPath(home + "/.astrolabe")
		}
		viper.SetConfigName("connection")
		viper.SetConfigType("yaml")
	}
	viper.SetEnvPrefix("ASTROLABE")
	viper.AutomaticEnv()
	_ = viper.ReadInConfig()
}

func Execute() error {
	return rootCmd.Execute()
}
