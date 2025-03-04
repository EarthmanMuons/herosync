package cmd

import (
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/EarthmanMuons/herosync/config"
)

var cfgFile string

// The bare root command runs status by default.
var rootCmd = &cobra.Command{
	Use:   "herosync",
	Short: "Download, combine, and publish GoPro videos with ease",
	Long: `A tool for automating GoPro video transfers. Download media files over WiFi,
combine chapters into complete videos, clean up storage, and optionally publish
to YouTube.`,
	RunE: runStatus,
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
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path")
}

func initConfig() {
	if err := config.Init(cfgFile); err != nil {
		log.Fatal(err)
	}
}
