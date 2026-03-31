package memobird

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func newTestServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

func newTestClient(serverURL string) *Client {
	return NewClient(Config{
		AccessKey: "test-ak",
		DeviceID:  "test-device",
		UserID:    123,
		BaseURL:   serverURL,
		Timeout:   5 * time.Second,
	})
}

func TestClient_BindUser(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/home/setuserbind" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}

		resp := BindResponse{
			BaseResponse: BaseResponse{ShowAPIResCode: 1, ShowAPIResError: "ok"},
			UserID:       456,
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.BindUser(context.Background(), "test-user")
	if err != nil {
		t.Fatalf("BindUser() error = %v", err)
	}
	if resp.UserID != 456 {
		t.Fatalf("UserID = %d, want 456", resp.UserID)
	}
}

func TestClient_GetPrintStatus(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/home/getprintstatus" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := PrintStatusResponse{
			BaseResponse:   BaseResponse{ShowAPIResCode: 1, ShowAPIResError: "ok"},
			PrintFlag:      1,
			PrintContentID: 789,
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.GetPrintStatus(context.Background(), 789)
	if err != nil {
		t.Fatalf("GetPrintStatus() error = %v", err)
	}
	if !resp.IsPrinted() {
		t.Fatal("expected IsPrinted() to be true")
	}
}

func TestClient_ConvertToMonochrome(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/home/getSignalBase64Pic" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := ImageConvertResponse{
			BaseResponse: BaseResponse{ShowAPIResCode: 1, ShowAPIResError: "ok"},
			Result:       "converted-base64-data",
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.ConvertToMonochrome(context.Background(), "original-base64")
	if err != nil {
		t.Fatalf("ConvertToMonochrome() error = %v", err)
	}
	if resp.Result != "converted-base64-data" {
		t.Fatalf("Result = %s, want converted-base64-data", resp.Result)
	}
}

func TestClient_PrintFromURLIncludesExpectedFormFields(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/x-www-form-urlencoded" {
			t.Fatalf("Content-Type = %q, want application/x-www-form-urlencoded", got)
		}

		form := mustReadForm(t, r)
		if got := form.Get("memobirdID"); got != "test-device" {
			t.Fatalf("memobirdID = %q, want %q", got, "test-device")
		}
		if got := form.Get("userID"); got != "123" {
			t.Fatalf("userID = %q, want %q", got, "123")
		}
		if got := form.Get("printUrl"); got != "https://example.com" {
			t.Fatalf("printUrl = %q, want https://example.com", got)
		}
		if got := form.Get("ak"); got != "test-ak" {
			t.Fatalf("ak = %q, want %q", got, "test-ak")
		}
		if form.Get("timestamp") == "" {
			t.Fatal("timestamp is empty")
		}

		resp := PrintResponse{
			BaseResponse:   BaseResponse{ShowAPIResCode: 1, ShowAPIResError: "ok"},
			Result:         1,
			PrintContentID: 999,
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.PrintFromURL(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("PrintFromURL() error = %v", err)
	}
	if resp.PrintContentID != 999 {
		t.Fatalf("PrintContentID = %d, want 999", resp.PrintContentID)
	}
}

func TestClient_PrintFromHTMLEncodesGBKPayload(t *testing.T) {
	html := "<h1>中文测试</h1>"

	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/home/printpaperFromHtml" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		form := mustReadForm(t, r)
		if got := form.Get("memobirdID"); got != "test-device" {
			t.Fatalf("memobirdID = %q, want %q", got, "test-device")
		}
		if got := form.Get("userID"); got != "123" {
			t.Fatalf("userID = %q, want %q", got, "123")
		}

		decoded := decodeGBKBase64(t, form.Get("printHtml"))
		if decoded != html {
			t.Fatalf("decoded HTML = %q, want %q", decoded, html)
		}

		resp := PrintResponse{
			BaseResponse:   BaseResponse{ShowAPIResCode: 1, ShowAPIResError: "ok"},
			Result:         1,
			PrintContentID: 888,
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.PrintFromHTML(context.Background(), html)
	if err != nil {
		t.Fatalf("PrintFromHTML() error = %v", err)
	}
	if resp.PrintContentID != 888 {
		t.Fatalf("PrintContentID = %d, want 888", resp.PrintContentID)
	}
}

func TestClient_PrintImageUsesSharedPipeline(t *testing.T) {
	var convertCalls atomic.Int32
	var printCalls atomic.Int32

	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/home/getSignalBase64Pic":
			convertCalls.Add(1)
			resp := ImageConvertResponse{
				BaseResponse: BaseResponse{ShowAPIResCode: 1, ShowAPIResError: "ok"},
				Result:       "signal-bitmap",
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "/home/printpaper":
			printCalls.Add(1)
			form := mustReadForm(t, r)
			if got := form.Get("printcontent"); got != "P:signal-bitmap" {
				t.Fatalf("printcontent = %q, want %q", got, "P:signal-bitmap")
			}

			resp := PrintResponse{
				BaseResponse:   BaseResponse{ShowAPIResCode: 1, ShowAPIResError: "ok"},
				Result:         1,
				PrintContentID: 321,
			}
			_ = json.NewEncoder(w).Encode(resp)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.PrintImage(context.Background(), "source-image")
	if err != nil {
		t.Fatalf("PrintImage() error = %v", err)
	}
	if resp.PrintContentID != 321 {
		t.Fatalf("PrintContentID = %d, want 321", resp.PrintContentID)
	}
	if convertCalls.Load() != 1 {
		t.Fatalf("convertCalls = %d, want 1", convertCalls.Load())
	}
	if printCalls.Load() != 1 {
		t.Fatalf("printCalls = %d, want 1", printCalls.Load())
	}
}

func TestClient_PrintingRequiresConfiguredUserID(t *testing.T) {
	serverCalls := atomic.Int32{}
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		serverCalls.Add(1)
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	})
	defer server.Close()

	client := newTestClient(server.URL)
	client.SetUserID(0)

	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "PrintFromURL",
			run: func() error {
				_, err := client.PrintFromURL(context.Background(), "https://example.com")
				return err
			},
		},
		{
			name: "PrintFromHTML",
			run: func() error {
				_, err := client.PrintFromHTML(context.Background(), "<p>ok</p>")
				return err
			},
		},
		{
			name: "PrintImage",
			run: func() error {
				_, err := client.PrintImage(context.Background(), "img")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.run()
			if err == nil || !strings.Contains(err.Error(), "user_id not configured") {
				t.Fatalf("error = %v, want user_id not configured", err)
			}
		})
	}

	if serverCalls.Load() != 0 {
		t.Fatalf("serverCalls = %d, want 0", serverCalls.Load())
	}
}

func TestClient_DoRequestDoesNotMutateCallerParams(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		resp := BindResponse{
			BaseResponse: BaseResponse{ShowAPIResCode: 1, ShowAPIResError: "ok"},
			UserID:       456,
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := newTestClient(server.URL)
	params := url.Values{}
	params.Set("memobirdID", "device-a")

	if _, err := client.doRequest(context.Background(), "/home/setuserbind", params); err != nil {
		t.Fatalf("doRequest() error = %v", err)
	}

	if got := params.Get("ak"); got != "" {
		t.Fatalf("params ak = %q, want empty", got)
	}
	if got := params.Get("timestamp"); got != "" {
		t.Fatalf("params timestamp = %q, want empty", got)
	}
}

func TestClient_PrintFromURLFailure(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		resp := PrintResponse{
			BaseResponse: BaseResponse{
				ShowAPIResCode:  0,
				ShowAPIResError: "Authorization code expired or invalid access",
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := newTestClient(server.URL)
	if _, err := client.PrintFromURL(context.Background(), "https://example.com"); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestClient_InvalidJSONIncludesResponseSnippet(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html>gateway error</html>"))
	})
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.PrintFromURL(context.Background(), "https://example.com")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "body=") || !strings.Contains(err.Error(), "gateway error") {
		t.Fatalf("error = %q, want response snippet", err)
	}
}

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

func mustReadForm(t *testing.T, r *http.Request) url.Values {
	t.Helper()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}

	form, err := url.ParseQuery(string(body))
	if err != nil {
		t.Fatalf("failed to parse form body: %v", err)
	}

	return form
}

func decodeGBKBase64(t *testing.T, encoded string) string {
	t.Helper()

	gbkBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("printHtml is not valid base64: %v", err)
	}

	decoded, _, err := transform.Bytes(simplifiedchinese.GBK.NewDecoder(), gbkBytes)
	if err != nil {
		t.Fatalf("printHtml is not GBK decodable: %v", err)
	}

	return string(decoded)
}
