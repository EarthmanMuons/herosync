package fsutil

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GenerateUniqueFilename generates a unique filename based on the provided base path.
// If the file already exists, it appends a counter (e.g., "_1", "_2") before the extension.
func GenerateUniqueFilename(basePath string) (string, error) {
	dir := filepath.Dir(basePath)
	ext := filepath.Ext(basePath)
	name := strings.TrimSuffix(filepath.Base(basePath), ext)

	newPath := basePath
	counter := 1

	for {
		_, err := os.Stat(newPath)
		if err != nil {
			if os.IsNotExist(err) {
				return newPath, nil // File doesn't exist, we're good.
			}
			return "", fmt.Errorf("stat file %s: %w", newPath, err)
		}

		newPath = filepath.Join(dir, fmt.Sprintf("%s_%d%s", name, counter, ext))
		counter++
	}
}

// SetMtime sets the modification time (mtime) of the file at the given path.
func SetMtime(logger *slog.Logger, path string, mtime time.Time) error {
	if err := os.Chtimes(path, time.Now(), mtime); err != nil {
		return fmt.Errorf("set mtime on %s: %w", path, err)
	}
	logger.Debug("mtime updated", slog.String("path", path), slog.Time("timestamp", mtime))
	return nil
}

// ShortenPath replaces the home directory path with ~
func ShortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return strings.Replace(path, home, "~", 1)
}

// VerifySize checks if the file at the given path has a size within an
// acceptable range of the expected size. The tolerance is expressed as a
// percentage (e.g., 0.01 for 1%). A tolerance of 0.0 enforces an exact match.
func VerifySize(path string, expectedSize int64, tolerance float64) error {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	actualSize := fileInfo.Size()

	if tolerance == 0.0 {
		if actualSize != expectedSize {
			return fmt.Errorf("file size mismatch: got %d, expected %d", actualSize, expectedSize)
		}
		return nil
	}

	min := float64(expectedSize) * (1 - tolerance)
	max := float64(expectedSize) * (1 + tolerance)

	if float64(actualSize) < min || float64(actualSize) > max {
		base := filepath.Base(path)
		return fmt.Errorf("file size for %s out of tolerance: got %d, expected [%.2f, %.2f]", base, actualSize, min, max)
	}

	return nil
}

// VerifySizeExact is a convenience function for exact size verification.
func VerifySizeExact(path string, expectedSize int64) error {
	return VerifySize(path, expectedSize, 0.0)
}
