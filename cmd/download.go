package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/EarthmanMuons/herosync/internal/fsutil"
	"github.com/EarthmanMuons/herosync/internal/gopro"
	"github.com/EarthmanMuons/herosync/internal/media"
)

type downloadOptions struct {
	logger       *slog.Logger
	client       *gopro.Client
	incomingDir  string
	inventory    *media.Inventory
	force        bool
	keepOriginal bool
}

// newDownloadCmd constructs the "download" subcommand.
func newDownloadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "download [FILENAME]...",
		Aliases: []string{"dl"},
		Short:   "Fetch new media files from the GoPro",
		Long: `Fetch new media files from the GoPro.

If one or more [FILENAME] arguments are provided, only matching files will be affected.`,
		Args: cobra.ArbitraryArgs,
		RunE: runDownload,
	}

	cmd.Flags().BoolP("force", "f", false, "force re-download of existing files")
	cmd.Flags().BoolP("keep-original", "k", false, "prevent deleting remote files after downloading")

	return cmd
}

// runDownload is the entry point for the "download" subcommand.
func runDownload(cmd *cobra.Command, args []string) error {
	ctx, logger, cfg, err := contextLoggerConfig(cmd)
	if err != nil {
		return err
	}

	client, err := gopro.NewClient(logger, cfg.GoPro.Scheme, cfg.GoPro.Host)
	if err != nil {
		return err
	}

	incomingDir := cfg.IncomingMediaDir()

	inventory, err := media.NewInventory(ctx, client, incomingDir)
	if err != nil {
		return err
	}
	inventory, err = inventory.FilterByFilename(args)
	if err != nil {
		return err
	}

	force, _ := cmd.Flags().GetBool("force")
	keepOriginal, _ := cmd.Flags().GetBool("keep-original")

	opts := downloadOptions{
		logger:       logger,
		client:       client,
		incomingDir:  incomingDir,
		inventory:    inventory,
		force:        force,
		keepOriginal: keepOriginal,
	}

	return downloadInventory(ctx, &opts)
}

// downloadInventory handles downloading files based on their sync status.
func downloadInventory(ctx context.Context, opts *downloadOptions) error {
	var errs []error
	for _, file := range opts.inventory.Files {
		shouldDownload := shouldDownload(file, opts.force)
		if !shouldDownload {
			opts.logger.Debug("skipping file", slog.String("filename", file.Filename), slog.String("status", file.Status.String()))
			continue
		}

		opts.logger.Info("downloading file", slog.String("filename", file.Filename), slog.String("status", file.Status.String()))

		if err := downloadAndVerify(ctx, &file, opts); err != nil {
			opts.logger.Error("failed to download", slog.String("filename", file.Filename), slog.Any("error", err))
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// shouldDownload determines whether a file should be downloaded.
func shouldDownload(file media.File, force bool) bool {
	switch file.Status {
	case media.OnlyRemote:
		return true
	case media.OutOfSync, media.InSync:
		return force
	default:
		return false
	}
}

// downloadAndVerify handles downloading a single file and post-download checks.
func downloadAndVerify(ctx context.Context, file *media.File, opts *downloadOptions) error {
	downloadPath := filepath.Join(opts.incomingDir, file.Filename)

	if err := opts.client.DownloadMediaFile(ctx, file.Directory, file.Filename, opts.incomingDir); err != nil {
		return fmt.Errorf("failed to download file %s: %w", file.Filename, err)
	}
	opts.logger.Info("download complete", slog.String("filename", file.Filename))

	// Preserve the modification time.
	if err := fsutil.SetMtime(opts.logger, downloadPath, file.CreatedAt); err != nil {
		return err
	}

	// Verify the file size.
	if err := fsutil.VerifySizeExact(downloadPath, file.Size); err != nil {
		return fmt.Errorf("failed to verify downloaded file: %w", err)
	}

	// Delete the original remote file if --keep-original is not set.
	if !opts.keepOriginal {
		remotePath := fmt.Sprintf("%s/%s", file.Directory, file.Filename)
		if err := opts.client.DeleteSingleMediaFile(ctx, remotePath); err != nil {
			opts.logger.Error("failed to delete remote file", slog.String("path", remotePath), slog.Any("error", err))
			return err
		}
		opts.logger.Debug("remote file deleted", slog.String("filename", file.Filename))
	}

	return nil
}
