package cmd

import (
	"fmt"
	"log/slog"
	// "path/filepath"

	// "github.com/adrg/xdg"
	"github.com/spf13/cobra"
	"google.golang.org/api/youtube/v3"

	yt "github.com/EarthmanMuons/herosync/internal/youtube"
)

// newPublishCmd constructs the "publish" subcommand.
func newPublishCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "publish",
		Aliases: []string{"pub"},
		Short:   "Upload processed videos to YouTube",
		RunE:    runPublish,
	}
	return cmd
}

// runPublish is the entry point for the "publish" subcommand.
func runPublish(cmd *cobra.Command, args []string) error {
	ctx, logger, _, err := contextLoggerConfig(cmd)
	if err != nil {
		return err
	}

	scope := "https://www.googleapis.com/auth/youtube.readonly"

	logger.Info("creating youtube client", slog.String("scope", scope))

	clientFile := defaultClientSecretPath()
	client := yt.GetClient(ctx, clientFile, scope)

	svc, err := youtube.New(client)
	if err != nil {
		return fmt.Errorf("unable to create YouTube service: %v", err)
	}

	call := svc.Channels.List([]string{"mine", "snippet"}).Mine(true)
	resp, err := call.Do()
	if err != nil {
		return fmt.Errorf("making API call: %v", err)
	}

	fmt.Printf("Channel Name: %v\n", resp.Items[0].Snippet.Title)

	return nil
}

// TODO: is proper location $XDG_CONFIG/herosync/client_secret.json???
func defaultClientSecretPath() string {
	// return filepath.Join(xdg.ConfigHome, "herosync", "client_secret.json")
	return "client_secret.json"
}
