package gopro

// https://gopro.github.io/OpenGoPro/http#tag/Models
//
// Note: This implementation intentionally parses only a subset of available API fields.
// For the complete API specification, see the Open GoPro documentation linked above.

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// HardwareInfo represents the response from the hardware info API.
type HardwareInfo struct {
	ModelName       string `json:"model_name"`
	SerialNumber    string `json:"serial_number"`
	FirmwareVersion string `json:"firmware_version"`
}

// CameraState represents the top-level response from the camera state API.
type CameraState struct {
	Status cameraStateStatus `json:"status"`
}

type cameraStateStatus struct {
	SDCardCapacity  int64 `json:"117"`
	SDCardRemaining int64 `json:"54"`
}

// MediaList represents the top-level response from the media list API.
type MediaList struct {
	ID    string       `json:"id"`    // Media list identifier
	Media []MediaFiles `json:"media"` // Array of media directories
}

// MediaFiles represents a directory of media items.
type MediaFiles struct {
	Directory string          `json:"d"`  // Directory name (e.g., "100GOPRO")
	Items     []MediaListItem `json:"fs"` // List of files in the directory
}

// MediaListItem represents a single media file and its metadata.
type MediaListItem struct {
	Filename  string    `json:"n"`   // Media filename
	CreatedAt time.Time `json:"cre"` // Creation time in seconds since epoch
	Size      int64     `json:"s"`   // Size of media in bytes
}

type cameraDateTime struct {
	Date     string `json:"date"`  // Format: YYYY_MM_DD
	Time     string `json:"time"`  // Format: HH_MM_SS
	DST      int    `json:"dst"`   // 1 if DST is active, 0 if not
	TZOffset int    `json:"tzone"` // Timezone offset in minutes
}

func (m *MediaListItem) UnmarshalJSON(data []byte) error {
	type Alias MediaListItem
	aux := &struct {
		CreatedAt string `json:"cre"`
		Size      string `json:"s"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Convert the API's string value into a time.Time for storage.
	seconds, err := strconv.ParseInt(aux.CreatedAt, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid creation timestamp '%s': %w", aux.CreatedAt, err)
	}
	m.CreatedAt = time.Unix(seconds, 0) // Parse as UTC!

	// Convert the API's string value into an int64 for storage.
	parsedSize, err := strconv.ParseInt(aux.Size, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid size string '%s': %w", aux.Size, err)
	}
	m.Size = parsedSize

	return nil
}

func (c *cameraStateStatus) UnmarshalJSON(data []byte) error {
	type Alias cameraStateStatus
	aux := &struct {
		CapacityKB  int64 `json:"117"`
		RemainingKB int64 `json:"54"`
		*Alias
	}{
		Alias: (*Alias)(c),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Convert the API's kilobyte values into bytes for storage.
	const bytesPerKB = 1000
	c.SDCardCapacity = aux.CapacityKB * bytesPerKB
	c.SDCardRemaining = aux.RemainingKB * bytesPerKB

	return nil
}
