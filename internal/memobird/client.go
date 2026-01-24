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

type Client struct {
	httpClient *http.Client
	baseURL    string
	accessKey  string
	deviceID   string
	userID     int
}

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

type BaseResponse struct {
	ShowAPIResCode  int    `json:"showapi_res_code"`
	ShowAPIResError string `json:"showapi_res_error"`
}

func (r *BaseResponse) IsSuccess() bool {
	return r.ShowAPIResCode == 1
}

func (r *BaseResponse) Error() string {
	return r.ShowAPIResError
}

type BindResponse struct {
	BaseResponse
	UserID int `json:"showapi_userid"`
}

type PrintResponse struct {
	BaseResponse
	Result         int    `json:"result"`
	SmartGuid      string `json:"smartGuid"`
	PrintContentID int    `json:"printcontentid"`
}

func (p *PrintResponse) IsPrinted() bool {
	return p.Result == 1
}

type PrintStatusResponse struct {
	BaseResponse
	PrintFlag      int    `json:"printflag"`
	PrintContentID string `json:"printcontentID"`
}

func (p *PrintStatusResponse) IsPrinted() bool {
	return p.PrintFlag == 1
}

type ImageConvertResponse struct {
	BaseResponse
	Result string `json:"result"`
}

func (c *Client) timestamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

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

func (c *Client) BindUser(ctx context.Context, userIdentifying string) (*BindResponse, error) {
	params := url.Values{}
	params.Set("memobirdID", c.deviceID)
	params.Set("useridentifying", userIdentifying)

	body, err := c.doRequest(ctx, "/home/setuserbind", params)
	if err != nil {
		return nil, err
	}

	var resp BindResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("bind failed: %s", resp.Error())
	}

	return &resp, nil
}

func (c *Client) GetPrintStatus(ctx context.Context, printContentID int) (*PrintStatusResponse, error) {
	params := url.Values{}
	params.Set("printcontentid", fmt.Sprintf("%d", printContentID))

	body, err := c.doRequest(ctx, "/home/getprintstatus", params)
	if err != nil {
		return nil, err
	}

	var resp PrintStatusResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("get status failed: %s", resp.Error())
	}

	return &resp, nil
}

func (c *Client) ConvertToMonochrome(ctx context.Context, imgBase64 string) (*ImageConvertResponse, error) {
	params := url.Values{}
	params.Set("imgBase64String", imgBase64)

	body, err := c.doRequest(ctx, "/home/getSignalBase64Pic", params)
	if err != nil {
		return nil, err
	}

	var resp ImageConvertResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("image conversion failed: %s", resp.Error())
	}

	return &resp, nil
}

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

	body, err := c.doRequest(ctx, "/home/printpaper", params)
	if err != nil {
		return nil, err
	}

	var resp PrintResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("print text failed: %s", resp.Error())
	}

	return &resp, nil
}

func (c *Client) PrintFromURL(ctx context.Context, pageURL string) (*PrintResponse, error) {
	params := url.Values{}
	params.Set("memobirdID", c.deviceID)
	params.Set("userID", fmt.Sprintf("%d", c.userID))
	params.Set("printUrl", pageURL)

	body, err := c.doRequest(ctx, "/home/printpaperFromUrl", params)
	if err != nil {
		return nil, err
	}

	var resp PrintResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("print from URL failed: %s", resp.Error())
	}

	return &resp, nil
}

func (c *Client) PrintFromHTML(ctx context.Context, html string) (*PrintResponse, error) {
	// Try UTF-8 Base64 (no GBK conversion), see if server handles it
	encoded := formatter.EncodeHTMLToUTF8Base64(html)

	// Base64 encoded result will be URL encoded by params.Encode()
	params := url.Values{}
	params.Set("memobirdID", c.deviceID)
	params.Set("userID", fmt.Sprintf("%d", c.userID))
	params.Set("printHtml", encoded)

	body, err := c.doRequest(ctx, "/home/printpaperFromHtml", params)
	if err != nil {
		return nil, err
	}

	var resp PrintResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("print from HTML failed: %s", resp.Error())
	}

	return &resp, nil
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

	body, err := c.doRequest(ctx, "/home/printpaper", params)
	if err != nil {
		return nil, err
	}

	var resp PrintResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("print image failed: %s", resp.Error())
	}

	return &resp, nil
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

	body, err := c.doRequest(ctx, "/home/printpaper", params)
	if err != nil {
		return nil, err
	}

	var resp PrintResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if !resp.IsSuccess() {
		return nil, fmt.Errorf("print image failed: %s", resp.Error())
	}

	return &resp, nil
}

func (c *Client) SetUserID(userID int) {
	c.userID = userID
}

func (c *Client) GetUserID() int {
	return c.userID
}
