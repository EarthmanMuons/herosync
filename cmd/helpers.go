package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/EarthmanMuons/herosync/config"
)

// applyLocalFlags collects any set flags from the command and applies them to the configuration.
func applyLocalFlags(cmd *cobra.Command) (*config.Config, error) {
	flags := make(map[string]any)
	cmd.Flags().Visit(func(f *pflag.Flag) {
		flags[f.Name] = f.Value.String()
	})
	return config.ApplyFlags(flags)
}
