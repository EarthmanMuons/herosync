package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
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

			if localFileExists {
				if localFileInfo.Size() == file.Size {
					status = StatusSynced
				} else {
					status = StatusDifferent
				}
			}

			mediaFile := MediaFile{
				Directory: media.Directory,
				Filename:  file.Filename,
				CreatedAt: file.CreatedAt,
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

	// Sort the inventory by file creation time.
	sort.Slice(inventory.Files, func(i, j int) bool {
		return inventory.Files[i].CreatedAt.Before(inventory.Files[j].CreatedAt)
	})

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
				return fmt.Errorf("finding relative path of, %v, to output path, %v: %w", path, dir, err)
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

// AllFilesLocal checks if all files in the inventory are either StatusSynced or StatusOnlyLocal.
func (mi *MediaInventory) AllFilesLocal() bool {
	for _, file := range mi.Files {
		if file.Status != StatusSynced && file.Status != StatusOnlyLocal {
			return false
		}
	}
	return true
}

// FilterByDate returns a new MediaInventory containing only files created on the specified date.
func (mi *MediaInventory) FilterByDate(date time.Time) *MediaInventory {
	filtered := &MediaInventory{}

	for _, file := range mi.Files {
		// Compare year, month, and day.
		y1, m1, d1 := file.CreatedAt.Date()
		y2, m2, d2 := date.Date()
		if y1 == y2 && m1 == m2 && d1 == d2 {
			filtered.Files = append(filtered.Files, file)
		}
	}
	return filtered
}

// FilterByFilename returns a new MediaInventory containing only files whose
// filenames match (case-insensitive, partial match) any of the strings in
// the provided filenames slice.
func (mi *MediaInventory) FilterByFilename(filenames []string) *MediaInventory {
	filtered := &MediaInventory{}

	if len(filenames) == 0 {
		return mi
	}

	for _, file := range mi.Files {
		for _, filter := range filenames {
			if strings.Contains(strings.ToLower(file.Filename), strings.ToLower(filter)) {
				filtered.Files = append(filtered.Files, file)
				break // Avoid adding the same file multiple times.
			}
		}
	}

	return filtered
}

// FilterByMediaID returns a new MediaInventory containing only files with the specified Media ID.
func (mi *MediaInventory) FilterByMediaID(mediaID int) *MediaInventory {
	filtered := &MediaInventory{}
	var chapters []MediaFile

	for _, file := range mi.Files {
		fileInfo := gopro.ParseFilename(file.Filename)
		if fileInfo.IsValid && fileInfo.MediaID == mediaID {
			chapters = append(chapters, file)
		}
	}

	// Sort by chapter to ensure correct concatenation order.
	slices.SortFunc(chapters, func(a, b MediaFile) int {
		fileInfoA := gopro.ParseFilename(a.Filename)
		fileInfoB := gopro.ParseFilename(b.Filename)
		return fileInfoA.Chapter - fileInfoB.Chapter // Compare the integer value of both chapters.
	})
	filtered.Files = append(filtered.Files, chapters...)

	return filtered
}

// FilterByStatus returns a new MediaInventory containing only files that have
// one of the specified statuses.
func (mi *MediaInventory) FilterByStatus(statuses ...FileStatus) *MediaInventory {
	filtered := &MediaInventory{}

	if len(statuses) == 0 {
		return mi // Return the original if no statuses are provided
	}

	for _, file := range mi.Files {
		if slices.Contains(statuses, file.Status) {
			filtered.Files = append(filtered.Files, file)
		}
	}

	return filtered
}

// GetUniqueDates returns a sorted list of unique dates from the MediaInventory.
func (mi *MediaInventory) GetUniqueDates() []time.Time {
	dateMap := make(map[time.Time]bool)
	var dates []time.Time

	for _, file := range mi.Files {
		date := time.Date(file.CreatedAt.Year(), file.CreatedAt.Month(), file.CreatedAt.Day(), 0, 0, 0, 0, file.CreatedAt.Location())
		if _, ok := dateMap[date]; !ok {
			dateMap[date] = true
			dates = append(dates, date)
		}
	}
	slices.SortFunc(dates, func(a, b time.Time) int {
		return a.Compare(b)
	})
	return dates
}

// GetMediaIds returns a sorted list of unique Media IDs from the MediaInventory.
func (mi *MediaInventory) GetMediaIDs() []int {
	keys := make(map[int]bool)
	var ids []int

	for _, file := range mi.Files {
		fileInfo := gopro.ParseFilename(file.Filename)
		if _, ok := keys[fileInfo.MediaID]; !ok {
			keys[fileInfo.MediaID] = true
			ids = append(ids, fileInfo.MediaID)
		}
	}
	slices.Sort(ids)
	return ids
}
