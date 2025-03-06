package gopro

// https://gopro.github.io/OpenGoPro/http#tag/Models
//
// Note: This implementation intentionally parses only a subset of available API fields.
// For the complete API specification, see the OpenGoPro documentation linked above.

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
	CreatedAt Timestamp `json:"cre"` // Creation time in seconds since epoch
	Size      String64  `json:"s"`   // Size of media in bytes
}

type cameraDateTime struct {
	Date     string `json:"date"`  // Format: YYYY_MM_DD
	Time     string `json:"time"`  // Format: HH_MM_SS
	DST      int    `json:"dst"`   // 1 if DST is active, 0 if not
	TZOffset int    `json:"tzone"` // Timezone offset in minutes
}

// Timestamp is a Unix timestamp that can be unmarshaled from either a string or number.
type Timestamp int64

// Time converts the Unix timestamp to time.Time.
func (t Timestamp) Time() time.Time {
	return time.Unix(int64(t), 0)
}

// String returns the timestamp as a formatted string.
func (t Timestamp) String() string {
	return t.Time().String()
}

func (t Timestamp) MarshalJSON() ([]byte, error) {
	return fmt.Appendf(nil, "%d", t), nil
}

func (t *Timestamp) UnmarshalJSON(data []byte) error {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	switch v := raw.(type) {
	case float64:
		*t = Timestamp(v)
	case string:
		i, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return err
		}
		*t = Timestamp(i)
	default:
		return fmt.Errorf("unexpected type for Timestamp: %T", v)
	}

	return nil
}

// String64 is an int64 that can be unmarshaled from either a string or number.
type String64 int64

func (s String64) MarshalJSON() ([]byte, error) {
	return fmt.Appendf(nil, "%d", s), nil
}

func (s *String64) UnmarshalJSON(data []byte) error {
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	switch v := raw.(type) {
	case float64:
		*s = String64(v)
	case string:
		i, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return err
		}
		*s = String64(i)
	default:
		return fmt.Errorf("unexpected type for String64: %T", v)
	}

	return nil
}
