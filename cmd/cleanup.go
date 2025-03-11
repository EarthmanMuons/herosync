package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/EarthmanMuons/herosync/internal/gopro"
	"github.com/EarthmanMuons/herosync/internal/logging"
	"github.com/EarthmanMuons/herosync/internal/media"
)

var cleanupCmd = &cobra.Command{
	Use:     "cleanup [FILENAME]...",
	Aliases: []string{"clean"},
	Short:   "Delete transferred media from GoPro storage",
	Long: `Delete transferred media from GoPro storage

By default, only files that have been successfully transferred to local storage
("already in sync") will be deleted from the GoPro.

If the --force flag is used, local files will also be deleted.

USE WITH CAUTION! This means *all* files on the GoPro and *all* videos in the
"raw" output subdirectory on the local machine will be removed. The "processed"
output subdirectory will remain untouched.

If one or more [FILENAME] arguments are provided, only matching files will be
affected.`,
	Args: cobra.ArbitraryArgs,
	RunE: runCleanup,
}

type cleanupOptions struct {
	force bool
}

var cleanupOpts cleanupOptions

func init() {
	cleanupCmd.Flags().BoolVarP(&cleanupOpts.force, "force", "f", false, "delete local files too (UNSAFE)")
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

	// Filenames were specified, so filter the inventory.
	if len(args) > 0 {
		log.Debug("cleaning up specific files", slog.Any("filenames", args))
		inventory = inventory.FilterByFilenames(args)
	}

	// Only remove synced files unless explicitly forced.
	if !cleanupOpts.force {
		log.Debug("limit cleanup to synced files")
		inventory = inventory.FilterByStatus(media.StatusSynced)
	}

	// No eligible files left in the inventory after filtering.
	if len(inventory.Files) == 0 {
		log.Warn("no eligible files to clean up")
		return nil
	}

	for _, file := range inventory.Files {
		// *** GoPro Deletion
		goproPath := fmt.Sprintf("%s/%s", file.Directory, file.Filename)
		log.Info("deleting file from GoPro", slog.String("path", goproPath))

		if err := client.DeleteSingleMediaFile(cmd.Context(), goproPath); err != nil {
			log.Error("failed to delete file", slog.String("path", goproPath), slog.Any("error", err))
		}

		// *** Local Deletion
		if cleanupOpts.force {
			localPath := filepath.Join(cfg.SourceDir(), file.Filename)
			log.Info("deleting local file", slog.String("path", localPath))

			if err := os.Remove(localPath); err != nil {
				if os.IsNotExist(err) {
					log.Warn("local file does not exist", "path", localPath)
				} else {
					log.Error("failed to delete file", slog.String("path", localPath), slog.Any("error", err))
				}
			}
		}
	}

	return nil
}
