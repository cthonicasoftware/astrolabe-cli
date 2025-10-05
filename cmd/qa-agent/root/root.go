package root

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
    Use:   "qa-agent",
    Short: "Standardized capture & upload for QA artifacts",
    Long:  "qa-agent is a stylish, operator-friendly CLI to capture, normalize, cache, and upload QA run data.",
}

func init() {
    cobra.OnInitialize(initConfig)

    rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.qa-agent.yml)")
    rootCmd.PersistentFlags().BoolP("json", "", false, "emit machine-readable JSON output")
    viper.BindPFlag("json", rootCmd.PersistentFlags().Lookup("json"))
}

func initConfig() {
    if cfgFile != "" {
        viper.SetConfigFile(cfgFile)
    } else {
        home, err := os.UserHomeDir()
        if err == nil {
            viper.AddConfigPath(home)
        }
        viper.SetConfigName(".qa-agent")
        viper.SetConfigType("yaml")
    }
    viper.SetEnvPrefix("QA_AGENT")
    viper.AutomaticEnv()
    _ = viper.ReadInConfig()
}

func Execute() {
    // attach subcommands
    rootCmd.AddCommand(captureCmd)
    rootCmd.AddCommand(uploadCmd)
    rootCmd.AddCommand(validateCmd)
    rootCmd.AddCommand(configCmd)
    rootCmd.AddCommand(versionCmd)

    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
