package sync

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/EarthmanMuons/herosync/internal/fsutil"
	"github.com/EarthmanMuons/herosync/internal/gopro"
)

type SyncService struct {
	goproClient *gopro.Client
	outputDir   string
	logger      *slog.Logger
}

func NewSyncService(goproClient *gopro.Client, outputDir string, logger *slog.Logger) *SyncService {
	return &SyncService{
		goproClient: goproClient,
		outputDir:   outputDir,
		logger:      logger,
	}
}

// SyncMedia synchronizes media from the GoPro to the local directory.
func (s *SyncService) SyncMedia(ctx context.Context) error {
	mediaList, err := s.goproClient.GetMediaList(ctx)
	if err != nil {
		return fmt.Errorf("getting media list: %w", err)
	}

	// Convert outputDir to an absolute path
	absOutputDir, err := filepath.Abs(s.outputDir)
	if err != nil {
		return fmt.Errorf("getting absolute path for output directory: %w", err)
	}

	for _, media := range mediaList.Media {
		for _, file := range media.Items {
			localFilePath := filepath.Join(absOutputDir, file.Filename)
			s.logger.Debug("checking download status", "filepath", localFilePath)
			if fsutil.FileExistsAndMatchesSize(localFilePath, file.Size) {
				s.logger.Info("File already exists, and size matches. Skipping", "filename", file.Filename)
				continue // Skip to the next file
			}

			// Download the file.
			err = s.goproClient.DownloadMediaFile(ctx, media.Directory, file.Filename, filepath.Join(absOutputDir, media.Directory))
			if err != nil {
				return fmt.Errorf("downloading file: %w", err)
			}
			s.logger.Info("File downloaded succesfully", "filename", file.Filename)
		}
	}

	return nil
}
