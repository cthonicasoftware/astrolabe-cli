package root

import (
	"errors"
	"fmt"
	"os"

	"github.com/cthonicasoftware/astrolabe-cli/internal/cliout"
	"github.com/cthonicasoftware/astrolabe-cli/internal/config"
	"github.com/cthonicasoftware/astrolabe-cli/internal/tui"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Args:  cobra.NoArgs,
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit configuration interactively",
	Long: `Launch an interactive TUI to configure Astrolabe settings:
- API URL for your backend
- Project ID
- Authentication token
- Offline cache location
- Upload retry settings`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.RunTUI(tui.RouterConfig{
			InitialScreen: tui.ScreenConfig,
		})
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a config value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create styled printer
		jsonMode, _ := cmd.Flags().GetBool("json")
		out := cliout.DefaultPrinter(jsonMode)

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
				out.KeyValue(key, meta.Operator)
				return nil
			}
		case "location":
			if meta.Location != "" {
				out.KeyValue(key, meta.Location)
				return nil
			}
		}

		// Check if the key exists in metadata attributes
		if val, ok := meta.Attributes[key]; ok {
			out.KeyValue(key, val)
			return nil
		}

		// Fall back to viper config
		val := viper.Get(key)
		if val != nil {
			out.KeyValue(key, fmt.Sprintf("%v", val))
		} else {
			if jsonMode {
				out.Info(fmt.Sprintf("Config key '%s' not set", key))
			} else {
				out.Muted(fmt.Sprintf("Config key '%s' not set", key))
			}
		}
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create styled printer
		jsonMode, _ := cmd.Flags().GetBool("json")
		out := cliout.DefaultPrinter(jsonMode)

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

		out.Success("Configuration saved")
		out.KeyValue(key, value)
		out.Muted(fmt.Sprintf("Saved to: %s", path))
		return nil
	},
}

func init() {
	configCmd.AddCommand(configEditCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
}
