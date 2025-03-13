package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/EarthmanMuons/herosync/config"
	"github.com/EarthmanMuons/herosync/internal/gopro"
	"github.com/EarthmanMuons/herosync/internal/media"
)

// newCleanupCmd constructs the "cleanup" subcommand.
func newCleanupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "cleanup [FILENAME]...",
		Aliases: []string{"clean"},
		Short:   "Delete transferred media from GoPro storage",
		Long: `Delete transferred media from GoPro storage.

By default, only files that have been successfully transferred to local storage
("already in sync") will be deleted from the GoPro.

USE FLAGS WITH CAUTION!

* --remote deletes all GoPro files regardless of sync status.
* --local deletes all local files in the "original" output subdirectory.

Combining --remote and --local will delete everything from both GoPro storage
and local original storage. The "processed" output subdirectory will remain
untouched.

If one or more [FILENAME] arguments are provided, only matching files will be
affected.`,
		Args: cobra.ArbitraryArgs,
		RunE: runCleanup,
	}

	cmd.Flags().Bool("remote", false, "delete all files from GoPro storage")
	cmd.Flags().Bool("local", false, "delete all files from local storage")

	return cmd
}

// runCleanup is the entry point for the "cleanup" subcommand.
func runCleanup(cmd *cobra.Command, args []string) error {
	logger, cfg, err := parseConfigAndLogger(cmd)
	if err != nil {
		return err
	}

	client, err := gopro.NewClient(logger, cfg.GoPro.Scheme, cfg.GoPro.Host)
	if err != nil {
		return fmt.Errorf("failed to initialize GoPro client: %w", err)
	}

	inventory, err := media.NewInventory(cmd.Context(), client, cfg.OriginalMediaDir())
	if err != nil {
		return err
	}

	// Apply filename filtering if any were provided.
	inventory, err = inventory.FilterByFilename(args)
	if err != nil {
		return err
	}

	remote, _ := cmd.Flags().GetBool("remote")
	local, _ := cmd.Flags().GetBool("local")

	return cleanupInventory(cmd, logger, client, cfg, inventory, remote, local)
}

// cleanupInventory loops through the inventory and deletes applicable files.
func cleanupInventory(cmd *cobra.Command, logger *slog.Logger, client *gopro.Client, cfg *config.Config, inventory *media.Inventory, remote, local bool) error {
	for _, file := range inventory.Files {
		if err := cleanupFile(cmd, logger, client, cfg, file, remote, local); err != nil {
			logger.Error("cleanup failed", slog.String("filename", file.Filename), slog.Any("error", err))
		}
	}
	return nil
}

// cleanupFile deletes a single file according to the specified cleanup rules.
func cleanupFile(cmd *cobra.Command, logger *slog.Logger, client *gopro.Client, cfg *config.Config, file media.File, remote, local bool) error {
	// Determine whether we should delete remote and/or local versions.
	deleteRemote, deleteLocal := shouldCleanup(file, remote, local)

	if deleteRemote {
		remotePath := fmt.Sprintf("%s/%s", file.Directory, file.Filename)
		logger.Info("deleting remote file", slog.String("path", remotePath))
		if err := client.DeleteSingleMediaFile(cmd.Context(), remotePath); err != nil {
			logger.Error("failed to delete remote file", slog.String("path", remotePath), slog.Any("error", err))
		}
	}

	if deleteLocal {
		localPath := filepath.Join(cfg.OriginalMediaDir(), file.Filename)
		logger.Info("deleting local file", slog.String("path", localPath))
		if err := os.Remove(localPath); err != nil {
			if os.IsNotExist(err) {
				logger.Warn("local file does not exist", slog.String("path", localPath))
			} else {
				logger.Error("failed to delete local file", slog.String("path", localPath), slog.Any("error", err))
			}
		}
	}

	return nil
}

// shouldCleanup determines whether a file should be deleted based on the flags.
func shouldCleanup(file media.File, remote, local bool) (deleteRemote bool, deleteLocal bool) {
	if file.Status == media.InSync {
		if !remote && !local {
			return true, false // default behavior: delete remote, keep local
		} else if !remote && local {
			return false, true // preserve remote file, delete local version only
		}
		return remote, local // follow explicit flag settings
	}

	// Skip files we know don't exist in the respective locations.
	deleteRemote = remote && file.Status != media.OnlyLocal
	deleteLocal = local && file.Status != media.OnlyRemote

	return deleteRemote, deleteLocal
}
