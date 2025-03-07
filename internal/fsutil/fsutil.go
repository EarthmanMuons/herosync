package fsutil

import (
	"errors"
	"io/fs"
	"os"
	"strings"
)

// FileExistsAndMatchesSize checks if a file exists and has the expected size.
func FileExistsAndMatchesSize(filePath string, expectedSize uint64) bool {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false // File doesn't exist
		}
		// For now, assume any other stat error means no match.
		return false
	}

	return uint64(fileInfo.Size()) == expectedSize
}

// ShortenPath replaces the home directory path with ~
func ShortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return strings.Replace(path, home, "~", 1)
}
