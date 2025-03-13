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

	"github.com/EarthmanMuons/herosync/config"
	"github.com/EarthmanMuons/herosync/internal/fsutil"
	"github.com/EarthmanMuons/herosync/internal/gopro"
	"github.com/EarthmanMuons/herosync/internal/media"
)

// newCombineCmd constructs the "combine" subcommand.
func newCombineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "combine",
		Aliases: []string{"merge"},
		Short:   "Merge grouped raw clips into final recordings",
		RunE:    runCombine,
	}

	cmd.Flags().String("group-by", "", "group videos by (media-id, date)")
	cmd.Flags().BoolP("keep-original", "k", false, "prevent deletion of raw files after combining")

	return cmd
}

// runCombine is the entry point for the "combine" subcommand.
func runCombine(cmd *cobra.Command, args []string) error {
	logger, cfg, err := parseConfigAndLogger(cmd)
	if err != nil {
		return err
	}

	client, err := gopro.NewClient(logger, cfg.GoPro.Scheme, cfg.GoPro.Host)
	if err != nil {
		return fmt.Errorf("failed to initialize GoPro client: %w", err)
	}

	inventory, err := media.NewInventory(cmd.Context(), client, cfg.RawMediaDir())
	if err != nil {
		return err
	}

	keepOriginal, _ := cmd.Flags().GetBool("keep-original")

	switch cfg.Group.By {
	case "media-id":
		return combineByMediaID(cmd.Context(), logger, cfg, inventory, keepOriginal)
	case "date":
		return combineByDate(cmd.Context(), logger, cfg, inventory, keepOriginal)
	default:
		return fmt.Errorf("invalid group-by option: %s", cfg.Group.By)
	}
}

func combineByMediaID(ctx context.Context, logger *slog.Logger, cfg *config.Config, inventory *media.Inventory, keepOriginal bool) error {
	mediaIDs := inventory.MediaIDs()
	if len(mediaIDs) == 0 {
		logger.Debug("no Media IDs found to combine")
		return nil
	}

	for _, mediaID := range mediaIDs {
		filtered, err := inventory.FilterByMediaID(mediaID)
		if err != nil {
			return err
		}

		logger.Debug("combining files", "media-id", mediaID)

		if err := combineFiles(ctx, logger, cfg, filtered, keepOriginal); err != nil {
			return fmt.Errorf("combining by media ID %d: %w", mediaID, err)
		}
	}
	return nil
}

func combineByDate(ctx context.Context, logger *slog.Logger, cfg *config.Config, inventory *media.Inventory, keepOriginal bool) error {
	dates := inventory.UniqueDates()
	if len(dates) == 0 {
		logger.Debug("no dates found to combine")
		return nil
	}

	for _, date := range dates {
		filtered, err := inventory.FilterByDate(date)
		if err != nil {
			return err
		}

		logger.Debug("combining files", "date", date.Format(time.DateOnly))

		if err := combineFiles(ctx, logger, cfg, filtered, keepOriginal); err != nil {
			return fmt.Errorf("combining by date %s: %w", date.Format(time.DateOnly), err)
		}
	}
	return nil
}

func combineFiles(ctx context.Context, logger *slog.Logger, cfg *config.Config, inv *media.Inventory, keepOriginal bool) error {
	if inv.HasUnsyncedFiles() {
		logger.Warn("skipping group; not all files have been downloaded")
		return nil
	}

	// Build the input file list for FFmpeg.
	inputFiles, err := buildFFmpegInputList(cfg, inv)
	if err != nil {
		return err
	}

	outputPath, err := generateOutputPath(cfg, inv)
	if err != nil {
		return err
	}
	fmt.Printf("Output file: %s\n", fsutil.ShortenPath(outputPath))

	if err := runFFmpegWithInputList(ctx, logger, cfg, inputFiles, outputPath); err != nil {
		return err
	}

	// Preserve the modification time from the first video.
	if err := fsutil.SetMtime(logger, outputPath, inv.Files[0].CreatedAt); err != nil {
		return err
	}

	// Verify the file size (within 1% tolerance).
	if err := fsutil.VerifySize(outputPath, inv.TotalSize(), 0.01); err != nil {
		return fmt.Errorf("failed to verify combined file: %w", err)
	}

	// Delete the original files if --keep-original is not set.
	if !keepOriginal {
		for _, file := range inv.Files {
			path := filepath.Join(cfg.RawMediaDir(), file.Filename)
			if err := os.Remove(path); err != nil {
				logger.Error("failed to delete local file", slog.String("path", path), slog.Any("error", err))
				return err
			}
			logger.Info("local file deleted", slog.String("filename", file.Filename))
		}
	}

	return nil
}

// buildFFmpegInputList builds the list of input files for FFmpeg and calculates total size.
func buildFFmpegInputList(cfg *config.Config, inv *media.Inventory) ([]string, error) {
	var inputFiles []string
	fmt.Println("Combining files:")
	for _, file := range inv.Files {
		fmt.Printf("  %s\n", file.Filename)
		inputFiles = append(inputFiles, fmt.Sprintf("file '%s/%s'", cfg.RawMediaDir(), file.Filename))
	}
	return inputFiles, nil
}

// generateOutputPath determines a unique output file path based on the grouping method.
func generateOutputPath(cfg *config.Config, inv *media.Inventory) (string, error) {
	var outputFilename string

	switch cfg.Group.By {
	case "media-id":
		firstFile := gopro.ParseFilename(inv.Files[0].Filename)
		outputFilename = fmt.Sprintf("gopro-%04d.mp4", firstFile.MediaID)
	case "date":
		outputFilename = fmt.Sprintf("daily-%s.mp4", inv.Files[0].CreatedAt.Format(time.DateOnly))
	default:
		return "", fmt.Errorf("invalid group-by option: %s", cfg.Group.By)
	}

	fullPath := filepath.Join(cfg.ProcessedMediaDir(), outputFilename)
	return fsutil.GenerateUniqueFilename(fullPath)
}

// runFFmpegWithInputList creates a temp file list, and executes FFmpeg.
func runFFmpegWithInputList(ctx context.Context, logger *slog.Logger, cfg *config.Config, inputFiles []string, outputFilePath string) error {
	// Ensure the output directory exists before running FFmpeg.
	if err := os.MkdirAll(outputFilePath, 0o750); err != nil {
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

	return runFFmpeg(ctx, logger, cfg, tmpFile.Name(), outputFilePath)
}

func runFFmpeg(ctx context.Context, logger *slog.Logger, cfg *config.Config, inputFileList, outputFilePath string) error {
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

	// Only show FFmpeg output if log level is "debug" or higher.
	if cfg.Log.Level == "debug" {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stderr = &stdErrBuff
	}

	if err := cmd.Run(); err != nil {
		if cfg.Log.Level != "debug" {
			logger.Error(stdErrBuff.String())
		}
		return fmt.Errorf("running ffmpeg: %w", err)
	}

	return nil
}
