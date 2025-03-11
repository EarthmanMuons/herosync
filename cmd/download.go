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
	Use:     "download",
	Aliases: []string{"dl"},
	Short:   "Fetch new media files from the GoPro",
	RunE:    runDownload,
}

func init() {
	downloadCmd.Flags().String("gopro-host", "", "GoPro host (hostname:port or IP)")
	downloadCmd.Flags().String("gopro-scheme", "", "GoPro scheme (http/https)")
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

	inventory, err := media.NewMediaInventory(cmd.Context(), client, cfg.RawMediaDir())
	if err != nil {
		return err
	}

	// Iterate through the inventory and download files based on status.
	for _, file := range inventory.Files {
		switch file.Status {
		case media.StatusOnlyGoPro, media.StatusDifferent:
			log.Info("downloading file", slog.String("filename", file.Filename), slog.String("status", file.Status.String()))

			if err := downloadFile(cmd.Context(), client, &file, cfg.RawMediaDir(), log); err != nil {
				continue // Skip to the next file.
			}
		default:
			log.Debug("skipping file", slog.String("filename", file.Filename), slog.String("status", file.Status.String()))
		}
	}

	return nil
}

// downloadFile handles downloading a single file and preserving its timestamp.
func downloadFile(ctx context.Context, client *gopro.Client, file *media.MediaFile, outputDir string, log *slog.Logger) error {
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

	return nil
}
