package gopro

import (
	"regexp"
	"strconv"
	"strings"
)

// FilenameInfo holds the parsed information from a GoPro filename.
type FilenameInfo struct {
	Quality  string // Quality level (X, H, M, L)
	Chapter  int    // Chapter number (01-99)
	MediaID  int    // Media ID (0001-9999)
	FileType string // File extension (MP4, THM, LRV)
	IsValid  bool   // Flag to indicate if parsing was successful
	Filename string // stores the filenaem to reference later if necessary
}

// parseGoProFilename parses a GoPro filename and returns a GoProFileInfo struct.
// Upstream docs: https://gopro.github.io/OpenGoPro/http#tag/Media/Chapters
func ParseFilename(filename string) FilenameInfo {
	// Regular expression to match GoPro filename format: Gqccmmmm.ext
	//   - quality: The quality character (e.g., H, X, M, L).
	//   - chapter: Two-digit chapter number (e.g., 01, 22).
	//   - mediaID: Four-digit media ID (e.g., 0078, 1234).
	//   - ext: the file extension (e.g. MP4).
	re := regexp.MustCompile(`^G(?P<quality>[XHLM])(?P<chapter>\d{2})(?P<mediaID>\d{4})\.(?P<ext>MP4|THM|LRV)$`)

	match := re.FindStringSubmatch(filename)
	if len(match) == 0 {
		return FilenameInfo{IsValid: false, Filename: filename} // Return invalid if no match.
	}

	result := make(map[string]string)
	for i, name := range re.SubexpNames() {
		if i != 0 && name != "" {
			result[name] = match[i]
		}
	}

	chapter, _ := strconv.Atoi(result["chapter"]) // Convert chapter string to int.
	mediaID, _ := strconv.Atoi(result["mediaID"]) // Convert mediaID string to int.
	fileType := strings.ToUpper(result["ext"])    // Convert extension for consistency.

	return FilenameInfo{
		Quality:  result["quality"],
		Chapter:  chapter,
		MediaID:  mediaID,
		FileType: fileType,
		IsValid:  true,
		Filename: filename,
	}
}
