package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/EarthmanMuons/herosync/config"
	"github.com/EarthmanMuons/herosync/internal/logging"
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
		// Bootstrap the logger as early as possible.
		if rootOpts.logLevel == "" {
			cfg, err := config.Get()
			if err != nil {
				log.Fatalf("Failed to get config: %v", err)
			}
			rootOpts.logLevel = cfg.Log.Level
		}

		logging.Initialize(rootOpts.logLevel)
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.EnableCommandSorting = false
	cobra.OnInitialize(initConfig)

	// Global Flags

	rootCmd.PersistentFlags().StringVar(&rootOpts.configFile, "config-file", "",
		fmt.Sprintf("configuration file path\n"+
			"[env: HEROSYNC_CONFIG_FILE]\n"+
			"[default: %s]\n",
			shortenPath(config.DefaultConfigPath())))

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
			shortenPath(config.DefaultOutputDir()),
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
