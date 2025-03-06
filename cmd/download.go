package cmd

import (
	"fmt"
	// "path/filepath"

	"github.com/spf13/cobra"

	"github.com/EarthmanMuons/herosync/internal/gopro"
	"github.com/EarthmanMuons/herosync/internal/logging"
)

type downloadOptions struct {
	force  bool
	dryRun bool
}

var downloadOpts downloadOptions

var downloadCmd = &cobra.Command{
	Use:     "download",
	Aliases: []string{"dl"},
	Short:   "Fetch new media files from the GoPro",
	RunE:    runDownload,
}

func init() {
	rootCmd.AddCommand(downloadCmd)
}

func runDownload(cmd *cobra.Command, args []string) error {
	log := logging.GetLogger()

	cfg, err := getConfigWithFlags(cmd)
	if err != nil {
		return err
	}

	if err := ensureDir(cfg.Output.Dir); err != nil {
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

	if len(mediaList.Media) == 0 {
		fmt.Println("No media files found on GoPro")
		return nil
	}

	for _, dir := range mediaList.Media {
		for _, item := range dir.Items {
			log.Debug("starting download", "directory", dir.Directory, "filename", item.Filename, "size", item.Size)
			// localPath := filepath.Join(cfg.Output.Dir, item.Filename)
			// if err := client.DownloadMediaFile(dir.Directory, item.Filename, localPath); err != nil {
			// 	return err
			// }
		}
	}

	return nil
}
