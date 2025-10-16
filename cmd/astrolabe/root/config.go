package root

import (
	"errors"
	"fmt"
	"os"

	"github.com/LostinTimeandspaceYT/qa_cli_agent/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a config value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]

		// Try to load from metadata.json first
		meta, err := config.LoadMetadata()
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("load metadata: %w", err)
		}

		// Check top-level metadata fields first
		switch key {
		case "operator":
			if meta.Operator != "" {
				fmt.Println(meta.Operator)
				return nil
			}
		case "location":
			if meta.Location != "" {
				fmt.Println(meta.Location)
				return nil
			}
		}

		// Check if the key exists in metadata attributes
		if val, ok := meta.Attributes[key]; ok {
			fmt.Println(val)
			return nil
		}

		// Fall back to viper config
		fmt.Println(viper.Get(key))
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]

		// Load existing metadata or create new one
		meta, err := config.LoadMetadata()
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("load metadata: %w", err)
		}

		// Handle top-level metadata fields
		switch key {
		case "operator":
			meta.Operator = value
		case "location":
			meta.Location = value
		default:
			// Set the value in metadata attributes for all other keys
			if meta.Attributes == nil {
				meta.Attributes = make(map[string]string)
			}
			meta.Attributes[key] = value
		}

		// Save to metadata.json
		path, err := config.SaveMetadata(meta)
		if err != nil {
			return fmt.Errorf("save metadata: %w", err)
		}

		// Also set in viper for in-memory access during the current session
		viper.Set(key, value)

		fmt.Printf("ok (persisted to %s)\n", path)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
}
