package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/EarthmanMuons/herosync/config"
)

var configFile string

var rootCmd = &cobra.Command{
	Use:   "herosync",
	Short: "Download, combine, and publish GoPro videos with ease",
	Long: `A tool for automating GoPro video transfers. Download media files over WiFi,
combine chapters into complete videos, clean up storage, and optionally publish
to YouTube.`,
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
	rootCmd.PersistentFlags().StringVar(&configFile, "config-file", "",
		fmt.Sprintf("configuration file path\n"+
			"[env: HEROSYNC_CONFIG_FILE]\n"+
			"[default: %s]\n",
			shortenPath(config.DefaultConfigPath())))
}

func initConfig() {
	flags := make(map[string]any)
	rootCmd.PersistentFlags().Visit(func(f *pflag.Flag) {
		flags[f.Name] = f.Value.String()
	})

	// Only use environment variable or default path if --config-file flag wasn't explicitly set
	if configFile == "" {
		if envConfig := os.Getenv("HEROSYNC_CONFIG_FILE"); envConfig != "" {
			configFile = envConfig
		} else {
			configFile = config.DefaultConfigPath()
		}
	}

	if err := config.Init(configFile, flags); err != nil {
		log.Fatal(err)
	}
}
