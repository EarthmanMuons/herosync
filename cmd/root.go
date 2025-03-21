package cmd

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/EarthmanMuons/herosync/config"
	"github.com/EarthmanMuons/herosync/internal/fsutil"
	"github.com/EarthmanMuons/herosync/internal/gopro"
	"github.com/EarthmanMuons/herosync/internal/media"
)

// NewRootCmd constructs the root command.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "herosync",
		Short: "Download, combine, and publish GoPro videos with ease",
		CompletionOptions: cobra.CompletionOptions{
			HiddenDefaultCmd: true,
		},
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Only print usage for argument parsing errors.
			cmd.SilenceUsage = true

			logger := initLogger(logLevel(cmd))
			slog.SetDefault(logger)
		},
	}

	cobra.EnableCommandSorting = false
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newDownloadCmd())
	rootCmd.AddCommand(newCombineCmd())
	rootCmd.AddCommand(newPublishCmd())
	rootCmd.AddCommand(newCleanupCmd())
	rootCmd.AddCommand(newYOLOCmd())

	addGlobalFlags(rootCmd)

	// Ensure configuration is initialized before running any command.
	cobra.OnInitialize(func() { initConfig(rootCmd) })

	return rootCmd
}

const (
	configFileUsage = `configuration file path
[env: HEROSYNC_CONFIG_FILE]
[default: %s]
`
	goproHostUsage = `GoPro URL host (IP, hostname:port, "" for mDNS discovery)
[env: HEROSYNC_GOPRO_HOST]
[default: ""]
`
	goproSchemeUsage = `GoPro URL scheme (http, https)
[env: HEROSYNC_GOPRO_SCHEME]
[default: http]
`
	helpUsage = `help for herosync
`
	logLevelUsage = `logging level (debug, info, warn, error)
[env: HEROSYNC_LOG_LEVEL]
[default: info]
`
	mediaDirUsage = `parent directory for media storage
[env: HEROSYNC_MEDIA_DIR]
[default: %s]
`
)

// addGlobalFlags registers global CLI flags.
func addGlobalFlags(rootCmd *cobra.Command) {
	defaultConfig := fsutil.ShortenPath(config.DefaultConfigPath())
	defaultMedia := fsutil.ShortenPath(config.DefaultMediaDir())

	rootCmd.PersistentFlags().StringP("config-file", "c", "", fmt.Sprintf(configFileUsage, defaultConfig))
	rootCmd.PersistentFlags().String("gopro-host", "", goproHostUsage)
	rootCmd.PersistentFlags().String("gopro-scheme", "", goproSchemeUsage)
	rootCmd.PersistentFlags().BoolP("help", "h", false, helpUsage)
	rootCmd.PersistentFlags().StringP("log-level", "l", "", logLevelUsage)
	rootCmd.PersistentFlags().StringP("media-dir", "m", "", fmt.Sprintf(mediaDirUsage, defaultMedia))

	// Define shell completion hints.
	rootCmd.MarkPersistentFlagFilename("config-file", "toml")
	rootCmd.MarkPersistentFlagDirname("media-dir")
}

// logLevel retrieves the log level from flags or config.
func logLevel(cmd *cobra.Command) string {
	lvl, _ := cmd.Flags().GetString("log-level")
	if lvl == "" {
		cfg, err := config.Get()
		if err != nil {
			slog.Default().Warn("failed to load config, using default log level", "error", err)
			return "info"
		}
		return cfg.Log.Level
	}
	return lvl
}

// initConfig initializes the configuration.
func initConfig(cmd *cobra.Command) {
	path := configFile(cmd)
	flags := collectFlagOverrides(cmd)

	// Load the full configuration stack:
	// 1. Defaults
	// 2. Config file
	// 3. Env vars
	// 4. Global CLI flags (subcommand flags are NOT available yet)
	if err := config.Init(path, flags); err != nil {
		log.Fatal(err)
	}
}

// configFile retrieves the configuration file path from flags or config.
func configFile(cmd *cobra.Command) (path string) {
	if path, _ = cmd.Flags().GetString("config-file"); path != "" {
		return path
	}
	if path = os.Getenv("HEROSYNC_CONFIG_FILE"); path != "" {
		return path
	}
	return config.DefaultConfigPath()
}

// collectFlagOverrides extracts flag values for config overrides.
func collectFlagOverrides(cmd *cobra.Command) map[string]any {
	flags := make(map[string]any)
	cmd.Flags().Visit(func(f *pflag.Flag) {
		flags[f.Name] = f.Value.String()
	})
	return flags
}

// initLogger initializes the global logger.
func initLogger(level string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo // fallback to a safe default
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
	return slog.New(handler)
}

// contextLoggerConfig retrieves the runtime context, configuration, and logger.
func contextLoggerConfig(cmd *cobra.Command) (context.Context, *slog.Logger, *config.Config, error) {
	ctx := cmd.Context()
	logger := slog.Default()

	// Apply subcommand-specific flags.
	flags := collectFlagOverrides(cmd)
	if err := config.LoadFlags(flags); err != nil {
		return nil, nil, nil, fmt.Errorf("applying subcommand flags: %w", err)
	}

	// Retrieve the fully merged configuration.
	cfg, err := config.Get()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("loading configuration: %w", err)
	}

	return ctx, logger, cfg, nil
}

func loadFilteredInventory(ctx context.Context, cfg *config.Config, client *gopro.Client, keywords []string) (*media.Inventory, error) {
	inventory, err := media.NewInventory(ctx, client, cfg.IncomingMediaDir(), cfg.OutgoingMediaDir())
	if err != nil {
		return nil, err
	}

	// Apply filtering only if terms are provided.
	if len(keywords) > 0 {
		inventory, err = inventory.FilterByDisplayInfo(keywords)
		if err != nil {
			return nil, err
		}
	}

	return inventory, nil
}
