package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/EarthmanMuons/herosync/internal/gopro"
	"github.com/EarthmanMuons/herosync/internal/media"
)

type cleanupOptions struct {
	logger    *slog.Logger
	client    *gopro.Client
	outputDir string
	inventory *media.Inventory
	remote    bool
	local     bool
}

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
	ctx, logger, cfg, err := contextLoggerConfig(cmd)
	if err != nil {
		return err
	}

	client, err := gopro.NewClient(logger, cfg.GoPro.Scheme, cfg.GoPro.Host)
	if err != nil {
		return err
	}

	outputDir := cfg.OriginalMediaDir()

	inventory, err := media.NewInventory(ctx, client, outputDir)
	if err != nil {
		return err
	}
	inventory, err = inventory.FilterByFilename(args)
	if err != nil {
		return err
	}

	remote, _ := cmd.Flags().GetBool("remote")
	local, _ := cmd.Flags().GetBool("local")

	opts := cleanupOptions{
		logger:    logger,
		client:    client,
		outputDir: outputDir,
		inventory: inventory,
		remote:    remote,
		local:     local,
	}

	return cleanupInventory(ctx, &opts)
}

// cleanupInventory loops through the inventory and deletes applicable files.
func cleanupInventory(ctx context.Context, opts *cleanupOptions) error {
	for _, file := range opts.inventory.Files {
		if err := cleanupFile(ctx, &file, opts); err != nil {
			opts.logger.Error("cleanup failed", slog.String("filename", file.Filename), slog.Any("error", err))
		}
	}
	return nil
}

// cleanupFile deletes a single file according to the specified cleanup rules.
func cleanupFile(ctx context.Context, file *media.File, opts *cleanupOptions) error {
	// Determine whether we should delete remote and/or local versions.
	deleteRemote, deleteLocal := shouldCleanup(file, opts.remote, opts.local)

	if deleteRemote {
		remotePath := fmt.Sprintf("%s/%s", file.Directory, file.Filename)
		opts.logger.Info("deleting remote file", slog.String("path", remotePath))
		if err := opts.client.DeleteSingleMediaFile(ctx, remotePath); err != nil {
			opts.logger.Error("failed to delete remote file", slog.String("path", remotePath), slog.Any("error", err))
		}
	}

	if deleteLocal {
		localPath := filepath.Join(opts.outputDir, file.Filename)
		opts.logger.Info("deleting local file", slog.String("path", localPath))
		if err := os.Remove(localPath); err != nil {
			if os.IsNotExist(err) {
				opts.logger.Warn("local file does not exist", slog.String("path", localPath))
			} else {
				opts.logger.Error("failed to delete local file", slog.String("path", localPath), slog.Any("error", err))
			}
		}
	}

	return nil
}

// shouldCleanup determines whether a file should be deleted based on the flags.
func shouldCleanup(file *media.File, remote, local bool) (deleteRemote bool, deleteLocal bool) {
	if file.Status == media.InSync {
		if !remote && !local {
			return true, false // default behavior: delete remote, keep local
		}
		return remote, local // follow explicit flag settings
	}

	// Skip files we know don't exist in the respective locations.
	deleteRemote = remote && file.Status != media.OnlyLocal
	deleteLocal = local && file.Status != media.OnlyRemote

	return deleteRemote, deleteLocal
}
