package gopro

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

type Client struct {
	httpClient *retryablehttp.Client
	baseURL    *url.URL
	logger     *slog.Logger
}

// progressWriter wraps an io.Reader to report download progress periodically.
type progressWriter struct {
	reader       io.Reader
	totalSize    int64
	written      int64
	logger       *slog.Logger
	interval     time.Duration
	lastUpdate   time.Time
	fileName     string
	bytesWritten int64
}

func NewClient(baseURL *url.URL, logger *slog.Logger) *Client {
	client := retryablehttp.NewClient()
	client.Logger = logger

	return &Client{
		httpClient: client,
		baseURL:    baseURL,
		logger:     logger,
	}
}

func (c *Client) GetHardwareInfo(ctx context.Context) (*HardwareInfo, error) {
	resp, err := c.get(ctx, "/gopro/camera/info")
	if err != nil {
		return nil, fmt.Errorf("getting hardware info: %w", err)
	}
	defer resp.Body.Close()

	var hwInfo HardwareInfo
	if err := json.NewDecoder(resp.Body).Decode(&hwInfo); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &hwInfo, nil
}

func (c *Client) GetMediaList(ctx context.Context) (*MediaList, error) {
	resp, err := c.get(ctx, "/gopro/media/list")
	if err != nil {
		return nil, fmt.Errorf("listing media: %w", err)
	}
	defer resp.Body.Close()

	var mediaList MediaList
	if err := json.NewDecoder(resp.Body).Decode(&mediaList); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if len(mediaList.Media) != 0 {
		// Get camera's timezone offset and adjust the media timestamps.
		tzOffset, err := c.getTimezoneOffset(ctx)
		if err != nil {
			return nil, fmt.Errorf("getting timezone offset: %w", err)
		}

		if err := adjustTimestamps(&mediaList, tzOffset); err != nil {
			return nil, fmt.Errorf("adjusting timestamps: %w", err)
		}
	}

	return &mediaList, nil
}

func (c *Client) DownloadMediaFile(ctx context.Context, directory string, filename string, outputDir string) error {
	reqURL := fmt.Sprintf("/videos/DCIM/%s/%s", directory, filename)

	resp, err := c.get(ctx, reqURL)
	if err != nil {
		return fmt.Errorf("downloading media file: %w", err)
	}
	defer resp.Body.Close()

	// Convert outputDir to an absolute path.
	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("getting absolute path for output directory: %w", err)
	}

	fullLocalPath := filepath.Join(absOutputDir, filename)
	if err := os.MkdirAll(filepath.Dir(fullLocalPath), 0o750); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	out, err := os.Create(fullLocalPath)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer out.Close()

	totalSize := resp.ContentLength
	if totalSize <= 0 {
		c.logger.Warn("Content-Length header not found or invalid, progress won't show total size.")
	}

	progressReader := &progressWriter{
		reader:     resp.Body,
		totalSize:  totalSize,
		logger:     c.logger,
		interval:   5 * time.Second,
		lastUpdate: time.Now(),
		fileName:   filename,
	}

	_, err = io.Copy(out, progressReader)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("writing to file: %w", err)
	}

	return nil
}

func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	reqURL := c.baseURL.JoinPath(path)

	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("doing request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return resp, nil
}

func (c *Client) getTimezoneOffset(ctx context.Context) (int, error) {
	resp, err := c.get(ctx, "/gopro/camera/get_date_time")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var dt cameraDateTime
	if err := json.NewDecoder(resp.Body).Decode(&dt); err != nil {
		return 0, fmt.Errorf("decoding response: %w", err)
	}

	return dt.TZOffset, nil
}

func (pw *progressWriter) Read(p []byte) (int, error) {
	n, err := pw.reader.Read(p)
	pw.written += int64(n)
	pw.bytesWritten += int64(n)

	now := time.Now()
	if now.Sub(pw.lastUpdate) >= pw.interval {
		if pw.totalSize > 0 {
			percent := float64(pw.written) / float64(pw.totalSize) * 100
			pw.logger.Info("download progress", "filename", pw.fileName, "progress", fmt.Sprintf("%.2f%%", percent), "written", pw.written, "total", pw.totalSize)
		} else {
			pw.logger.Info("download progress", "filename", pw.fileName, "written", pw.written)
		}
		pw.lastUpdate = now
	}

	return n, err
}

// adjustTimestamps converts camera-local timestamps to UTC.
func adjustTimestamps(mediaList *MediaList, tzOffset int) error {
	for _, media := range mediaList.Media {
		for file := range media.Items {
			originalTime := media.Items[file].CreatedAt
			adjustedTime := originalTime.Add(-time.Duration(tzOffset) * time.Minute)

			media.Items[file].CreatedAt = adjustedTime
		}
	}
	return nil
}
