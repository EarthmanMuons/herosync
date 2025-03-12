package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/EarthmanMuons/herosync/internal/fsutil"
	"github.com/EarthmanMuons/herosync/internal/gopro"
	"github.com/EarthmanMuons/herosync/internal/media"
)

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
	cmd.Flags().BoolP("keep-original", "k", false, "prevent cleaning remote files after download")

	return cmd
}

// runDownload is the entry point for the "download" subcommand.
func runDownload(cmd *cobra.Command, args []string) error {
	logger, cfg, err := parseConfigAndLogger(cmd)
	if err != nil {
		return err
	}

	client, err := gopro.NewClient(logger, cfg.GoPro.Scheme, cfg.GoPro.Host)
	if err != nil {
		return fmt.Errorf("failed to initialize GoPro client: %w", err)
	}

	inventory, err := media.NewInventory(cmd.Context(), client, cfg.RawMediaDir())
	if err != nil {
		return err
	}

	// Apply filename filtering if any were provided.
	inventory, err = inventory.FilterByFilename(args)
	if err != nil {
		return err
	}

	force, _ := cmd.Flags().GetBool("force")
	keepOriginal, _ := cmd.Flags().GetBool("keep-original")

	return downloadInventory(cmd.Context(), logger, client, inventory, cfg.RawMediaDir(), force, keepOriginal)
}

// downloadInventory handles downloading files based on their sync status.
func downloadInventory(ctx context.Context, logger *slog.Logger, client *gopro.Client, inventory *media.Inventory, outputDir string, force, keepOriginal bool) error {
	var encounteredError error

	for _, file := range inventory.Files {
		shouldDownload := shouldDownload(file, force)
		if !shouldDownload {
			logger.Debug("skipping file", slog.String("filename", file.Filename), slog.String("status", file.Status.String()))
			continue
		}

		logger.Info("downloading file", slog.String("filename", file.Filename), slog.String("status", file.Status.String()))

		if err := downloadAndVerify(ctx, logger, client, &file, outputDir, keepOriginal); err != nil {
			logger.Error("failed to download", slog.String("filename", file.Filename), slog.Any("error", err))
			encounteredError = err // capture the error but continue processing remaining files
		}
	}

	return encounteredError
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
func downloadAndVerify(ctx context.Context, logger *slog.Logger, client *gopro.Client, file *media.File, outputDir string, keepOriginal bool) error {
	downloadPath := filepath.Join(outputDir, file.Filename)

	if err := client.DownloadMediaFile(ctx, file.Directory, file.Filename, outputDir); err != nil {
		return fmt.Errorf("failed to download file %s: %w", file.Filename, err)
	}
	logger.Info("download complete", slog.String("filename", file.Filename))

	// Preserve the modification time.
	if err := fsutil.SetMtime(logger, downloadPath, file.CreatedAt); err != nil {
		return err
	}

	// Verify the file size.
	if err := fsutil.VerifySizeExact(downloadPath, file.Size); err != nil {
		return fmt.Errorf("failed to verify downloaded file: %w", err)
	}

	// Delete the original remote file if --keep-original is not set.
	if !keepOriginal {
		remotePath := fmt.Sprintf("%s/%s", file.Directory, file.Filename)
		if err := client.DeleteSingleMediaFile(ctx, remotePath); err != nil {
			logger.Error("failed to delete remote file", slog.String("path", remotePath), slog.Any("error", err))
			return err
		}
		logger.Debug("remote file deleted", slog.String("filename", file.Filename))
	}

	return nil
}
