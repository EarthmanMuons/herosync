package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "herosync",
	Short: "Download, combine, and publish GoPro videos with ease",
	Long: `A tool for automating GoPro video transfers. Download media files over WiFi,
combine chapters into complete videos, clean up storage, and optionally publish
to YouTube.`,
	// The bare root command runs status by default
	Run: runStatus,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.herosync.yaml)")
}
