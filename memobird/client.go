package memobird

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ruhuang2001/memobird-go/formatter"
)

const (
	defaultBaseURL = "https://open.memobird.cn"
	defaultTimeout = 30 * time.Second

	// defaultUserAgent identifies this library to the Memobird API.
	defaultUserAgent = "memobird-go/" + Version

	// maxResponseBytes bounds how much of an API response is read into memory.
	maxResponseBytes = 1 << 20
)

// Version is the library version, kept in sync with the latest release tag.
const Version = "0.0.2"

var (
	// ErrUserIDNotConfigured is returned when a print call runs without a bound user.
	ErrUserIDNotConfigured = errors.New("user_id not configured")
	// ErrBindingStoreRequired is returned when a binding workflow is called without a store.
	ErrBindingStoreRequired = errors.New("binding store is required")
	// ErrRendererRequired is returned when a local render workflow is called without a renderer.
	ErrRendererRequired = errors.New("image renderer is required")
	// ErrNoPagesRendered is returned when a renderer yields no pages to print.
	ErrNoPagesRendered = errors.New("renderer returned no pages")
)

// APIError reports a request that the Memobird API answered with a non-success code.
type APIError struct {
	Action  string
	Code    int
	Message string
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("%s failed: showapi_res_code=%d", e.Action, e.Code)
	}
	return fmt.Sprintf("%s failed: %s", e.Action, e.Message)
}

// Config configures a Memobird API client.
type Config struct {
	AccessKey  string
	DeviceID   string
	UserID     int
	BaseURL    string
	Timeout    time.Duration
	HTTPClient *http.Client
	// PrintInterval is the delay between page submissions in the multi-page
	// print workflows. Zero submits pages back to back.
	PrintInterval time.Duration
}

func (c Config) normalizedHTTPClient() *http.Client {
	if c.HTTPClient != nil {
		if c.Timeout > 0 && c.HTTPClient.Timeout == 0 {
			cloned := *c.HTTPClient
			cloned.Timeout = c.Timeout
			return &cloned
		}
		return c.HTTPClient
	}

	timeout := c.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	return &http.Client{Timeout: timeout}
}

// normalizedBaseURL trims trailing slashes so endpoint paths always join cleanly.
func (c Config) normalizedBaseURL() string {
	if c.BaseURL == "" {
		return defaultBaseURL
	}
	return strings.TrimRight(c.BaseURL, "/")
}

// Validate checks that required client configuration is present.
func (c Config) Validate() error {
	if c.AccessKey == "" {
		return fmt.Errorf("access key is required")
	}
	if c.DeviceID == "" {
		return fmt.Errorf("device ID is required")
	}
	if c.BaseURL != "" {
		parsed, err := url.Parse(c.BaseURL)
		if err != nil {
			return fmt.Errorf("invalid base URL: %w", err)
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return fmt.Errorf("invalid base URL %q: scheme must be http or https", c.BaseURL)
		}
		if parsed.Host == "" {
			return fmt.Errorf("invalid base URL %q: host is required", c.BaseURL)
		}
	}
	return nil
}

// Client is a Memobird thermal printer API client.
// It is safe for concurrent use after construction.
type Client struct {
	httpClient *http.Client
	baseURL    string
	accessKey  string
	deviceID   string
	userAgent  atomic.Value // string
	userID     atomic.Int64

	printInterval time.Duration
}

// NewClient creates a new Memobird API client with the provided configuration.
func NewClient(cfg Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	client := &Client{
		httpClient: cfg.normalizedHTTPClient(),
		baseURL:    cfg.normalizedBaseURL(),
		accessKey:  cfg.AccessKey,
		deviceID:   cfg.DeviceID,

		printInterval: cfg.PrintInterval,
	}
	client.userAgent.Store(defaultUserAgent)
	client.SetUserID(cfg.UserID)
	return client, nil
}

// SetUserAgent overrides the User-Agent header. An empty value restores the default.
// It is safe to call concurrently with in-flight requests.
func (c *Client) SetUserAgent(userAgent string) {
	if userAgent == "" {
		userAgent = defaultUserAgent
	}
	c.userAgent.Store(userAgent)
}

// UserAgent returns the User-Agent header currently sent with requests.
func (c *Client) UserAgent() string {
	if value, ok := c.userAgent.Load().(string); ok {
		return value
	}
	return defaultUserAgent
}

// BaseResponse contains common fields returned by all API endpoints.
type BaseResponse struct {
	ShowAPIResCode  int    `json:"showapi_res_code"`
	ShowAPIResError string `json:"showapi_res_error"`
}

// apiResponse describes the common success/error contract shared by Memobird API responses.
type apiResponse interface {
	IsSuccess() bool
	Error() string
}

// IsSuccess reports whether the API returned a successful response code.
func (r *BaseResponse) IsSuccess() bool {
	return r.ShowAPIResCode == 1
}

// Error returns the API-provided error message.
func (r *BaseResponse) Error() string {
	return r.ShowAPIResError
}

// BindResponse contains the result of a user binding request.
type BindResponse struct {
	BaseResponse
	UserID int `json:"showapi_userid"`
}

// PrintResponse contains the result of a print request.
type PrintResponse struct {
	BaseResponse
	Result         int    `json:"result"`
	SmartGUID      string `json:"smartGuid"`
	PrintContentID int    `json:"printcontentid"`
}

// IsPrinted reports whether the print request was accepted as printed by the API.
func (p *PrintResponse) IsPrinted() bool {
	return p.Result == 1
}

// PrintStatusResponse contains the status of a print job.
type PrintStatusResponse struct {
	BaseResponse
	PrintFlag      int `json:"printflag"`
	PrintContentID int `json:"printcontentid"`
}

// IsPrinted reports whether the queried print job has finished printing.
func (p *PrintStatusResponse) IsPrinted() bool {
	return p.PrintFlag == 1
}

// ImageConvertResponse contains the result of an image conversion request.
type ImageConvertResponse struct {
	BaseResponse
	Result string `json:"result"`
}

// timestamp returns the current client-local time in the format required by the
// API (YYYY-MM-DD HH:MM:SS). The instructions define this as 客户端时间, so it
// deliberately reflects the host's local zone rather than UTC.
func (c *Client) timestamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// doRequest sends an HTTP POST request to the specified endpoint with the given parameters.
// It automatically adds the access key and timestamp to the request.
func (c *Client) doRequest(ctx context.Context, endpoint string, params url.Values) ([]byte, error) {
	params = cloneValues(params)
	params.Set("ak", c.accessKey)
	params.Set("timestamp", c.timestamp())

	reqURL := fmt.Sprintf("%s%s", c.baseURL, endpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader([]byte(params.Encode())))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", c.UserAgent())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if closeErr := resp.Body.Close(); err == nil && closeErr != nil {
		return nil, fmt.Errorf("failed to close response body: %w", closeErr)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if len(body) > maxResponseBytes {
		return nil, fmt.Errorf("response body exceeds max %d bytes", maxResponseBytes)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, summarizeBody(body))
	}

	return body, nil
}

// postFormJSON sends a form request and decodes a successful JSON response into the requested type.
func postFormJSON[T any](ctx context.Context, c *Client, endpoint string, params url.Values, action string) (*T, error) {
	body, err := c.doRequest(ctx, endpoint, params)
	if err != nil {
		return nil, err
	}

	var resp T
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response for %s: %w (body=%q)", action, err, summarizeBody(body))
	}

	apiResp, ok := any(&resp).(apiResponse)
	if !ok {
		return nil, fmt.Errorf("response type %T does not implement apiResponse", resp)
	}

	if !apiResp.IsSuccess() {
		code, message := apiResponseStatus(apiResp)
		return nil, &APIError{Action: action, Code: code, Message: message}
	}

	return &resp, nil
}

// apiResponseStatus extracts the raw response code for typed error reporting.
func apiResponseStatus(resp apiResponse) (int, string) {
	base, ok := resp.(*BaseResponse)
	if !ok {
		return 0, resp.Error()
	}
	return base.ShowAPIResCode, base.ShowAPIResError
}

func cloneValues(values url.Values) url.Values {
	cloned := make(url.Values, len(values))
	for key, items := range values {
		cloned[key] = append([]string(nil), items...)
	}

	return cloned
}

func summarizeBody(body []byte) string {
	const maxLen = 160

	snippet := strings.TrimSpace(string(body))
	if len(snippet) <= maxLen {
		return snippet
	}

	return snippet[:maxLen] + "..."
}

func (c *Client) requireUserID() error {
	if c.GetUserID() == 0 {
		return ErrUserIDNotConfigured
	}

	return nil
}

// BindUser binds a user identifier to the current device, returning the assigned user ID.
func (c *Client) BindUser(ctx context.Context, userIdentifying string) (*BindResponse, error) {
	if strings.TrimSpace(userIdentifying) == "" {
		return nil, fmt.Errorf("user identifying value is required")
	}

	params := url.Values{}
	params.Set("memobirdID", c.deviceID)
	params.Set("useridentifying", userIdentifying)

	return postFormJSON[BindResponse](ctx, c, "/home/setuserbind", params, "bind")
}

// GetPrintStatus retrieves the print status for a given print content ID.
func (c *Client) GetPrintStatus(ctx context.Context, printContentID int) (*PrintStatusResponse, error) {
	params := url.Values{}
	params.Set("printcontentid", fmt.Sprintf("%d", printContentID))

	return postFormJSON[PrintStatusResponse](ctx, c, "/home/getprintstatus", params, "get status")
}

// ConvertToMonochrome converts a base64-encoded image to monochrome format suitable for thermal printing.
func (c *Client) ConvertToMonochrome(ctx context.Context, imgBase64 string) (*ImageConvertResponse, error) {
	params := url.Values{}
	params.Set("imgBase64String", imgBase64)

	return postFormJSON[ImageConvertResponse](ctx, c, "/home/getSignalBase64Pic", params, "image conversion")
}

// PrintFromURL prints content from a web page by providing its URL to the print service.
func (c *Client) PrintFromURL(ctx context.Context, pageURL string) (*PrintResponse, error) {
	if err := c.requireUserID(); err != nil {
		return nil, err
	}

	params := url.Values{}
	params.Set("memobirdID", c.deviceID)
	params.Set("userID", fmt.Sprintf("%d", c.GetUserID()))
	params.Set("printUrl", pageURL)

	return postFormJSON[PrintResponse](ctx, c, "/home/printpaperFromUrl", params, "print from URL")
}

// PrintFromHTML prints HTML content directly to the thermal printer.
func (c *Client) PrintFromHTML(ctx context.Context, html string) (*PrintResponse, error) {
	if err := c.requireUserID(); err != nil {
		return nil, err
	}

	encoded, err := formatter.EncodeHTMLToGBKBase64(html)
	if err != nil {
		return nil, fmt.Errorf("failed to encode HTML: %w", err)
	}

	params := url.Values{}
	params.Set("memobirdID", c.deviceID)
	params.Set("userID", fmt.Sprintf("%d", c.GetUserID()))
	params.Set("printHtml", encoded)

	return postFormJSON[PrintResponse](ctx, c, "/home/printpaperFromHtml", params, "print from HTML")
}

// PrintImage prints a base64-encoded image after converting it to printer bitmap format.
func (c *Client) PrintImage(ctx context.Context, imgBase64 string) (*PrintResponse, error) {
	return c.printImage(ctx, imgBase64)
}

func (c *Client) printImage(ctx context.Context, imgBase64 string) (*PrintResponse, error) {
	if err := c.requireUserID(); err != nil {
		return nil, err
	}

	convertResp, err := c.ConvertToMonochrome(ctx, imgBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to convert image: %w", err)
	}

	printContent := "P:" + convertResp.Result

	params := url.Values{}
	params.Set("memobirdID", c.deviceID)
	params.Set("userID", fmt.Sprintf("%d", c.GetUserID()))
	params.Set("printcontent", printContent)

	return postFormJSON[PrintResponse](ctx, c, "/home/printpaper", params, "print image")
}

// SetUserID sets the user ID for subsequent print requests.
func (c *Client) SetUserID(userID int) {
	c.userID.Store(int64(userID))
}

// GetUserID returns the currently configured user ID.
func (c *Client) GetUserID() int {
	return int(c.userID.Load())
}
