package cmd

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/adrg/xdg"
	"github.com/spf13/cobra"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"

	"github.com/EarthmanMuons/herosync/internal/media"
	"github.com/EarthmanMuons/herosync/internal/ytclient"
)

// newPublishCmd constructs the "publish" subcommand.
func newPublishCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "publish",
		Aliases: []string{"pub"},
		Short:   "Upload outgoing videos to YouTube",
		Args:    cobra.ArbitraryArgs,
		RunE:    runPublish,
	}

	cmd.Flags().StringP("title", "t", "", "template for video title")
	cmd.Flags().StringP("description", "d", "", "template for video description")

	return cmd
}

// runPublish is the entry point for the "publish" subcommand.
func runPublish(cmd *cobra.Command, args []string) error {
	ctx, logger, cfg, err := contextLoggerConfig(cmd)
	if err != nil {
		return err
	}

	// Only look at local processed files that are ready to upload.
	inventory, err := media.NewProcessedInventory(cfg.OutgoingMediaDir())
	if err != nil {
		return err
	}

	// Apply filtering only if terms are provided.
	if len(args) > 0 {
		inventory, err = inventory.FilterByDisplayInfo(args)
		if err != nil {
			return err
		}
	}

	scopes := []string{
		youtube.YoutubeReadonlyScope,
		youtube.YoutubeUploadScope,
	}

	logger.Info("creating youtube client", slog.Any("scopes", scopes))

	clientFile := defaultClientSecretPath()
	client := ytclient.New(ctx, clientFile, scopes)

	svc, err := youtube.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("unable to create YouTube service: %v", err)
	}

	call := svc.Channels.List([]string{"snippet"}).Mine(true)
	resp, err := call.Do()
	if err != nil {
		return fmt.Errorf("making API call: %v", err)
	}

	fmt.Printf("Channel: %v\n", resp.Items[0].Snippet.Title)

	publishedAfter, err := inventory.EarliestProcessedDate()
	if err != nil {
		return err
	}

	// Check inventory size and decide whether to apply PublishedAfter optimization.
	inventorySize := len(inventory.Files)
	logger.Info("checking uploaded videos", slog.Int("inventory_size", inventorySize))

	// Fetch uploaded video list
	uploadedVideos, err := getUploadedVideos(svc, inventorySize, publishedAfter)
	if err != nil {
		return err
	}

	// Print uploaded videos for debugging
	printUploadedVideos(uploadedVideos)

	// Convert list to a map for quick lookup.
	uploadedFileMap := make(map[string]*youtube.Video)
	for _, vid := range uploadedVideos {
		if vid.FileDetails != nil && vid.FileDetails.FileName != "" {
			fmt.Printf("filename: %q\n", vid.FileDetails.FileName)
			uploadedFileMap[vid.FileDetails.FileName] = vid
		}
	}

	// // Get user-defined templates
	// titleTemplate, _ := cmd.Flags().GetString("title")
	// descriptionTemplate, _ := cmd.Flags().GetString("description")

	// // Call the upload function
	// return uploadVideos(svc, inventory, uploadedFileMap, titleTemplate, descriptionTemplate, logger)

	return nil
}

func defaultClientSecretPath() string {
	return filepath.Join(xdg.ConfigHome, "herosync", "client_secret.json")
}

func getUploadedVideos(service *youtube.Service, inventorySize int, after time.Time) ([]*youtube.Video, error) {
	// Search for the most recently published videos.
	call := service.Search.List([]string{"snippet"}).
		ForMine(true).
		Type("video").
		Order("date").
		MaxResults(50)

	// // Apply PublishedAfter as an optimization if the local inventory is small.
	// if inventorySize <= 50 {
	// 	call = call.PublishedAfter(after.Format(time.RFC3339))
	// }

	resp, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("making API call: %v", err)
	}

	var videoIDs []string
	for _, item := range resp.Items {
		videoIDs = append(videoIDs, item.Id.VideoId)
	}

	return getVideoDetails(service, videoIDs)
}

func getVideoDetails(service *youtube.Service, videoIDs []string) ([]*youtube.Video, error) {
	if len(videoIDs) == 0 {
		return nil, nil
	}

	call := service.Videos.List([]string{"snippet", "fileDetails", "processingDetails"}).Id(videoIDs...)
	videoResponse, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("fetching video details: %v", err)
	}

	// Debugging: Print processing details
	for _, item := range videoResponse.Items {
		fmt.Printf("Video ID: %s\n", item.Id)
		if item.ProcessingDetails != nil {
			fmt.Printf("  Processing Status: %s\n", item.ProcessingDetails.ProcessingStatus)
			fmt.Printf("  File Availability: %s\n", item.ProcessingDetails.FileDetailsAvailability)
			if item.ProcessingDetails.ProcessingFailureReason != "" {
				fmt.Printf("  Processing Failure Reason: %s\n", item.ProcessingDetails.ProcessingFailureReason)
			}
		} else {
			fmt.Println("  No processing details available")
		}
	}

	return videoResponse.Items, nil
}

func printUploadedVideos(videos []*youtube.Video) {
	fmt.Println("Uploaded Videos:")
	for _, vid := range videos {
		fmt.Printf("ID: %s\nTitle: %s\nFilename: %s\nDate: %v\n\n",
			vid.Id,
			vid.Snippet.Title,
			vid.FileDetails.FileName,
			vid.Snippet.PublishedAt)
	}
	fmt.Println("--------------------------------")
}

// func uploadVideos(svc *youtube.Service, inventory *media.Inventory, uploadedFileMap map[string]*youtube.Video, titleTemplate, descriptionTemplate string, logger *slog.Logger) error {
// 	for _, file := range inventory.Files {
// 		filename := filepath.Base(file.Path)

// 		// Skip if already uploaded
// 		if _, exists := uploadedFileMap[filename]; exists {
// 			logger.Info("Skipping already uploaded video", slog.String("filename", filename))
// 			continue
// 		}

// 		// Extract metadata
// 		metadata := extractMetadata(filename)

// 		// Apply title/description templates
// 		title, err := generateTitle(titleTemplate, metadata)
// 		if err != nil {
// 			logger.Error("Error generating title", slog.String("filename", filename), slog.Any("error", err))
// 			continue
// 		}
// 		description, _ := generateTitle(descriptionTemplate, metadata)

// 		logger.Info("Uploading video", slog.String("filename", filename), slog.String("title", title))

// 		video := &youtube.Video{
// 			Snippet: &youtube.VideoSnippet{
// 				Title:       title,
// 				Description: description,
// 			},
// 			Status: &youtube.VideoStatus{PrivacyStatus: "private"},
// 		}

// 		uploadCall := svc.Videos.Insert([]string{"snippet", "status"}, video, nil)
// 		_, err = uploadCall.Do()
// 		if err != nil {
// 			logger.Error("Error uploading video", slog.String("filename", filename), slog.Any("error", err))
// 			continue
// 		}

// 		logger.Info("Video uploaded successfully", slog.String("title", title))
// 	}
// 	return nil
// }
