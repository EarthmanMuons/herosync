package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/EarthmanMuons/herosync/config"
)

// getConfigWithFlags collects any set flags from the command and applies them to the configuration.
func getConfigWithFlags(cmd *cobra.Command) (*config.Config, error) {
	flags := make(map[string]any)
	cmd.Flags().Visit(func(f *pflag.Flag) {
		flags[f.Name] = f.Value.String()
	})

	if err := config.LoadFlags(flags); err != nil {
		return nil, err
	}
	return config.Get()
}
