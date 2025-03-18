package media

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/EarthmanMuons/herosync/internal/gopro"
)

// Status represents the synchronization status of a file.
type Status int

const (
	OnlyRemote Status = iota // File exists only on the GoPro
	OnlyLocal                // File exists only locally
	InSync                   // File exists on both, with matching sizes
	OutOfSync                // File exists on both, but sizes differ
	Processed                // File is ready for uploading to YouTube
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
		return "saved on all devices"
	case OutOfSync:
		return "SIZES ARE MISMATCHED"
	case Processed:
		return "ready for publishing"
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
	case Processed:
		return "^"
	default:
		return fmt.Sprintf("unknown status (%d)", int(s))
	}
}

// File represents a single media file and its synchronization status.
type File struct {
	Directory   string
	Filename    string
	CreatedAt   time.Time
	Size        int64
	Status      Status
	DisplayInfo string
}

// Inventory holds the results of comparing remote and local files.
type Inventory struct {
	Files []File
}

// NewInventory creates an Inventory by comparing remote and local files.
func NewInventory(ctx context.Context, client *gopro.Client, incomingDir, outgoingDir string) (*Inventory, error) {
	mediaList, err := client.GetMediaList(ctx)
	if err != nil {
		return nil, err
	}

	incomingFiles, err := scanLocalFiles(incomingDir)
	if err != nil {
		return nil, err
	}

	outgoingFiles, err := scanLocalFiles(outgoingDir)
	if err != nil {
		return nil, err
	}

	inventory := &Inventory{}
	processRemoteFiles(mediaList, incomingFiles, incomingDir, inventory)
	processIncomingFiles(incomingFiles, incomingDir, inventory)
	processOutgoingFiles(outgoingFiles, outgoingDir, inventory)

	sort.Slice(inventory.Files, func(i, j int) bool {
		return inventory.Files[i].CreatedAt.Before(inventory.Files[j].CreatedAt)
	})

	return inventory, nil
}

// scanLocalFiles builds a map of local files (filename -> os.FileInfo).
func scanLocalFiles(dir string) (map[string]os.FileInfo, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("getting absolute path for directory: %w", err)
	}

	files := make(map[string]os.FileInfo)

	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, fmt.Errorf("reading directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}

		filePath := filepath.Join(absDir, entry.Name())
		info, err := os.Stat(filePath)
		if err != nil {
			return nil, fmt.Errorf("stat file: %w", err)
		}

		files[entry.Name()] = info
	}

	return files, nil
}

// processRemoteFiles adds files from GoPro and updates their status if found locally in incoming directory.
func processRemoteFiles(mediaList *gopro.MediaList, incomingFiles map[string]os.FileInfo, incomingDir string, inventory *Inventory) {
	for _, media := range mediaList.Media {
		for _, file := range media.Items {
			localFileInfo, localFileExists := incomingFiles[file.Filename]

			status := OnlyRemote
			if localFileExists {
				if localFileInfo.Size() == file.Size {
					status = InSync
				} else {
					status = OutOfSync
				}
				delete(incomingFiles, file.Filename)
			}

			mediaFile := File{
				Directory: media.Directory,
				Filename:  file.Filename,
				CreatedAt: file.CreatedAt,
				Size:      file.Size,
				Status:    status,
			}
			mediaFile.DisplayInfo = generateDisplayInfo(mediaFile)

			inventory.Files = append(inventory.Files, mediaFile)
		}
	}
}

// processIncomingFiles handles local files that were not found on the GoPro (incoming media).
func processIncomingFiles(incomingFiles map[string]os.FileInfo, incomingDir string, inventory *Inventory) {
	for filename, fileInfo := range incomingFiles {
		mediaFile := File{
			Directory: incomingDir,
			Filename:  filename,
			CreatedAt: fileInfo.ModTime(),
			Size:      fileInfo.Size(),
			Status:    OnlyLocal,
		}
		mediaFile.DisplayInfo = generateDisplayInfo(mediaFile)

		inventory.Files = append(inventory.Files, mediaFile)
	}
}

// processOutgoingFiles handles local files that are in the outgoing directory (ready for upload).
func processOutgoingFiles(outgoingFiles map[string]os.FileInfo, outgoingDir string, inventory *Inventory) {
	for filename, fileInfo := range outgoingFiles {
		mediaFile := File{
			Directory: outgoingDir,
			Filename:  filename,
			CreatedAt: fileInfo.ModTime(),
			Size:      fileInfo.Size(),
			Status:    Processed,
		}
		mediaFile.DisplayInfo = generateDisplayInfo(mediaFile)

		inventory.Files = append(inventory.Files, mediaFile)
	}
}

// generateDisplayInfo precomputes the file's full display string for easy filtering.
// func generateDisplayInfo(directory, filename string, createdAt time.Time, size int64, status Status) string {
func generateDisplayInfo(file File) string {
	displayDir := file.Directory
	if file.Status == OnlyRemote {
		displayDir = " pending"
	} else if file.Status == InSync {
		displayDir = "incoming"
	}

	return fmt.Sprintf("%s  %s %8s  %20s  %s / %s",
		file.Status.Symbol(),
		file.Status.String(),
		humanize.Bytes(uint64(file.Size)),
		file.CreatedAt.Format(time.DateTime),
		path.Base(displayDir),
		file.Filename,
	)
}

// String() returns the precomputed display information.
func (f File) String() string {
	return f.DisplayInfo
}

// FilterByDate returns a new Inventory containing only files created on the specified date.
func (inv *Inventory) FilterByDate(date time.Time) (*Inventory, error) {
	filtered := &Inventory{}

	for _, file := range inv.Files {
		// Skip already processed (outgoing) files.
		if file.Status == Processed {
			continue
		}

		// Compare year, month, and day.
		y1, m1, d1 := file.CreatedAt.Date()
		y2, m2, d2 := date.Date()
		if y1 == y2 && m1 == m2 && d1 == d2 {
			filtered.Files = append(filtered.Files, file)
		}
	}

	if len(filtered.Files) == 0 {
		return nil, fmt.Errorf("no matching files found for date: %s", date.Format(time.DateOnly))
	}

	return filtered, nil
}

// FilterByDisplayInfo returns a new Inventory containing only files whose
// display info matches (case-insensitive, partial match) any of the strings in
// the provided keywords slice.
func (inv *Inventory) FilterByDisplayInfo(keywords []string) (*Inventory, error) {
	if len(keywords) == 0 {
		return inv, nil // no filtering needed
	}

	filtered := &Inventory{}
	for _, file := range inv.Files {
		haystack := strings.ToLower(file.DisplayInfo)

		for _, needle := range keywords {
			if strings.Contains(haystack, strings.ToLower(needle)) {
				filtered.Files = append(filtered.Files, file)
				break // avoid adding the same file multiple times
			}
		}
	}

	if len(filtered.Files) == 0 {
		return nil, fmt.Errorf("no matching files found for: %v", keywords)
	}

	return filtered, nil
}

// FilterByMediaID returns a new Inventory containing only files with the specified Media ID.
func (inv *Inventory) FilterByMediaID(mediaID int) (*Inventory, error) {
	filtered := &Inventory{}
	var chapters []File

	for _, file := range inv.Files {
		// Skip already processed (outgoing) files.
		if file.Status == Processed {
			continue
		}

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

	if len(filtered.Files) == 0 {
		return nil, fmt.Errorf("no matching files found for Media ID: %d", mediaID)
	}

	return filtered, nil
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
		// Skip already processed (outgoing) files.
		if file.Status == Processed {
			continue
		}

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
		// Skip already processed (outgoing) files.
		if file.Status == Processed {
			continue
		}

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
