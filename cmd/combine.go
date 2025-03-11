package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

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
	Short:   "Merge source clips into final recordings",
	RunE:    runCombine,
}

func init() {
	combineCmd.Flags().String("group-by", "", "group videos by (media-id, date)")
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

	inventory, err := media.NewMediaInventory(cmd.Context(), client, cfg.SourceDir())
	if err != nil {
		return err
	}

	switch cfg.Group.By {
	case "media-id":
		mediaIDs := inventory.GetMediaIDs()
		for _, mediaID := range mediaIDs {
			filtered := inventory.FilterByMediaID(mediaID)
			log.Debug("combining files", "media-id", mediaID)

			if err := combineFiles(cmd.Context(), filtered, cfg); err != nil {
				fmt.Errorf("combining by media ID %d: %v", mediaID, err)
			}
		}
	case "date":
		dates := inventory.GetUniqueDates()
		for _, date := range dates {
			filtered := inventory.FilterByDate(date)
			log.Debug("combining files", "date", date.Format("20060102"))

			if err := combineFiles(cmd.Context(), filtered, cfg); err != nil {
				fmt.Errorf("combining by date %s: %v", date.Format("20060102"), err)
			}
		}
	default:
		panic(fmt.Sprintf("unreachable: invalid group-by option: %s", cfg.Group.By))
	}

	return nil
}

func combineFiles(ctx context.Context, inv *media.MediaInventory, cfg *config.Config) error {
	log := logging.GetLogger()

	if len(inv.Files) == 0 {
		log.Info("no files to combine")
		return nil
	}

	if !inv.AllFilesLocal() {
		log.Warn("skipping combination; not all group files are local")
		return nil
	}

	if err := os.MkdirAll(cfg.FinalDir(), 0o750); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	// Build the list of input files for FFmpeg.
	var inputFiles []string
	fmt.Println("Combining files:")
	for _, file := range inv.Files {
		fmt.Printf("  %s\n", file.Filename)
		inputFiles = append(inputFiles, fmt.Sprintf("file '%s/%s'", cfg.SourceDir(), file.Filename))
	}

	// Create a temporary file for the file list.
	tmpFile, err := os.CreateTemp("", "filelist*.txt")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(strings.Join(inputFiles, "\n")); err != nil {
		return fmt.Errorf("writing to temp file: %w", err)
	}

	firstFile := gopro.ParseFilename(inv.Files[0].Filename)
	outputFilename := strconv.Itoa(firstFile.MediaID)
	var dateString string
	// Prefix filename if the filter includes more than one filename
	if len(inv.Files) > 1 {
		dateString = inv.Files[0].CreatedAt.Format("20060102")
		outputFilename = fmt.Sprintf("%s_%s", dateString, outputFilename)
	}

	fmt.Println("Output file:")
	outputFilePath := fmt.Sprintf("%s/%s", cfg.FinalDir(), outputFilename)
	fmt.Printf("  %s\n", fsutil.ShortenPath(outputFilePath))

	// Execute FFmpeg to concatenate the files.
	cmd := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-f", "concat",
		"-safe", "0",
		"-i", tmpFile.Name(),
		"-c", "copy",
		fmt.Sprintf("%s/%s.mp4", cfg.FinalDir(), outputFilename),
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
