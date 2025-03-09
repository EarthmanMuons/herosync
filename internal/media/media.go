package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/EarthmanMuons/herosync/internal/gopro"
)

// FileStatus represents the synchronization status of a file.
type FileStatus int

const (
	StatusOnlyGoPro FileStatus = iota // File exists only on the GoPro
	StatusOnlyLocal                   // File exists only locally
	StatusSynced                      // File exists on both, with matching sizes
	StatusDifferent                   // File exists on both, but sizes differ
	StatusError                       // Represents stat error
)

// String provides a human-readable representation of the FileStatus.
func (fs FileStatus) String() string {
	switch fs {
	case StatusOnlyGoPro:
		return "only stored on gopro"
	case StatusOnlyLocal:
		return "only stored on local"
	case StatusSynced:
		return "file already in sync"
	case StatusDifferent:
		return "SIZES ARE MISMATCHED"
	case StatusError:
		return "stat error"
	default:
		return fmt.Sprintf("unknown status (%d)", int(fs))
	}
}

// Symbol provides a symbolic representation of the FileStatus.
func (fs FileStatus) Symbol() string {
	switch fs {
	case StatusOnlyGoPro:
		return "«"
	case StatusOnlyLocal:
		return "»"
	case StatusSynced:
		return "="
	case StatusDifferent:
		return "!"
	case StatusError:
		return "?"
	default:
		return fmt.Sprintf("unknown status (%d)", int(fs))
	}
}

// MediaFile represents a single media file and its synchronization status.
type MediaFile struct {
	Directory string
	Filename  string
	CreatedAt time.Time
	Size      int64
	Status    FileStatus
	Error     error
}

// MediaInventory holds the results of comparing GoPro and local files.
type MediaInventory struct {
	Files []MediaFile
}

// NewMediaInventory creates a MediaInventory by comparing GoPro and local files.
func NewMediaInventory(ctx context.Context, goproClient *gopro.Client, outputDir string) (*MediaInventory, error) {
	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return nil, fmt.Errorf("getting absolute path for output directory: %w", err)
	}

	mediaList, err := goproClient.GetMediaList(ctx)
	if err != nil {
		return nil, err
	}

	localFiles, err := getLocalFiles(absOutputDir)
	if err != nil {
		return nil, err
	}

	inventory := &MediaInventory{}

	// Add files from GoPro.
	for _, media := range mediaList.Media {
		for _, file := range media.Items {
			localFileInfo, localFileExists := localFiles[file.Filename]

			status := StatusOnlyGoPro // Default status.
			var createdAt time.Time

			if localFileExists {
				createdAt = localFileInfo.ModTime() // Use mtime from local file.
				if localFileInfo.Size() == file.Size {
					status = StatusSynced
				} else {
					status = StatusDifferent
				}
			} else {
				createdAt = file.CreatedAt // fallback to GoPro's timestamp.
			}

			mediaFile :=MediaFile{
				Directory: media.Directory,
				Filename:  file.Filename,
				CreatedAt: createdAt, // Use determined createdAt.
				Size:      file.Size,
				Status:    status,
			}
			inventory.Files = append(inventory.Files, mediaFile)
			delete(localFiles, file.Filename) // Remove from the map.
		}
	}

	// Add any remaining local files (files that exist only locally).
	for localFileName, localFileInfo := range localFiles {
		mediaFile := MediaFile{
			Directory: absOutputDir, // Assume no subdirectory.
			Filename:  localFileName,
			CreatedAt: localFileInfo.ModTime(), // Use mtime from local file.
			Size:      localFileInfo.Size(),
			Status:    StatusOnlyLocal,
		}
		inventory.Files = append(inventory.Files, mediaFile)
	}

	return inventory, nil
}

// getLocalFiles builds a map of local files (filename -> os.FileInfo).
func getLocalFiles(dir string) (map[string]os.FileInfo, error) {
	localFiles := make(map[string]os.FileInfo)
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return fmt.Errorf("finding relative path of, %v, to output path, %v: %w",path, dir, err)
			}
			localFiles[rel] = info
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking local directory %s: %w", dir, err)
	}
	return localFiles, nil
}
