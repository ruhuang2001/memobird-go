package memobird

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ruhuang2001/memobird-playground/formatter"
)

const (
	defaultBaseURL = "http://open.memobird.cn"
	defaultTimeout = 30 * time.Second
)

// Config configures a Memobird API client.
type Config struct {
	AccessKey  string
	DeviceID   string
	UserID     int
	BaseURL    string
	Timeout    time.Duration
	HTTPClient *http.Client
}

func (c Config) normalizedHTTPClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}

	timeout := c.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	return &http.Client{Timeout: timeout}
}

func (c Config) normalizedBaseURL() string {
	if c.BaseURL == "" {
		return defaultBaseURL
	}
	return c.BaseURL
}

// Validate checks that required client configuration is present.
func (c Config) Validate() error {
	if c.AccessKey == "" {
		return fmt.Errorf("access key is required")
	}
	if c.DeviceID == "" {
		return fmt.Errorf("device ID is required")
	}
	return nil
}

// Client is a Memobird thermal printer API client.
type Client struct {
	httpClient *http.Client
	baseURL    string
	accessKey  string
	deviceID   string
	userID     int
}

// NewClient creates a new Memobird API client with the provided configuration.
func NewClient(cfg Config) *Client {
	return &Client{
		httpClient: cfg.normalizedHTTPClient(),
		baseURL:    cfg.normalizedBaseURL(),
		accessKey:  cfg.AccessKey,
		deviceID:   cfg.DeviceID,
		userID:     cfg.UserID,
	}
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
	SmartGuid      string `json:"smartGuid"`
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

// timestamp returns the current time in the format required by the API (YYYY-MM-DD HH:MM:SS).
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

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
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
		return nil, fmt.Errorf("%s failed: %s", action, apiResp.Error())
	}

	return &resp, nil
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
	if c.userID == 0 {
		return fmt.Errorf("user_id not configured")
	}

	return nil
}

// BindUser binds a user identifier to the current device, returning the assigned user ID.
func (c *Client) BindUser(ctx context.Context, userIdentifying string) (*BindResponse, error) {
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
	params.Set("userID", fmt.Sprintf("%d", c.userID))
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
	params.Set("userID", fmt.Sprintf("%d", c.userID))
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
	params.Set("userID", fmt.Sprintf("%d", c.userID))
	params.Set("printcontent", printContent)

	return postFormJSON[PrintResponse](ctx, c, "/home/printpaper", params, "print image")
}

// SetUserID sets the user ID for subsequent print requests.
func (c *Client) SetUserID(userID int) {
	c.userID = userID
}

// GetUserID returns the currently configured user ID.
func (c *Client) GetUserID() int {
	return c.userID
}
