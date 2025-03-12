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

// Status represents the synchronization status of a file.
type Status int

const (
	OnlyRemote Status = iota // File exists only on the GoPro
	OnlyLocal                // File exists only locally
	InSync                   // File exists on both, with matching sizes
	OutOfSync                // File exists on both, but sizes differ
	StatError                // Represents stat error
)

// String provides a human-readable representation of the Status.
func (s Status) String() string {
	switch s {
	case OnlyRemote:
		return "only stored on gopro"
	case OnlyLocal:
		return "only stored on local"
	case InSync:
		return "file already in sync"
	case OutOfSync:
		return "SIZES ARE MISMATCHED"
	default:
		return fmt.Sprintf("unknown status (%d)", int(s))
	}
}

// Symbol provides a symbolic representation of the FileStatus.
func (s Status) Symbol() string {
	switch s {
	case OnlyRemote:
		return "«"
	case OnlyLocal:
		return "»"
	case InSync:
		return "="
	case OutOfSync:
		return "!"
	default:
		return fmt.Sprintf("unknown status (%d)", int(s))
	}
}

// File represents a single media file and its synchronization status.
type File struct {
	Directory string
	Filename  string
	CreatedAt time.Time
	Size      int64
	Status    Status
	Error     error
}

// Inventory holds the results of comparing remote and local files.
type Inventory struct {
	Files []File
}

// NewInventory creates an Inventory by comparing remote and local files.
func NewInventory(ctx context.Context, client *gopro.Client, outputDir string) (*Inventory, error) {
	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return nil, fmt.Errorf("getting absolute path for output directory: %w", err)
	}

	mediaList, err := client.GetMediaList(ctx)
	if err != nil {
		return nil, err
	}

	localFiles, err := scanLocalFiles(absOutputDir)
	if err != nil {
		return nil, err
	}

	inventory := &Inventory{}

	// Add files from GoPro.
	for _, media := range mediaList.Media {
		for _, file := range media.Items {
			localFileInfo, localFileExists := localFiles[file.Filename]

			status := OnlyRemote // Default status.

			if localFileExists {
				if localFileInfo.Size() == file.Size {
					status = InSync
				} else {
					status = OutOfSync
				}
			}

			mediaFile := File{
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
		mediaFile := File{
			Directory: absOutputDir, // Assume no subdirectory.
			Filename:  localFileName,
			CreatedAt: localFileInfo.ModTime(), // Use mtime from local file.
			Size:      localFileInfo.Size(),
			Status:    OnlyLocal,
		}
		inventory.Files = append(inventory.Files, mediaFile)
	}

	// Sort the inventory by file creation time.
	sort.Slice(inventory.Files, func(i, j int) bool {
		return inventory.Files[i].CreatedAt.Before(inventory.Files[j].CreatedAt)
	})

	return inventory, nil
}

// scanLocalFiles builds a map of local files (filename -> os.FileInfo).
func scanLocalFiles(dir string) (map[string]os.FileInfo, error) {
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

// FilterByDate returns a new Inventory containing only files created on the specified date.
func (inv *Inventory) FilterByDate(date time.Time) *Inventory {
	filtered := &Inventory{}

	for _, file := range inv.Files {
		// Compare year, month, and day.
		y1, m1, d1 := file.CreatedAt.Date()
		y2, m2, d2 := date.Date()
		if y1 == y2 && m1 == m2 && d1 == d2 {
			filtered.Files = append(filtered.Files, file)
		}
	}
	return filtered
}

// FilterByFilename returns a new Inventory containing only files whose
// filenames match (case-insensitive, partial match) any of the strings in the
// provided filenames slice.
func (inv *Inventory) FilterByFilename(filenames []string) (*Inventory, error) {
	if len(filenames) == 0 {
		return inv, nil // no filtering needed
	}

	filtered := &Inventory{}
	for _, file := range inv.Files {
		for _, filter := range filenames {
			if strings.Contains(strings.ToLower(file.Filename), strings.ToLower(filter)) {
				filtered.Files = append(filtered.Files, file)
				break // avoid adding the same file multiple times
			}
		}
	}

	if len(filtered.Files) == 0 {
		return nil, fmt.Errorf("no matching files found for: %v", filenames)
	}

	return filtered, nil
}

// FilterByMediaID returns a new Inventory containing only files with the specified Media ID.
func (inv *Inventory) FilterByMediaID(mediaID int) *Inventory {
	filtered := &Inventory{}
	var chapters []File

	for _, file := range inv.Files {
		fileInfo := gopro.ParseFilename(file.Filename)
		if fileInfo.IsValid && fileInfo.MediaID == mediaID {
			chapters = append(chapters, file)
		}
	}

	// Sort by chapter to ensure correct concatenation order.
	slices.SortFunc(chapters, func(a, b File) int {
		fileInfoA := gopro.ParseFilename(a.Filename)
		fileInfoB := gopro.ParseFilename(b.Filename)
		return fileInfoA.Chapter - fileInfoB.Chapter // Compare the integer value of both chapters.
	})
	filtered.Files = append(filtered.Files, chapters...)

	return filtered
}

// FilterByStatus returns a new Inventory containing only files that have one of
// the specified statuses.
func (inv *Inventory) FilterByStatus(statuses ...Status) *Inventory {
	filtered := &Inventory{}

	if len(statuses) == 0 {
		return inv // Return the original if no statuses are provided
	}

	for _, file := range inv.Files {
		if slices.Contains(statuses, file.Status) {
			filtered.Files = append(filtered.Files, file)
		}
	}

	return filtered
}

// TotalSize returns the sum total size (in bytes) of all of the files in the Inventory.
func (inv *Inventory) TotalSize() int64 {
	var totalSize int64
	for _, file := range inv.Files {
		totalSize += file.Size
	}
	return totalSize
}

// HasUnsyncedFiles checks if the inventory has files that need downloading.
func (inv *Inventory) HasUnsyncedFiles() bool {
	for _, file := range inv.Files {
		if file.Status != InSync && file.Status != OnlyLocal {
			return true
		}
	}
	return false
}

// MediaIds returns a sorted list of unique Media IDs from the Inventory.
func (inv *Inventory) MediaIDs() []int {
	keys := make(map[int]bool)
	var ids []int

	for _, file := range inv.Files {
		fileInfo := gopro.ParseFilename(file.Filename)
		if _, ok := keys[fileInfo.MediaID]; !ok {
			keys[fileInfo.MediaID] = true
			ids = append(ids, fileInfo.MediaID)
		}
	}
	slices.Sort(ids)
	return ids
}

// UniqueDates returns a sorted list of unique dates from the Inventory.
func (inv *Inventory) UniqueDates() []time.Time {
	dateMap := make(map[time.Time]bool)
	var dates []time.Time

	for _, file := range inv.Files {
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
