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
	Args:    cobra.ArbitraryArgs,
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

	if len(args) > 0 {
		// Filenames were provided. Filter by filename.
		log.Debug("cleaning up specific files", slog.Any("filenames", args))
		inventory = inventory.FilterByFilenames(args)
	} else {
		// No filenames provided. Filter by StatusSynced.
		log.Debug("cleaning up all synced files")
		inventory = inventory.FilterByStatus(media.StatusSynced)
	}

	if len(inventory.Files) == 0 {
		if len(args) > 0 {
			log.Warn("no files found for input arguments", "files", args)
		} else {
			log.Warn("no files eligible to clean up")
		}
		return nil
	}

	for _, file := range inventory.Files {
		path := fmt.Sprintf("%s/%s", file.Directory, file.Filename)

		log.Info("deleting file", slog.String("filename", file.Filename))
		fmt.Printf("Deleting file from GoPro: %s\n", file.Filename)

		if err := client.DeleteSingleMediaFile(cmd.Context(), path); err != nil {
			log.Error("failed to delete file", slog.String("filename", file.Filename), slog.Any("error", err))
			fmt.Printf("Error deleting file %s: %v\n", file.Filename, err)
		}
	}

	return nil
}
