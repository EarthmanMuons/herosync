package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/EarthmanMuons/herosync/internal/fsutil"
	"github.com/EarthmanMuons/herosync/internal/gopro"
	"github.com/EarthmanMuons/herosync/internal/media"
)

type combineOptions struct {
	logger       *slog.Logger
	client       *gopro.Client
	origMediaDir string
	procMediaDir string
	inventory    *media.Inventory
	groupBy      string
	keepOriginal bool
}

// newCombineCmd constructs the "combine" subcommand.
func newCombineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "combine",
		Aliases: []string{"merge"},
		Short:   "Merge original media into processed videos",
		RunE:    runCombine,
	}

	cmd.Flags().String("group-by", "", "group videos by (media-id, date)")
	cmd.Flags().BoolP("keep-original", "k", false, "prevent deleting original files after combining")

	return cmd
}

// runCombine is the entry point for the "combine" subcommand.
func runCombine(cmd *cobra.Command, args []string) error {
	ctx, logger, cfg, err := contextLoggerConfig(cmd)
	if err != nil {
		return err
	}

	client, err := gopro.NewClient(logger, cfg.GoPro.Scheme, cfg.GoPro.Host)
	if err != nil {
		return err
	}

	origMediaDir := cfg.OriginalMediaDir()
	procMediaDir := cfg.ProcessedMediaDir()

	inventory, err := media.NewInventory(ctx, client, origMediaDir)
	if err != nil {
		return err
	}

	groupBy := cfg.Group.By
	keepOriginal, _ := cmd.Flags().GetBool("keep-original")

	opts := combineOptions{
		logger:       logger,
		client:       client,
		origMediaDir: origMediaDir,
		procMediaDir: procMediaDir,
		inventory:    inventory,
		groupBy:      groupBy,
		keepOriginal: keepOriginal,
	}

	switch groupBy {
	case "media-id":
		return combineByMediaID(ctx, &opts)
	case "date":
		return combineByDate(ctx, &opts)
	default:
		return fmt.Errorf("invalid group-by option: %s", groupBy)
	}
}

func combineByMediaID(ctx context.Context, opts *combineOptions) error {
	mediaIDs := opts.inventory.MediaIDs()
	if len(mediaIDs) == 0 {
		opts.logger.Debug("no Media IDs found to combine")
		return nil
	}

	for _, mediaID := range mediaIDs {
		filtered, err := opts.inventory.FilterByMediaID(mediaID)
		if err != nil {
			return err
		}

		opts.logger.Debug("combining files", "media-id", mediaID)

		if err := combineFiles(ctx, filtered, opts); err != nil {
			return fmt.Errorf("combining by media ID %d: %w", mediaID, err)
		}
	}
	return nil
}

func combineByDate(ctx context.Context, opts *combineOptions) error {
	dates := opts.inventory.UniqueDates()
	if len(dates) == 0 {
		opts.logger.Debug("no dates found to combine")
		return nil
	}

	for _, date := range dates {
		filtered, err := opts.inventory.FilterByDate(date)
		if err != nil {
			return err
		}

		opts.logger.Debug("combining files", "date", date.Format(time.DateOnly))

		if err := combineFiles(ctx, filtered, opts); err != nil {
			return fmt.Errorf("combining by date %s: %w", date.Format(time.DateOnly), err)
		}
	}
	return nil
}

func combineFiles(ctx context.Context, inv *media.Inventory, opts *combineOptions) error {
	if inv.HasUnsyncedFiles() {
		opts.logger.Warn("skipping group; not all files have been downloaded")
		return nil
	}

	inputFiles, err := buildFFmpegInputList(inv, opts.origMediaDir)
	if err != nil {
		return err
	}

	outputPath, err := generateOutputPath(inv, opts.groupBy, opts.procMediaDir)
	if err != nil {
		return err
	}
	fmt.Printf("Output file: %s\n", fsutil.ShortenPath(outputPath))

	if err := runFFmpegWithInputList(ctx, inputFiles, outputPath, opts); err != nil {
		return err
	}

	// Preserve the modification time from the first video.
	if err := fsutil.SetMtime(opts.logger, outputPath, inv.Files[0].CreatedAt); err != nil {
		return err
	}

	// Verify the file size (within 1% tolerance).
	if err := fsutil.VerifySize(outputPath, inv.TotalSize(), 0.01); err != nil {
		return fmt.Errorf("failed to verify combined file: %w", err)
	}

	// Delete the original files if --keep-original is not set.
	if !opts.keepOriginal {
		for _, file := range inv.Files {
			path := filepath.Join(opts.origMediaDir, file.Filename)
			if err := os.Remove(path); err != nil {
				opts.logger.Error("failed to delete local file", slog.String("path", path), slog.Any("error", err))
				return err
			}
			opts.logger.Info("local file deleted", slog.String("filename", file.Filename))
		}
	}

	return nil
}

// buildFFmpegInputList builds the list of input files for FFmpeg and calculates total size.
func buildFFmpegInputList(inv *media.Inventory, mediaDir string) ([]string, error) {
	var inputFiles []string
	fmt.Println("Combining files:")
	for _, file := range inv.Files {
		fmt.Printf("  %s\n", file.Filename)
		inputFiles = append(inputFiles, fmt.Sprintf("file '%s/%s'", mediaDir, file.Filename))
	}
	return inputFiles, nil
}

// generateOutputPath determines a unique output file path based on the grouping method.
func generateOutputPath(inv *media.Inventory, groupBy string, mediaDir string) (string, error) {
	var outputFilename string

	switch groupBy {
	case "media-id":
		firstFile := gopro.ParseFilename(inv.Files[0].Filename)
		outputFilename = fmt.Sprintf("gopro-%04d.mp4", firstFile.MediaID)
	case "date":
		outputFilename = fmt.Sprintf("daily-%s.mp4", inv.Files[0].CreatedAt.Format(time.DateOnly))
	default:
		return "", fmt.Errorf("invalid group-by option: %s", groupBy)
	}

	fullPath := filepath.Join(mediaDir, outputFilename)
	return fsutil.GenerateUniqueFilename(fullPath)
}

// runFFmpegWithInputList creates a temp file list, and executes FFmpeg.
func runFFmpegWithInputList(ctx context.Context, inputFiles []string, outputFilePath string, opts *combineOptions) error {
	// Ensure the output directory exists before running FFmpeg.
	if err := os.MkdirAll(filepath.Dir(outputFilePath), 0o750); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	tmpFile, err := os.CreateTemp("", "filelist*.txt")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(strings.Join(inputFiles, "\n")); err != nil {
		return fmt.Errorf("writing to temp file: %w", err)
	}

	return runFFmpeg(ctx, tmpFile.Name(), outputFilePath, opts)
}

func runFFmpeg(ctx context.Context, inputFileList, outputFilePath string, opts *combineOptions) error {
	cmd := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-f", "concat",
		"-safe", "0",
		"-i", inputFileList,
		"-c", "copy",
		outputFilePath,
	)

	var stdErrBuff strings.Builder

	// Suppress ffmpeg output unless debugging is enabled.
	if opts.logger.Enabled(ctx, slog.LevelDebug) {
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stderr = &stdErrBuff
	}

	if err := cmd.Run(); err != nil {
		// If debugging is off, print any captured stderr logs on failure.
		if !opts.logger.Enabled(ctx, slog.LevelDebug) {
			opts.logger.Error(stdErrBuff.String())
		}
		return fmt.Errorf("running ffmpeg: %w", err)
	}

	return nil
}
