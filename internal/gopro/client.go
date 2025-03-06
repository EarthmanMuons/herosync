package gopro

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

type Client struct {
	httpClient *retryablehttp.Client
	baseURL    *url.URL
	logger     *slog.Logger
}

func NewClient(baseURL *url.URL, logger *slog.Logger) *Client {
	client := retryablehttp.NewClient()
	client.RetryMax = 3
	client.RetryWaitMin = 1 * time.Second
	client.RetryWaitMax = 30 * time.Second

	client.Logger = logger

	return &Client{
		httpClient: client,
		baseURL:    baseURL,
		logger:     logger,
	}
}

// API methods

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

	// Get camera's timezone offset and adjust the media timestamps.
	tzOffset, err := c.getTimezoneOffset(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting timezone offset: %w", err)
	}

	if err := adjustTimestamps(&mediaList, tzOffset); err != nil {
		return nil, fmt.Errorf("adjusting timestamps: %w", err)
	}

	return &mediaList, nil
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

// adjustTimestamps converts camera-local timestamps to UTC while preserving timezone info.
func adjustTimestamps(mediaList *MediaList, tzOffset int) error {
	loc := time.FixedZone("Camera", tzOffset*60)

	for _, dir := range mediaList.Media {
		for i := range dir.Items {
			localTime := time.Unix(int64(dir.Items[i].CreatedAt), 0)
			utcTime := localTime.Add(time.Duration(-tzOffset) * time.Minute)
			dir.Items[i].CreatedAt = Timestamp(utcTime.In(loc).Unix())
		}
	}

	return nil
}

// Helper method for making GET requests.
func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	u := c.baseURL.JoinPath(path)

	req, err := retryablehttp.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return resp, nil
}
