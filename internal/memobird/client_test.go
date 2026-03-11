package memobird

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/ruhuang2001/memobird-playground/internal/config"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// newTestServer creates a test HTTP server with the given handler.
func newTestServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

// newTestClient creates a test client configured to use the given test server URL.
func newTestClient(serverURL string) *Client {
	cfg := &config.MemobirdConfig{
		AccessKey:  "test-ak",
		DeviceID:   "test-device",
		UserID:     123,
		BaseURL:    serverURL,
		TimeoutSec: 5,
	}
	return NewClient(cfg)
}

// TestClient_BindUser verifies the bind endpoint response is decoded correctly.
func TestClient_BindUser(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/home/setuserbind" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}

		resp := BindResponse{
			BaseResponse: BaseResponse{
				ShowAPIResCode:  1,
				ShowAPIResError: "ok",
			},
			UserID: 456,
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.BindUser(context.Background(), "test-user")
	if err != nil {
		t.Fatalf("BindUser() error = %v", err)
	}

	if resp.UserID != 456 {
		t.Errorf("UserID = %d, want 456", resp.UserID)
	}
}

// TestClient_GetPrintStatus verifies print status polling works against the API contract.
func TestClient_GetPrintStatus(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/home/getprintstatus" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := PrintStatusResponse{
			BaseResponse: BaseResponse{
				ShowAPIResCode:  1,
				ShowAPIResError: "ok",
			},
			PrintFlag:      1,
			PrintContentID: "789",
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.GetPrintStatus(context.Background(), 789)
	if err != nil {
		t.Fatalf("GetPrintStatus() error = %v", err)
	}

	if !resp.IsPrinted() {
		t.Error("expected IsPrinted() to be true")
	}
}

// TestClient_ConvertToMonochrome verifies image conversion responses are parsed correctly.
func TestClient_ConvertToMonochrome(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/home/getSignalBase64Pic" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := ImageConvertResponse{
			BaseResponse: BaseResponse{
				ShowAPIResCode:  1,
				ShowAPIResError: "ok",
			},
			Result: "converted-base64-data",
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.ConvertToMonochrome(context.Background(), "original-base64")
	if err != nil {
		t.Fatalf("ConvertToMonochrome() error = %v", err)
	}

	if resp.Result != "converted-base64-data" {
		t.Errorf("Result = %s, want converted-base64-data", resp.Result)
	}
}

// TestClient_PrintFromURL verifies URL printing requests succeed against the expected endpoint.
func TestClient_PrintFromURL(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/home/printpaperFromUrl" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := PrintResponse{
			BaseResponse: BaseResponse{
				ShowAPIResCode:  1,
				ShowAPIResError: "ok",
			},
			Result:         1,
			PrintContentID: 999,
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.PrintFromURL(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("PrintFromURL() error = %v", err)
	}

	if resp.PrintContentID != 999 {
		t.Errorf("PrintContentID = %d, want 999", resp.PrintContentID)
	}
}

// TestClient_PrintFromHTML verifies HTML payloads are encoded and sent correctly.
func TestClient_PrintFromHTML(t *testing.T) {
	html := "<h1>中文测试</h1>"

	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/home/printpaperFromHtml" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}

		form, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("failed to parse form body: %v", err)
		}

		encoded := form.Get("printHtml")
		if encoded == "" {
			t.Fatal("printHtml is empty")
		}

		gbkBytes, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatalf("printHtml is not valid base64: %v", err)
		}

		decoded, _, err := transform.Bytes(simplifiedchinese.GBK.NewDecoder(), gbkBytes)
		if err != nil {
			t.Fatalf("printHtml is not GBK decodable: %v", err)
		}

		if !bytes.Equal(decoded, []byte(html)) {
			t.Fatalf("decoded HTML mismatch: got %q, want %q", string(decoded), html)
		}

		resp := PrintResponse{
			BaseResponse: BaseResponse{
				ShowAPIResCode:  1,
				ShowAPIResError: "ok",
			},
			Result:         1,
			PrintContentID: 888,
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.PrintFromHTML(context.Background(), html)
	if err != nil {
		t.Fatalf("PrintFromHTML() error = %v", err)
	}

	if resp.PrintContentID != 888 {
		t.Errorf("PrintContentID = %d, want 888", resp.PrintContentID)
	}
}

// TestClient_PrintFromURL_Failure verifies API-level failures surface as Go errors.
func TestClient_PrintFromURL_Failure(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		resp := PrintResponse{
			BaseResponse: BaseResponse{
				ShowAPIResCode:  0,
				ShowAPIResError: "Authorization code expired or invalid access",
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.PrintFromURL(context.Background(), "https://example.com")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

// TestBaseResponse_IsSuccess verifies success detection mirrors the Memobird API contract.
func TestBaseResponse_IsSuccess(t *testing.T) {
	tests := []struct {
		code int
		want bool
	}{
		{1, true},
		{0, false},
		{-1, false},
		{2, false},
	}

	for _, tt := range tests {
		resp := BaseResponse{ShowAPIResCode: tt.code}
		if got := resp.IsSuccess(); got != tt.want {
			t.Errorf("IsSuccess() with code %d = %v, want %v", tt.code, got, tt.want)
		}
	}
}
