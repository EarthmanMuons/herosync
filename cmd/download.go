package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/EarthmanMuons/herosync/internal/gopro"
	"github.com/EarthmanMuons/herosync/internal/logging"
	"github.com/EarthmanMuons/herosync/internal/media"
)

var downloadCmd = &cobra.Command{
	Use:     "download [FILENAME]...",
	Aliases: []string{"dl"},
	Short:   "Fetch new media files from the GoPro",
	Long: `Fetch new media files from the GoPro

If one or more [FILENAME] arguments are provided, only matching files will be
affected.`,
	Args: cobra.ArbitraryArgs,
	RunE: runDownload,
}

type downloadOptions struct {
	force bool
	keep  bool
}

var downloadOpts downloadOptions

func init() {
	downloadCmd.Flags().BoolVarP(&downloadOpts.force, "force", "f", false, "force re-download of existing files")
	downloadCmd.Flags().BoolVarP(&downloadOpts.keep, "keep-originals", "k", false, "prevent cleaning remote files after download")
}

func runDownload(cmd *cobra.Command, args []string) error {
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

	// Iterate through the inventory and download files based on status.
	for _, file := range inventory.Files {
		skipDownload := true

		if file.Status == media.OnlyRemote {
			skipDownload = false
			log.Info("downloading file", slog.String("filename", file.Filename), slog.String("status", file.Status.String()))
		}

		if downloadOpts.force {
			switch file.Status {
			case media.OutOfSync, media.InSync:
				skipDownload = false
				log.Info("force downloading file", slog.String("filename", file.Filename), slog.String("status", file.Status.String()))
			}
		}

		if skipDownload {
			log.Debug("skipping file", slog.String("filename", file.Filename), slog.String("status", file.Status.String()))
			continue
		}

		if err := downloadFile(cmd.Context(), client, &file, cfg.RawMediaDir(), log); err != nil {
			log.Error("failed to download", slog.String("filename", file.Filename), slog.Any("error", err))
			// Don't return here, attempt to download other files.
		}
	}

	return nil
}

// downloadFile handles downloading a single file and preserving its timestamp.
func downloadFile(ctx context.Context, client *gopro.Client, file *media.File, outputDir string, log *slog.Logger) error {
	downloadPath := filepath.Join(outputDir, file.Filename)

	if err := client.DownloadMediaFile(ctx, file.Directory, file.Filename, outputDir); err != nil {
		log.Error("failed to download file", slog.String("filename", file.Filename), slog.Any("error", err))
		return err
	}
	log.Info("download complete", slog.String("filename", file.Filename))

	// Set the file's modification time (mtime) to match the video's creation timestamp.
	if err := os.Chtimes(downloadPath, time.Now(), file.CreatedAt); err != nil {
		log.Error("failed to set file mtime", slog.String("filename", file.Filename), slog.Time("mtime", file.CreatedAt), slog.Any("error", err))
		return err
	}
	log.Debug("mtime updated", slog.String("filename", file.Filename))

	// Verify the file size.
	fileInfo, err := os.Stat(downloadPath)
	if err != nil {
		log.Error("failed to stat downloaded file", slog.String("path", downloadPath), slog.Any("error", err))
		return err
	}

	if fileInfo.Size() != file.Size {
		log.Error("downloaded file size mismatch",
			slog.String("filename", file.Filename),
			slog.Int64("actual", fileInfo.Size()),
			slog.Int64("expected", file.Size),
		)
		return fmt.Errorf("file size mismatch: got %d, expected %d", fileInfo.Size(), file.Size)
	}

	// Delete the remote file if --keep-originals is not specified.
	if !downloadOpts.keep {
		goproPath := fmt.Sprintf("%s/%s", file.Directory, file.Filename)
		if err := client.DeleteSingleMediaFile(ctx, goproPath); err != nil {
			log.Error("failed to delete remote file", slog.String("path", goproPath), slog.Any("error", err))
			return err
		}
		log.Debug("remote file deleted", slog.String("filename", file.Filename))
	}

	return nil
}
