package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/EarthmanMuons/herosync/config"
	"github.com/EarthmanMuons/herosync/internal/logging"
)

type rootOptions struct {
	configFile string
	logLevel   string
}

var opts rootOptions

var rootCmd = &cobra.Command{
	Use:   "herosync",
	Short: "Download, combine, and publish GoPro videos with ease",
	Long: `A tool for automating GoPro video transfers. Download media files over WiFi,
combine chapters into complete videos, clean up storage, and optionally publish
to YouTube.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if opts.logLevel == "" {
			cfg, err := config.Get()
			if err != nil {
				log.Fatalf("Failed to get config: %v", err)
			}
			opts.logLevel = cfg.Log.Level
		}

		logging.Initialize(opts.logLevel)
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&opts.configFile, "config-file", "",
		fmt.Sprintf("configuration file path\n"+
			"[env: HEROSYNC_CONFIG_FILE]\n"+
			"[default: %s]\n",
			shortenPath(config.DefaultConfigPath())))

	rootCmd.PersistentFlags().StringVar(&opts.logLevel, "log-level", "",
		"logging level (debug, info, warn, error)\n"+
			"[env: HEROSYNC_LOG_LEVEL]\n"+
			"[default: info]\n")
}

func initConfig() {
	flags := make(map[string]any)
	rootCmd.PersistentFlags().Visit(func(f *pflag.Flag) {
		flags[f.Name] = f.Value.String()
	})

	// Only use environment variable or default path if --config-file flag wasn't explicitly set
	if opts.configFile == "" {
		if envConfig := os.Getenv("HEROSYNC_CONFIG_FILE"); envConfig != "" {
			opts.configFile = envConfig
		} else {
			opts.configFile = config.DefaultConfigPath()
		}
	}

	if err := config.Init(opts.configFile, flags); err != nil {
		log.Fatal(err)
	}
}
