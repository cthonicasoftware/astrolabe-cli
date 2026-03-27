package root

import (
	"context"
	"fmt"

	"github.com/cthonicasoftware/astrolabe-cli/internal/config"
	"github.com/spf13/cobra"
)

type configContextKey struct{}

func withConfig(ctx context.Context, cfg config.Config) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, configContextKey{}, cfg)
}

func configFromCmd(cmd *cobra.Command) (config.Config, error) {
	if cmd == nil {
		return config.Config{}, fmt.Errorf("config not found in context: command is nil")
	}

	cfg, ok := cmd.Context().Value(configContextKey{}).(config.Config)
	if !ok {
		return config.Config{}, fmt.Errorf("config not found in context: PersistentPreRunE may not have run")
	}
	return cfg, nil
}
