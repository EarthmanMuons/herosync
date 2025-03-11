package cmd

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/EarthmanMuons/herosync/internal/gopro"
	"github.com/EarthmanMuons/herosync/internal/logging"
	"github.com/EarthmanMuons/herosync/internal/media"
)

var cleanupCmd = &cobra.Command{
	Use:     "cleanup",
	Aliases: []string{"clean"},
	Short:   "Delete transferred media from GoPro storage",
	RunE:    runCleanup,
}

func init() {
	cleanupCmd.Flags().String("gopro-host", "", "GoPro host (hostname:port or IP)")
	cleanupCmd.Flags().String("gopro-scheme", "", "GoPro scheme (http/https)")
}

func runCleanup(cmd *cobra.Command, args []string) error {
	log := logging.GetLogger()

	cfg, err := getConfigWithFlags(cmd)
	if err != nil {
		return err
	}

	baseURL, err := cfg.GoProURL()
	if err != nil {
		return fmt.Errorf("failed to resolve GoPro connection: %v", err)
	}

	client := gopro.NewClient(baseURL, logging.GetLogger())

	inventory, err := media.NewMediaInventory(cmd.Context(), client, cfg.SourceDir())
	if err != nil {
		return err
	}

	for _, file := range inventory.Files {
		switch file.Status {
		case media.StatusSynced:
			path := fmt.Sprintf("%s/%s", file.Directory, file.Filename)

			log.Info("deleting file", slog.String("filename", file.Filename))
			fmt.Printf("Deleting synced file from GoPro: %s\n", file.Filename)

			if err := client.DeleteSingleMediaFile(cmd.Context(), path); err != nil {
				return fmt.Errorf("running cleanup command: %w", err)
			}
		default:
			log.Debug("skipping file", slog.String("filename", file.Filename), slog.String("status", file.Status.String()))
		}
	}

	return nil
}
