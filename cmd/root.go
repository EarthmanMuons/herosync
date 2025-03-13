package cmd

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/EarthmanMuons/herosync/config"
	"github.com/EarthmanMuons/herosync/internal/fsutil"
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

// addGlobalFlags registers global CLI flags.
func addGlobalFlags(rootCmd *cobra.Command) {
	rootCmd.PersistentFlags().StringP("config-file", "c", "",
		fmt.Sprintf("Configuration file path\n"+
			"[env: HEROSYNC_CONFIG_FILE]\n"+
			"[default: %s]\n", fsutil.ShortenPath(config.DefaultConfigPath())))

	rootCmd.MarkPersistentFlagFilename("config-file", "toml")

	rootCmd.PersistentFlags().String("gopro-host", "",
		"GoPro URL host (IP, hostname:port, \"\" for mDNS discovery)\n"+
			"[env: HEROSYNC_GOPRO_HOST]\n"+
			"[default: \"\"]")

	rootCmd.PersistentFlags().String("gopro-scheme", "",
		"GoPro URL scheme (http, https)\n"+
			"[env: HEROSYNC_GOPRO_SCHEME]\n"+
			"[default: http]")

	rootCmd.PersistentFlags().StringP("log-level", "l", "",
		"Logging level (debug, info, warn, error)\n"+
			"[env: HEROSYNC_LOG_LEVEL]\n"+
			"[default: info]")

	rootCmd.PersistentFlags().StringP("output-dir", "o", "",
		fmt.Sprintf("Output directory path\n"+
			"[env: HEROSYNC_OUTPUT_DIR]\n"+
			"[default: %s%c]\n",
			fsutil.ShortenPath(config.DefaultOutputDir()), filepath.Separator))

	rootCmd.MarkPersistentFlagDirname("output-dir")
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
	cmd.PersistentFlags().Visit(func(f *pflag.Flag) {
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

// parseConfigAndLogger retrieves the runtime configuration and logger.
func parseConfigAndLogger(cmd *cobra.Command) (*slog.Logger, *config.Config, error) {
	logger := slog.Default()

	cfg, err := getConfigWithFlags(cmd)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	return logger, cfg, nil
}

// getConfigWithFlags applies CLI flags to config.
func getConfigWithFlags(cmd *cobra.Command) (*config.Config, error) {
	flags := collectFlagOverrides(cmd)
	if err := config.LoadFlags(flags); err != nil {
		return nil, err
	}
	return config.Get()
}
