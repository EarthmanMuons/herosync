package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/EarthmanMuons/herosync/internal/gopro"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List media files on connected GoPro",
	RunE:    runList,
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

	client := gopro.NewClient(baseURL)

	mediaList, err := client.GetMediaList(cmd.Context())
	if err != nil {
		return err
	}

	for _, dir := range mediaList.Media {
		for _, item := range dir.Items {
			fmt.Printf("%s (created: %s, size: %d bytes)\n",
				item.Filename,
				item.CreatedAt.Time().Format(time.RFC3339),
				item.Size)
		}
	}

	return nil
}
