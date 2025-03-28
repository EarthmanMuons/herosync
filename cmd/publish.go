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

	"github.com/EarthmanMuons/herosync/config"
	"github.com/EarthmanMuons/herosync/internal/media"
	"github.com/EarthmanMuons/herosync/internal/ytclient"
)

type publishOptions struct {
	logger            *slog.Logger
	cfg               *config.Config
	inventory         *media.Inventory
	service           *youtube.Service
	uploadedDurations map[string]map[uint64]struct{}
}

var (
	counterRe = regexp.MustCompile(`_(\d+)$`)
	mediaRe   = regexp.MustCompile(`^gopro-0*(\d+)$`)
	dateRe    = regexp.MustCompile(`^daily-(\d{4}-\d{2}-\d{2})$`)
)

const durationTolerance = 100 // max milliseconds difference to consider videos identical

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
	inventory, err := media.NewProcessedInventory(ctx, cfg.OutgoingMediaDir())
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

	logger.Debug("creating youtube client", slog.Any("scopes", scopes))

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

	logger.Debug("connected to youtube", slog.String("channel", resp.Items[0].Snippet.Title))

	uploadedVideos, err := getUploadedVideos(service)
	if err != nil {
		return err
	}

	// Map of recording date to a set of durations (to handle multiple uploads on the same day).
	uploadedDurations := make(map[string]map[uint64]struct{})

	for _, video := range uploadedVideos {
		if video.RecordingDetails != nil && video.RecordingDetails.RecordingDate != "" {
			key := video.RecordingDetails.RecordingDate
			duration := video.FileDetails.DurationMs

			// Initialize the inner map if it doesn't exist.
			if _, exists := uploadedDurations[key]; !exists {
				uploadedDurations[key] = make(map[uint64]struct{})
			}

			// Store the duration in the set for this date.
			uploadedDurations[key][duration] = struct{}{}
		}
	}

	opts := &publishOptions{
		logger:            logger,
		cfg:               cfg,
		inventory:         inventory,
		service:           service,
		uploadedDurations: uploadedDurations,
	}

	return uploadVideos(opts)
}

func defaultClientSecretPath() string {
	return filepath.Join(xdg.ConfigHome, "herosync", "client_secret.json")
}

func getUploadedVideos(service *youtube.Service) ([]*youtube.Video, error) {
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

func uploadVideos(opts *publishOptions) error {
	for _, file := range opts.inventory.Files {
		key := formatRecordingDate(file.CreatedAt)

		if !shouldUpload(key, file.Duration, opts.uploadedDurations) {
			opts.logger.Info("skipping already uploaded video", slog.String("filename", file.Filename))
			continue
		}

		// Update the durations map for this date.
		if _, exists := opts.uploadedDurations[key]; !exists {
			opts.uploadedDurations[key] = make(map[uint64]struct{})
		}
		opts.uploadedDurations[key][file.Duration] = struct{}{}

		title := generateTitle(opts.cfg, file.Filename)
		opts.logger.Info("uploading video", slog.String("filename", file.Filename), slog.String("title", title))

		// Open video file.
		videoPath := filepath.Join(file.Directory, file.Filename)
		videoFile, err := os.Open(videoPath)
		if err != nil {
			opts.logger.Error("opening video", slog.String("filename", file.Filename))
			continue
		}
		defer videoFile.Close()

		videoID, err := processUpload(file, title, videoFile, opts)
		if err != nil {
			opts.logger.Error("uploading video", slog.String("filename", file.Filename), slog.Any("error", err))
			continue
		}

		opts.logger.Info("video uploaded successfully", slog.String("title", title), slog.String("video-id", videoID))
	}
	return nil
}

// processUpload handles the actual API call for a single video upload.
func processUpload(file media.File, title string, videoFile *os.File, opts *publishOptions) (string, error) {
	upload := &youtube.Video{
		RecordingDetails: &youtube.VideoRecordingDetails{
			RecordingDate: file.CreatedAt.Format(time.RFC3339),
		},
		Snippet: &youtube.VideoSnippet{
			Title:       title,
			Description: opts.cfg.Video.Description,
			CategoryId:  opts.cfg.Video.CategoryID,
		},
		Status: &youtube.VideoStatus{
			PrivacyStatus: opts.cfg.Video.PrivacyStatus,
		},
	}

	// The API returns a 400 Bad Request response if tags is an empty string.
	if trimmedTags := strings.TrimSpace(opts.cfg.Video.Tags); trimmedTags != "" {
		upload.Snippet.Tags = strings.Split(trimmedTags, ",")
	}

	call := opts.service.Videos.Insert([]string{"recordingDetails", "snippet", "status"}, upload)
	resp, err := call.Media(videoFile).
		ProgressUpdater(func(current, _ int64) {
			total := file.Size
			progress := float64(current) / float64(total) * 100
			opts.logger.Info("upload progress",
				slog.String("filename", file.Filename),
				slog.Int64("written", current),
				slog.Int64("total", total),
				slog.String("progress", fmt.Sprintf("%.2f%%", progress)),
			)
		}).Do()
	if err != nil {
		return "", err
	}
	return resp.Id, nil
}

// formatRecordingDate returns a formatted date string truncated to midnight (UTC).
func formatRecordingDate(t time.Time) string {
	truncated := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	return truncated.Format(time.RFC3339)
}

// shouldUpload determines whether the video should be uploaded based on uploaded files and duration tolerance.
func shouldUpload(key string, duration uint64, uploadedDurations map[string]map[uint64]struct{}) bool {
	if durations, exists := uploadedDurations[key]; exists {
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

func generateTitle(cfg *config.Config, filename string) string {
	metadata := extractMetadata(filename)
	title := cfg.Video.Title

	for key, value := range metadata {
		placeholder := fmt.Sprintf("${%s}", key)
		title = strings.ReplaceAll(title, placeholder, value)
	}

	return strings.TrimSpace(title)
}

// extractMetadata parses the filename to extract metadata, including media ID or date
func extractMetadata(filename string) map[string]string {
	metadata := map[string]string{"counter": ""}
	baseFilename := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Match counter suffix if present (e.g., "_1", "_2").
	counterMatch := counterRe.FindStringSubmatch(baseFilename)
	if len(counterMatch) > 1 {
		metadata["counter"] = counterMatch[1]
		baseFilename = strings.TrimSuffix(baseFilename, "_"+counterMatch[1])
	}

	// Match GoPro media ID filenames: gopro-<MEDIA-ID>
	mediaMatch := mediaRe.FindStringSubmatch(baseFilename)
	if len(mediaMatch) > 1 {
		metadata["type"] = "chapters"
		metadata["media_id"] = mediaMatch[1]
		metadata["identifier"] = mediaMatch[1]
		return metadata
	}

	// Match date-based filenames: daily-YYYY-MM-DD
	dateMatch := dateRe.FindStringSubmatch(baseFilename)
	if len(dateMatch) > 1 {
		metadata["type"] = "date"
		metadata["date"] = dateMatch[1]
		metadata["identifier"] = dateMatch[1]
		return metadata
	}

	// Fallback: use the whole base filename as a generic identifier.
	metadata["type"] = "unknown"
	metadata["identifier"] = baseFilename

	return metadata
}
