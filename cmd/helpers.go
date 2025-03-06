package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/EarthmanMuons/herosync/config"
)

func ensureDir(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	err = os.MkdirAll(absPath, 0o750)
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %w", absPath, err)
	}

	return nil
}

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

// shortenPath replaces the home directory path with ~
func shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return strings.Replace(path, home, "~", 1)
}
