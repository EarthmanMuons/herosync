package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/EarthmanMuons/herosync/config"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status and statistics",
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().String("camera-ip", "", "override camera IP address")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Apply any local flags that were actually set.
	flags := make(map[string]any)
	cmd.Flags().Visit(func(f *pflag.Flag) {
		flags[f.Name] = f.Value.String()
	})

	cfg, err := config.ApplyFlags(flags)
	if err != nil {
		return err
	}

	log.Printf("[DEBUG] statusCmd collected flags: %+v", flags)

	fmt.Printf("GoPro base URL: %s://%s\n", cfg.Camera.Protocol, cfg.Camera.IP)
	return nil
}
