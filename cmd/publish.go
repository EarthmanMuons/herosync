package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

	// publishedAfter, err := inventory.EarliestProcessedDate()
	// if err != nil {
	// 	return err
	// }

	// // Check inventory size and decide whether to apply PublishedAfter optimization.
	// inventorySize := len(inventory.Files)
	// logger.Info("checking uploaded videos", slog.Int("inventory_size", inventorySize))

	// // Fetch uploaded video list
	// uploadedVideos, err := getUploadedVideos(svc, inventorySize, publishedAfter)
	// if err != nil {
	// 	return err
	// }

	// // DEBUG: print uploaded video details
	// printUploadedVideos(uploadedVideos)

	// // Convert list to a map for quick lookup.
	// uploadedFileMap := make(map[string]*youtube.Video)
	// for _, vid := range uploadedVideos {
	// 	if vid.FileDetails != nil && vid.FileDetails.FileName != "" {
	//      // DEBUG: upstream bug that no file details are ever returned
	// 		fmt.Printf("filename: %q\n", vid.FileDetails.FileName)
	// 		uploadedFileMap[vid.FileDetails.FileName] = vid
	// 	}
	// }

	// Call the upload function
	return uploadVideos(svc, inventory, logger)

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

	// // DEBUG: print processing details
	// for _, item := range videoResponse.Items {
	// 	fmt.Printf("Video ID: %s\n", item.Id)
	// 	if item.ProcessingDetails != nil {
	// 		fmt.Printf("  Processing Status: %s\n", item.ProcessingDetails.ProcessingStatus)
	// 		fmt.Printf("  File Availability: %s\n", item.ProcessingDetails.FileDetailsAvailability)
	// 		if item.ProcessingDetails.ProcessingFailureReason != "" {
	// 			fmt.Printf("  Processing Failure Reason: %s\n", item.ProcessingDetails.ProcessingFailureReason)
	// 		}
	// 	} else {
	// 		fmt.Println("  No processing details available")
	// 	}
	// }

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

func uploadVideos(service *youtube.Service, inventory *media.Inventory, logger *slog.Logger) error {
	for _, file := range inventory.Files {
		// // Skip if already uploaded
		// if _, exists := uploadedFileMap[file.Filename]; exists {
		// 	logger.Info("Skipping already uploaded video", slog.String("filename", filename))
		// 	continue
		// }

		filePath := filepath.Join(file.Directory, file.Filename)

		metadata := extractMetadata(file.Filename)
		title := generateTitle(metadata)
		description := "Uploaded via herosync."
		category := "10"
		keywords := ""

		logger.Info("uploading video", slog.String("filename", file.Filename), slog.String("title", title))

		upload := &youtube.Video{
			Snippet: &youtube.VideoSnippet{
				Title:       title,
				Description: description,
				CategoryId:  category,
			},
			Status: &youtube.VideoStatus{PrivacyStatus: "private"},
		}

		// The API returns a 400 Bad Request response if tags is an empty string.
		if strings.Trim(keywords, "") != "" {
			upload.Snippet.Tags = strings.Split(keywords, ",")
		}

		// TODO: set these:
		// recordingDetails.recordingDate
		// status.containsSyntheticMedia

		call := service.Videos.Insert([]string{"snippet", "status"}, upload)

		video, err := os.Open(filePath)
		defer video.Close()
		if err != nil {
			logger.Error("opening file", slog.String("filename", file.Filename))
			continue
		}

		resp, err := call.Media(video).Do()
		if err != nil {
			logger.Error("uploading video", slog.String("filename", file.Filename), slog.Any("error", err))
			continue
		}

		logger.Info("video uploaded successfully", slog.String("title", title), slog.String("response-id", resp.Id))
	}

	return nil
}

// extractMetadata parses the filename to extract metadata, including media ID or date
func extractMetadata(filename string) map[string]string {
	metadata := map[string]string{}
	baseFilename := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Match counter suffix if present (e.g., "_1", "_2").
	counterRe := regexp.MustCompile(`_(\d+)$`)
	counterMatch := counterRe.FindStringSubmatch(baseFilename)
	counter := ""
	if len(counterMatch) > 1 {
		counter = counterMatch[1]
		// Strip the counter from the base filename
		baseFilename = strings.TrimSuffix(baseFilename, "_"+counter)
	}
	metadata["counter"] = counter

	// Match GoPro media ID filenames: gopro-<MEDIA-ID>
	mediaRe := regexp.MustCompile(`^gopro-0*(\d+)$`)
	mediaMatch := mediaRe.FindStringSubmatch(baseFilename)
	if len(mediaMatch) > 1 {
		metadata["type"] = "chapters"
		metadata["media_id"] = mediaMatch[1]
		return metadata
	}

	// Match date-based filenames: daily-YYYY-MM-DD
	dateRe := regexp.MustCompile(`^daily-(\d{4}-\d{2}-\d{2})$`)
	dateMatch := dateRe.FindStringSubmatch(baseFilename)
	if len(dateMatch) > 1 {
		metadata["type"] = "date"
		metadata["date"] = dateMatch[1]
		return metadata
	}

	// Fallback: use the whole base filename as a generic identifier.
	metadata["type"] = "unknown"
	metadata["identifier"] = baseFilename

	return metadata
}

// generateTitle generates a title based on extracted metadata
func generateTitle(metadata map[string]string) string {
	var title string

	switch metadata["type"] {
	case "chapters":
		title = fmt.Sprintf("Media ID: %s", metadata["media_id"])
		if metadata["counter"] != "" {
			title += fmt.Sprintf(" (Part %s)", metadata["counter"])
		}
	case "date":
		title = fmt.Sprintf("%s", metadata["date"])
		if metadata["counter"] != "" {
			title += fmt.Sprintf(" (Part %s)", metadata["counter"])
		}
	default:
		title = metadata["identifier"]
		if metadata["counter"] != "" {
			title += fmt.Sprintf(" (Part %s)", metadata["counter"])
		}
	}

	return title
}
