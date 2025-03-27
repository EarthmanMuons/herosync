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

const durationTolerance = 100 // milliseconds

// newPublishCmd constructs the "publish" subcommand.
func newPublishCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "publish",
		Aliases: []string{"pub", "upload"},
		Short:   "Upload outgoing videos to YouTube",
		Args:    cobra.ArbitraryArgs,
		RunE:    runPublish,
	}
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

	service, err := youtube.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("unable to create YouTube service: %v", err)
	}

	call := service.Channels.List([]string{"snippet"}).Mine(true)
	resp, err := call.Do()
	if err != nil {
		return fmt.Errorf("making API call: %v", err)
	}

	fmt.Printf("Channel: %v\n", resp.Items[0].Snippet.Title)

	// Fetch uploaded video list.
	uploadedVideos, err := getUploadedVideos(service)
	if err != nil {
		return err
	}

	// Map of recording date to a set of durations (to handle multiple uploads on the same day).
	uploadedFileMap := make(map[string]map[uint64]struct{})

	for _, video := range uploadedVideos {
		if video.RecordingDetails != nil && video.RecordingDetails.RecordingDate != "" {
			key := video.RecordingDetails.RecordingDate
			duration := video.FileDetails.DurationMs

			// Initialize the inner map if it doesn't exist.
			if _, exists := uploadedFileMap[key]; !exists {
				uploadedFileMap[key] = make(map[uint64]struct{})
			}

			// Store the duration in the set for this date.
			uploadedFileMap[key][duration] = struct{}{}
		}
	}

	return uploadVideos(service, inventory, uploadedFileMap, logger)
}

func defaultClientSecretPath() string {
	return filepath.Join(xdg.ConfigHome, "herosync", "client_secret.json")
}

func getUploadedVideos(service *youtube.Service) ([]*youtube.Video, error) {
	// Search for the most recently published videos.
	call := service.Search.List([]string{"snippet"}).
		ForMine(true).
		Type("video").
		Order("date").
		MaxResults(50)

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

	call := service.Videos.List([]string{"fileDetails", "recordingDetails", "snippet"}).Id(videoIDs...)
	videoResponse, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("fetching video details: %v", err)
	}

	return videoResponse.Items, nil
}

func uploadVideos(service *youtube.Service, inventory *media.Inventory, uploadedFileMap map[string]map[uint64]struct{}, logger *slog.Logger) error {
	for _, file := range inventory.Files {
		key := formatRecordingDate(file.CreatedAt)

		shouldUpload := shouldUpload(key, file.Duration, uploadedFileMap)
		if !shouldUpload {
			logger.Info("skipping already uploaded video", slog.String("filename", file.Filename))
			continue
		}

		// If not found, add the current duration to the map for this date.
		if _, exists := uploadedFileMap[key]; !exists {
			uploadedFileMap[key] = make(map[uint64]struct{})
		}
		uploadedFileMap[key][file.Duration] = struct{}{}

		// Proceed with the upload...
		filePath := filepath.Join(file.Directory, file.Filename)
		metadata := extractMetadata(file.Filename)
		title := generateTitle(metadata)
		description := "Uploaded via herosync."
		category := "10" // Music
		keywords := "piano,practice"

		logger.Info("uploading video", slog.String("filename", file.Filename), slog.String("title", title))

		upload := &youtube.Video{
			RecordingDetails: &youtube.VideoRecordingDetails{
				RecordingDate: file.CreatedAt.Format(time.RFC3339),
			},
			Snippet: &youtube.VideoSnippet{
				Title:       title,
				Description: description,
				CategoryId:  category,
			},
			Status: &youtube.VideoStatus{
				ContainsSyntheticMedia: false, // TODO: this doesn't seem to work
				PrivacyStatus:          "private",
			},
		}

		// The API returns a 400 Bad Request response if tags is an empty string.
		if strings.Trim(keywords, "") != "" {
			upload.Snippet.Tags = strings.Split(keywords, ",")
		}

		call := service.Videos.Insert([]string{"recordingDetails", "snippet", "status"}, upload)

		video, err := os.Open(filePath)
		defer video.Close()
		if err != nil {
			logger.Error("opening video", slog.String("filename", file.Filename))
			continue
		}

		resp, err := call.Media(video).AutoLevels(true).Do()
		if err != nil {
			logger.Error("uploading video", slog.String("filename", file.Filename), slog.Any("error", err))
			continue
		}

		logger.Info("video uploaded successfully", slog.String("title", title), slog.String("video-id", resp.Id))
	}

	return nil
}

// formatRecordingDate returns a formatted date string truncated to midnight (UTC).
func formatRecordingDate(t time.Time) string {
	truncated := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return truncated.Format(time.RFC3339)
}

// shouldUpload determines whether the video should be uploaded based on uploaded files and duration tolerance.
func shouldUpload(key string, duration uint64, uploadedFileMap map[string]map[uint64]struct{}) bool {
	// Check if any stored duration for the date is within tolerance.
	if durations, exists := uploadedFileMap[key]; exists {
		for uploadedDuration := range durations {
			if withinTolerance(uploadedDuration, duration) {
				return false
			}
		}
	}
	return true
}

func withinTolerance(a, b uint64) bool {
	if a > b {
		return a-b <= durationTolerance
	}
	return b-a <= durationTolerance
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
