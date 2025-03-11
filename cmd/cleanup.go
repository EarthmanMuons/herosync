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

USE FLAGS WITH CAUTION!

* --remote deletes all GoPro files regardless of sync status.
* --local deletes all local files in the "raw" output subdirectory.

Combining --remote and --local will delete everything from both GoPro storage
and local raw storage. The "processed" output subdirectory will remain
untouched.

If one or more [FILENAME] arguments are provided, only matching files will be
affected.`,
	Args: cobra.ArbitraryArgs,
	RunE: runCleanup,
}

type cleanupOptions struct {
	remote bool
	local  bool
}

var cleanupOpts cleanupOptions

func init() {
	cleanupCmd.Flags().BoolVar(&cleanupOpts.remote, "remote", false, "delete all files from GoPro storage")
	cleanupCmd.Flags().BoolVar(&cleanupOpts.local, "local", false, "delete all files from local storage")
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

	inventory, err := media.NewInventory(cmd.Context(), client, cfg.RawMediaDir())
	if err != nil {
		return err
	}

	// Apply filename filtering if any were provided.
	if len(args) > 0 {
		log.Debug("filtering by filename", slog.Any("args", args))
		inventory = inventory.FilterByFilename(args)

		if len(inventory.Files) == 0 {
			log.Error("no matching files", slog.Any("args", args))
			os.Exit(1)
		}
	}

	// Flags Used         | Deletes Synced GoPro Files | Deletes Unsynced GoPro Files | Deletes Local Raw Files
	// ------------------ | -------------------------- | ---------------------------- | -----------------------
	// (default)          | Yes                        | No                           | No
	// --remote           | Yes                        | Yes                          | No
	// --local            | No                         | No                           | Yes
	// --remote --local   | Yes                        | Yes                          | Yes

	for _, file := range inventory.Files {
		// *** GoPro Deletion
		goproPath := fmt.Sprintf("%s/%s", file.Directory, file.Filename)

		// Only delete synced files unless explicit --remote flag was provided.
		if !cleanupOpts.remote && file.Status != media.InSync {
			if file.Status == media.OnlyRemote {
				log.Debug("skipping unsynced file deletion", slog.String("path", goproPath))
			}
		} else if !cleanupOpts.remote && cleanupOpts.local && file.Status == media.InSync {
			log.Debug("skipping to prioritize local deletion", slog.String("path", goproPath))
		} else if file.Status != media.OnlyLocal {
			log.Info("deleting GoPro file", slog.String("path", goproPath))

			if err := client.DeleteSingleMediaFile(cmd.Context(), goproPath); err != nil {
				log.Error("failed to delete file", slog.String("path", goproPath), slog.Any("error", err))
			}
		}

		// *** Local Deletion
		if cleanupOpts.local && file.Status != media.OnlyRemote {
			localPath := filepath.Join(cfg.RawMediaDir(), file.Filename)
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
