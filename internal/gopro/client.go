// Package gopro provides a client for interacting with GoPro cameras.
//
// This client implements the Open GoPro API as documented at:
// https://gopro.github.io/OpenGoPro/http
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

// NewClientDefault initializes a GoPro API client with the standard GoPro IP.
func NewClientDefault(logger *slog.Logger) (*Client, error) {
	return NewClient(logger, "http", "10.5.5.9:8080")
}

// NewClient initializes a GoPro API client, resolving the address if necessary.
func NewClient(logger *slog.Logger, scheme, host string) (*Client, error) {
	baseURL, err := resolveGoPro(host, scheme)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GoPro client: %w", err)
	}

	client := retryablehttp.NewClient()
	client.Logger = logger

	return &Client{
		httpClient: client,
		baseURL:    baseURL,
		logger:     logger,
	}, nil
}

// BaseURL returns the GoPro's resolved base URL.
func (c *Client) BaseURL() string {
	return c.baseURL.String()
}

// get creates and performs a GET request, handling request creation, retries,
// and error handling. It takes the FULL URL as a string.
func (c *Client) get(ctx context.Context, fullURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Wrap the *http.Request with retryablehttp.
	retryableReq, err := retryablehttp.FromRequest(req)
	if err != nil {
		return nil, fmt.Errorf("creating retryable request: %w", err)
	}

	return c.httpClient.Do(retryableReq)
}

// Upstream API: https://gopro.github.io/OpenGoPro/http#tag/Control/operation/OGP_TURBO_MODE_ENABLE
func (c *Client) ConfigureTurboTransfer(ctx context.Context, enable bool) error {
	// Convert boolean to the appropriate query parameter (0 for disable, 1 for enable).
	param := 0
	if enable {
		param = 1
	}

	// Create this manually as a string to prevent URL encoding.
	fullURL := fmt.Sprintf("%s/gopro/media/turbo_transfer?p=%d", c.baseURL, param)

	resp, err := c.get(ctx, fullURL)
	if err != nil {
		return fmt.Errorf("configuring turbo transfer mode: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("configuring turbo transfer mode: unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Upstream API: https://gopro.github.io/OpenGoPro/http#tag/Query/operation/OGP_GET_STATE
func (c *Client) GetCameraState(ctx context.Context) (*CameraState, error) {
	reqURL := c.baseURL.JoinPath("/gopro/camera/state").String()

	resp, err := c.get(ctx, reqURL)
	if err != nil {
		return nil, fmt.Errorf("getting camera state: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("getting camera state: unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var cameraState CameraState
	if err := json.NewDecoder(resp.Body).Decode(&cameraState); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &cameraState, nil
}

// Upstream API: https://gopro.github.io/OpenGoPro/http#tag/Query/operation/OGP_CAMERA_INFO
func (c *Client) GetHardwareInfo(ctx context.Context) (*HardwareInfo, error) {
	reqURL := c.baseURL.JoinPath("/gopro/camera/info").String()

	resp, err := c.get(ctx, reqURL)
	if err != nil {
		return nil, fmt.Errorf("getting hardware info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("getting hardware info: unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var hwInfo HardwareInfo
	if err := json.NewDecoder(resp.Body).Decode(&hwInfo); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &hwInfo, nil
}

// Upstream API: https://gopro.github.io/OpenGoPro/http#tag/Media/operation/OGP_MEDIA_LIST
func (c *Client) GetMediaList(ctx context.Context) (*MediaList, error) {
	reqURL := c.baseURL.JoinPath("/gopro/media/list").String()

	resp, err := c.get(ctx, reqURL)
	if err != nil {
		return nil, fmt.Errorf("getting media list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("getting media list: unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

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

// Upstream API: https://gopro.github.io/OpenGoPro/http#tag/Media/operation/OGP_DOWNLOAD_MEDIA
func (c *Client) DownloadMediaFile(ctx context.Context, directory string, filename string, downloadDir string) error {
	relPath := fmt.Sprintf("/videos/DCIM/%s/%s", directory, filename)
	reqURL := c.baseURL.JoinPath(relPath).String()

	resp, err := c.get(ctx, reqURL)
	if err != nil {
		return fmt.Errorf("downloading media file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("downloading media file: unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	absDownloadDir, err := filepath.Abs(downloadDir)
	if err != nil {
		return fmt.Errorf("getting absolute path for download directory: %w", err)
	}

	fullLocalPath := filepath.Join(absDownloadDir, filename)
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

// Upstream API: https://gopro.github.io/OpenGoPro/http#tag/Media/operation/OGP_DELETE_SINGLE_FILE
func (c *Client) DeleteSingleMediaFile(ctx context.Context, path string) error {
	// Create this manually as a string to prevent URL encoding.
	fullURL := fmt.Sprintf("%s/gopro/media/delete/file?path=%s", c.baseURL, path)

	resp, err := c.get(ctx, fullURL)
	if err != nil {
		return fmt.Errorf("deleting single media file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deleting single media file: unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Upstream API: https://gopro.github.io/OpenGoPro/http#tag/Query/operation/OGP_GET_DATE_AND_TIME_DST
func (c *Client) getTimezoneOffset(ctx context.Context) (int, error) {
	reqURL := c.baseURL.JoinPath("/gopro/camera/get_date_time").String()

	resp, err := c.get(ctx, reqURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("getting timezone offset: unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var dt cameraDateTime
	if err := json.NewDecoder(resp.Body).Decode(&dt); err != nil {
		return 0, fmt.Errorf("decoding response: %w", err)
	}

	return dt.TZOffset, nil
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

func (pw *progressWriter) Read(p []byte) (int, error) {
	n, err := pw.reader.Read(p)
	pw.written += int64(n)
	pw.bytesWritten += int64(n)

	now := time.Now()
	if now.Sub(pw.lastUpdate) >= pw.interval {
		if pw.totalSize > 0 {
			percent := float64(pw.written) / float64(pw.totalSize) * 100
			pw.logger.Info("download progress", "filename", pw.fileName, "written", pw.written, "total", pw.totalSize, "progress", fmt.Sprintf("%.2f%%", percent))
		} else {
			pw.logger.Info("download progress", "filename", pw.fileName, "written", pw.written)
		}
		pw.lastUpdate = now
	}

	return n, err
}

func (pw *progressWriter) Close() error {
	if closer, ok := pw.reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
