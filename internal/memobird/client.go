package memobird

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/ruhuang2001/memobird-playground/internal/config"
	"github.com/ruhuang2001/memobird-playground/internal/formatter"
)

// Client is a Memobird thermal printer API client.
type Client struct {
	httpClient *http.Client
	baseURL    string
	accessKey  string
	deviceID   string
	userID     int
}

// NewClient creates a new Memobird API client with the provided configuration.
func NewClient(cfg *config.MemobirdConfig) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: cfg.Timeout(),
		},
		baseURL:   cfg.GetBaseURL(),
		accessKey: cfg.AccessKey,
		deviceID:  cfg.DeviceID,
		userID:    cfg.UserID,
	}
}

// BaseResponse contains common fields returned by all API endpoints.
type BaseResponse struct {
	ShowAPIResCode  int    `json:"showapi_res_code"`
	ShowAPIResError string `json:"showapi_res_error"`
}

type apiResponse interface {
	IsSuccess() bool
	Error() string
}

func (r *BaseResponse) IsSuccess() bool {
	return r.ShowAPIResCode == 1
}

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

func (p *PrintResponse) IsPrinted() bool {
	return p.Result == 1
}

// PrintStatusResponse contains the status of a print job.
type PrintStatusResponse struct {
	BaseResponse
	PrintFlag      int    `json:"printflag"`
	PrintContentID string `json:"printcontentID"`
}

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
	params.Set("ak", c.accessKey)
	params.Set("timestamp", c.timestamp())

	reqURL := fmt.Sprintf("%s%s", c.baseURL, endpoint)

	// Use application/x-www-form-urlencoded format (required by API documentation)
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

func postFormJSON[T any](ctx context.Context, c *Client, endpoint string, params url.Values, action string) (*T, error) {
	body, err := c.doRequest(ctx, endpoint, params)
	if err != nil {
		return nil, err
	}

	var resp T
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
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

// PrintText prints plain text content to the thermal printer.
// The text is encoded to GBK and base64 before sending.
func (c *Client) PrintText(ctx context.Context, text string) (*PrintResponse, error) {
	encoded, err := formatter.EncodeTextToGBKBase64(text)
	if err != nil {
		return nil, fmt.Errorf("failed to encode text: %w", err)
	}

	// Use T: prefix to indicate text content
	printContent := "T:" + encoded

	params := url.Values{}
	params.Set("memobirdID", c.deviceID)
	params.Set("userID", fmt.Sprintf("%d", c.userID))
	params.Set("printcontent", printContent)

	return postFormJSON[PrintResponse](ctx, c, "/home/printpaper", params, "print text")
}

// PrintFromURL prints content from a web page by providing its URL to the print service.
func (c *Client) PrintFromURL(ctx context.Context, pageURL string) (*PrintResponse, error) {
	params := url.Values{}
	params.Set("memobirdID", c.deviceID)
	params.Set("userID", fmt.Sprintf("%d", c.userID))
	params.Set("printUrl", pageURL)

	return postFormJSON[PrintResponse](ctx, c, "/home/printpaperFromUrl", params, "print from URL")
}

// PrintFromHTML prints HTML content directly to the thermal printer.
func (c *Client) PrintFromHTML(ctx context.Context, html string) (*PrintResponse, error) {
	encoded, err := formatter.EncodeHTMLToGBKBase64(html)
	if err != nil {
		return nil, fmt.Errorf("failed to encode HTML: %w", err)
	}

	// Base64 encoded result will be URL encoded by params.Encode()
	params := url.Values{}
	params.Set("memobirdID", c.deviceID)
	params.Set("userID", fmt.Sprintf("%d", c.userID))
	params.Set("printHtml", encoded)

	return postFormJSON[PrintResponse](ctx, c, "/home/printpaperFromHtml", params, "print from HTML")
}

// PrintImage prints a base64-encoded image (will be converted to monochrome by API)
func (c *Client) PrintImage(ctx context.Context, imgBase64 string) (*PrintResponse, error) {
	// First convert the image to monochrome using API
	convertResp, err := c.ConvertToMonochrome(ctx, imgBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to convert image: %w", err)
	}

	// Use P: prefix for image content
	printContent := "P:" + convertResp.Result

	params := url.Values{}
	params.Set("memobirdID", c.deviceID)
	params.Set("userID", fmt.Sprintf("%d", c.userID))
	params.Set("printcontent", printContent)

	return postFormJSON[PrintResponse](ctx, c, "/home/printpaper", params, "print image")
}

// PrintImageProcessed prints a pre-processed base64 image (already 384px wide monochrome)
// Uses API to convert to printer format, but image is already optimized locally
func (c *Client) PrintImageProcessed(ctx context.Context, processedImgBase64 string) (*PrintResponse, error) {
	// Use API to convert to printer bitmap format
	convertResp, err := c.ConvertToMonochrome(ctx, processedImgBase64)
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
