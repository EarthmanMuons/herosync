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
	"github.com/EarthmanMuons/herosync/internal/logging"
	"github.com/EarthmanMuons/herosync/internal/media"
)

var combineCmd = &cobra.Command{
	Use:     "combine",
	Aliases: []string{"merge"},
	Short:   "Merge grouped raw clips into final recordings",
	RunE:    runCombine,
}

type combineOptions struct {
	keep bool
}

var combineOpts combineOptions

func init() {
	combineCmd.Flags().String("group-by", "", "group videos by (media-id, date)")
	combineCmd.Flags().BoolVarP(&combineOpts.keep, "keep-originals", "k", false, "prevent deletion of raw files after combining")
}

func runCombine(cmd *cobra.Command, args []string) error {
	log := logging.GetLogger()

	cfg, err := getConfigWithFlags(cmd)
	if err != nil {
		return err
	}

	baseURL, err := cfg.GoProURL()
	if err != nil {
		return fmt.Errorf("failed to resolve GoPro connection: %v", err)
	}

	client := gopro.NewClient(baseURL, logging.GetLogger())

	inventory, err := media.NewInventory(cmd.Context(), client, cfg.RawMediaDir())
	if err != nil {
		return err
	}

	switch cfg.Group.By {
	case "media-id":
		mediaIDs := inventory.MediaIDs()
		for _, mediaID := range mediaIDs {
			filtered := inventory.FilterByMediaID(mediaID)
			log.Debug("combining files", "media-id", mediaID)

			if err := combineFiles(cmd.Context(), cfg, filtered); err != nil {
				fmt.Errorf("combining by media ID %d: %v", mediaID, err)
			}
		}
	case "date":
		dates := inventory.UniqueDates()
		for _, date := range dates {
			filtered := inventory.FilterByDate(date)
			log.Debug("combining files", "date", date.Format(time.DateOnly))

			if err := combineFiles(cmd.Context(), cfg, filtered); err != nil {
				fmt.Errorf("combining by date %s: %v", date.Format(time.DateOnly), err)
			}
		}
	default:
		return fmt.Errorf("invalid group-by option: %s", cfg.Group.By)
	}

	return nil
}

func combineFiles(ctx context.Context, cfg *config.Config, inv *media.Inventory) error {
	log := logging.GetLogger()

	if len(inv.Files) == 0 {
		log.Info("no files to combine")
		return nil
	}

	if inv.HasUnsyncedFiles() {
		log.Warn("skipping combination; not all group files have been downloaded")
		return nil
	}

	if err := os.MkdirAll(cfg.ProcessedMediaDir(), 0o750); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// Build the list of input files for FFmpeg.
	inputFiles, totalSize, err := prepareFiles(cfg, inv)
	if err != nil {
		return err
	}

	outputFilePath, err := determineOutputFilePath(cfg, inv)
	if err != nil {
		return err
	}
	fmt.Printf("Output file: %s\n", fsutil.ShortenPath(outputFilePath))

	if err := executeFFmpegWithFileList(ctx, cfg, inputFiles, outputFilePath); err != nil {
		return err
	}

	// Set the file's modification time (mtime) to match the first video's creation timestamp.
	if err := os.Chtimes(outputFilePath, time.Now(), inv.Files[0].CreatedAt); err != nil {
		log.Error("failed to set file mtime", slog.String("path", outputFilePath), slog.Time("mtime", inv.Files[0].CreatedAt), slog.Any("error", err))
		return err
	}
	log.Debug("mtime updated", slog.String("path", outputFilePath), slog.Time("timestamp", inv.Files[0].CreatedAt))

	// Verify the file size (within 1% tolerance).
	if err := fsutil.VerifySize(outputFilePath, totalSize, 0.01); err != nil {
		return err
	}

	// Delete the original files if --keep-originals is not set.
	if !combineOpts.keep {
		for _, file := range inv.Files {
			path := filepath.Join(cfg.RawMediaDir(), file.Filename)
			if err := os.Remove(path); err != nil {
				log.Error("failed to delete local file", slog.String("path", path), slog.Any("error", err))
				return err
			}
			log.Info("local file deleted", slog.String("filename", file.Filename))
		}
	}

	return nil
}

// prepareFiles builds the list of input files for FFmpeg and calculates total size.
func prepareFiles(cfg *config.Config, inv *media.Inventory) ([]string, int64, error) {
	var inputFiles []string
	fmt.Println("Combining files:")
	var totalSize int64
	for _, file := range inv.Files {
		fmt.Printf("  %s\n", file.Filename)
		inputFiles = append(inputFiles, fmt.Sprintf("file '%s/%s'", cfg.RawMediaDir(), file.Filename))
		totalSize += file.Size
	}
	return inputFiles, totalSize, nil
}

// determineOutputFilePath determines and creates output file path, handling existing files.
func determineOutputFilePath(cfg *config.Config, inv *media.Inventory) (string, error) {
	outputFilename, err := determineOutputFilename(cfg, inv)
	if err != nil {
		return "", err
	}

	base := outputFilename
	ext := filepath.Ext(outputFilename)
	if ext != "" {
		base = outputFilename[:len(outputFilename)-len(ext)]
	}
	counter := 1
	outputFilePath := filepath.Join(cfg.ProcessedMediaDir(), outputFilename)

	for {
		_, err := os.Stat(outputFilePath)
		if os.IsNotExist(err) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("checking output file existence: %w", err)
		}
		outputFilename = fmt.Sprintf("%s_%d%s", base, counter, ext)
		outputFilePath = filepath.Join(cfg.ProcessedMediaDir(), outputFilename)
		counter++
	}
	return outputFilePath, nil
}

func determineOutputFilename(cfg *config.Config, inv *media.Inventory) (string, error) {
	switch cfg.Group.By {
	case "media-id":
		firstFile := gopro.ParseFilename(inv.Files[0].Filename)
		return fmt.Sprintf("gopro-%04d.mp4", firstFile.MediaID), nil
	case "date":
		return fmt.Sprintf("daily-%s.mp4", inv.Files[0].CreatedAt.Format(time.DateOnly)), nil
	default:
		return "", fmt.Errorf("invalid group-by option: %s", cfg.Group.By)
	}
}

// executeFFmpegWithFileList creates a temp file list, and executes ffmpeg.
func executeFFmpegWithFileList(ctx context.Context, cfg *config.Config, inputFiles []string, outputFilePath string) error {
	tmpFile, err := os.CreateTemp("", "filelist*.txt")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(strings.Join(inputFiles, "\n")); err != nil {
		return fmt.Errorf("writing to temp file: %w", err)
	}

	return executeFFmpeg(ctx, cfg, tmpFile.Name(), outputFilePath)
}

func executeFFmpeg(ctx context.Context, cfg *config.Config, inputFileList, outputFilePath string) error {
	log := logging.GetLogger()

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

	// Only show FFmpeg output if log level is "debug" or higher
	if cfg.Log.Level == "debug" {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stderr = &stdErrBuff
	}

	if err := cmd.Run(); err != nil {
		if cfg.Log.Level != "debug" {
			log.Error(stdErrBuff.String())
		}
		return fmt.Errorf("running ffmpeg: %w", err)
	}

	return nil
}
