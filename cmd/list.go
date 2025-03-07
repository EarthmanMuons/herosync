package cmd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/EarthmanMuons/herosync/internal/fsutil"
	"github.com/EarthmanMuons/herosync/internal/gopro"
	"github.com/EarthmanMuons/herosync/internal/logging"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List media files on connected GoPro",
	RunE:    runList,

	// only stored on gopro = File exists only on the GoPro
	// only stored on local = File exists only locally
	// file has been synced = File exists on both, with matching sizes
	// SIZES ARE MISMATCHED = File exists on both, but sizes differ
}

func init() {
	listCmd.Flags().String("gopro-host", "", "GoPro host (hostname:port or IP)")
	listCmd.Flags().String("gopro-scheme", "", "GoPro scheme (http/https)")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	cfg, err := getConfigWithFlags(cmd)
	if err != nil {
		return err
	}

	baseURL, err := cfg.GetGoProURL()
	if err != nil {
		return fmt.Errorf("failed to resolve GoPro connection: %v", err)
	}

	client := gopro.NewClient(baseURL, logging.GetLogger())

	mediaList, err := client.GetMediaList(cmd.Context())
	if err != nil {
		return err
	}

	// Convert outputDir to an absolute path
	absOutputDir, err := filepath.Abs(cfg.Output.Dir)
	if err != nil {
		return fmt.Errorf("getting absolute path for output directory: %w", err)
	}

	for _, media := range mediaList.Media {
		for _, file := range media.Items {
			localFilePath := filepath.Join(absOutputDir, file.Filename)
			createdAt := file.CreatedAt.Format(time.DateTime)
			humanSize := humanize.Bytes(uint64(file.Size))

			status := "only stored on gopro"
			if fsutil.FileExistsAndMatchesSize(localFilePath, file.Size) {
				status = "file has been synced"
			}

			fmt.Printf("%-14s  %s  %7s   %s\n", file.Filename, createdAt, humanSize, status)
		}
	}

	return nil
}
