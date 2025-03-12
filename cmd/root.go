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

type rootOptions struct {
	configFile string
	logLevel   string
}

var rootOpts rootOptions

var rootCmd = &cobra.Command{
	Use:   "herosync",
	Short: "Download, combine, and publish GoPro videos with ease",
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true,
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logLevel := rootOpts.logLevel

		if logLevel == "" {
			cfg, err := config.Get()
			if err != nil {
				slog.Default().Warn("failed to load config, using default log level", "error", err)
				logLevel = "info"
			} else {
				logLevel = cfg.Log.Level
			}
		}

		// Initialize the global logger.
		logger := initLogger(logLevel)
		slog.SetDefault(logger)
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Commands are added here in a specific order to control their appearance in
	// the help output.
	cobra.EnableCommandSorting = false
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(downloadCmd)
	rootCmd.AddCommand(combineCmd)
	rootCmd.AddCommand(publishCmd)
	rootCmd.AddCommand(cleanupCmd)
	rootCmd.AddCommand(yoloCmd)

	cobra.OnInitialize(initConfig)

	// Global Flags

	rootCmd.PersistentFlags().StringVar(&rootOpts.configFile, "config-file", "",
		fmt.Sprintf("configuration file path\n"+
			"[env: HEROSYNC_CONFIG_FILE]\n"+
			"[default: %s]\n",
			fsutil.ShortenPath(config.DefaultConfigPath())))

	rootCmd.MarkPersistentFlagFilename("config-file", "toml")

	rootCmd.PersistentFlags().String("gopro-host", "",
		fmt.Sprint("GoPro URL host (IP, hostname:port, \"\" for mDNS discovery)\n"+
			"[env: HEROSYNC_GOPRO_HOST]\n"+
			"[default: \"\"]\n"))

	rootCmd.PersistentFlags().String("gopro-scheme", "",
		fmt.Sprint("GoPro URL scheme (http, https)\n"+
			"[env: HEROSYNC_GOPRO_SCHEME]\n"+
			"[default: http]\n"))

	rootCmd.PersistentFlags().StringVarP(&rootOpts.logLevel, "log-level", "l", "",
		fmt.Sprint("logging level (debug, info, warn, error)\n"+
			"[env: HEROSYNC_LOG_LEVEL]\n"+
			"[default: info]\n"))

	rootCmd.PersistentFlags().StringP("output-dir", "o", "",
		fmt.Sprintf("output directory path\n"+
			"[env: HEROSYNC_OUTPUT_DIR]\n"+
			"[default: %s%c]\n",
			fsutil.ShortenPath(config.DefaultOutputDir()),
			filepath.Separator))

	rootCmd.MarkPersistentFlagDirname("output-dir")
}

func initConfig() {
	flags := make(map[string]any)
	rootCmd.PersistentFlags().Visit(func(f *pflag.Flag) {
		flags[f.Name] = f.Value.String()
	})

	// Only use environment variable or default path if --config-file flag wasn't explicitly set
	if rootOpts.configFile == "" {
		if envConfig := os.Getenv("HEROSYNC_CONFIG_FILE"); envConfig != "" {
			rootOpts.configFile = envConfig
		} else {
			rootOpts.configFile = config.DefaultConfigPath()
		}
	}

	if err := config.Init(rootOpts.configFile, flags); err != nil {
		log.Fatal(err)
	}
}

func initLogger(level string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo // fallback to a safe default
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(handler)
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
