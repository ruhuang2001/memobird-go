package memobird

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ruhuang2001/memobird-go/textrender"
)

func newTestServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

func newTestClient(serverURL string) *Client {
	client, err := NewClient(Config{
		AccessKey: "test-ak",
		DeviceID:  "test-device",
		UserID:    123,
		BaseURL:   serverURL,
		Timeout:   5 * time.Second,
	})
	if err != nil {
		panic(err)
	}
	return client
}

func TestNewClientValidatesConfig(t *testing.T) {
	_, err := NewClient(Config{})
	if err == nil {
		t.Fatal("NewClient() error = nil, want validation failure")
	}
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

func TestClient_PrintTextSubmitsRenderedImage(t *testing.T) {
	var submitted atomic.Value // []byte
	var printCalls atomic.Int32

	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/home/getSignalBase64Pic":
			form := mustReadForm(t, r)
			raw, err := base64.StdEncoding.DecodeString(form.Get("imgBase64String"))
			if err != nil {
				t.Fatalf("imgBase64String is not base64: %v", err)
			}
			submitted.Store(raw)
			_, _ = w.Write([]byte(`{"showapi_res_code":1,"result":"bitmap"}`))
		case "/home/printpaper":
			printCalls.Add(1)
			_, _ = w.Write([]byte(`{"showapi_res_code":1,"result":1,"printcontentid":55}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.PrintText(context.Background(), "Print receipt", textrender.Options{})
	if err != nil {
		t.Fatalf("PrintText() error = %v", err)
	}
	if resp.PrintContentID != 55 {
		t.Fatalf("PrintContentID = %d, want 55", resp.PrintContentID)
	}
	if printCalls.Load() != 1 {
		t.Fatalf("printCalls = %d, want 1", printCalls.Load())
	}

	// The payload must be a real 384px-wide PNG, not the text: sending the text
	// is exactly what the vendor cannot decode.
	raw, _ := submitted.Load().([]byte)
	if len(raw) == 0 {
		t.Fatal("no image was submitted for conversion")
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("submitted payload is not a PNG: %v", err)
	}
	if img.Bounds().Dx() != 384 {
		t.Fatalf("image width = %d, want 384", img.Bounds().Dx())
	}
}

func TestClient_PrintTextWrapsRenderFailure(t *testing.T) {
	client := newTestClient("https://example.invalid")

	// Invalid font bytes must surface as a render error, not a panic or a
	// partially printed slip.
	_, err := client.PrintText(context.Background(), "ok", textrender.Options{FontData: []byte("not a font")})
	if err == nil {
		t.Fatal("PrintText() error = nil, want a font parse failure")
	}
	if errors.Is(err, ErrMissingGlyphs) {
		t.Fatalf("PrintText() error = %v, want the parse failure rather than ErrMissingGlyphs", err)
	}
}

func TestClient_PrintTextRejectsUncoveredGlyphs(t *testing.T) {
	var serverCalls atomic.Int32
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		serverCalls.Add(1)
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	})
	defer server.Close()

	client := newTestClient(server.URL)

	// The bundled font covers Latin only, so Chinese without FontData must fail
	// instead of printing a blank box per character.
	_, err := client.PrintText(context.Background(), "中文", textrender.Options{})
	if !errors.Is(err, ErrMissingGlyphs) {
		t.Fatalf("PrintText() error = %v, want ErrMissingGlyphs", err)
	}
	if serverCalls.Load() != 0 {
		t.Fatalf("serverCalls = %d, want 0: nothing may be sent when the font cannot draw the text", serverCalls.Load())
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
			name: "PrintText",
			run: func() error {
				_, err := client.PrintText(context.Background(), "ok", textrender.Options{})
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

func TestClient_PrintImageFailure(t *testing.T) {
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
	if _, err := client.PrintImage(context.Background(), "AA=="); err == nil {
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
	_, err := client.PrintImage(context.Background(), "AA==")
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

func TestClient_UserAgentIsConcurrencySafe(t *testing.T) {
	var got atomic.Value
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		got.Store(r.Header.Get("User-Agent"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"showapi_res_code":1,"showapi_userid":1,"result":"x","printcontentid":1}`))
	})
	defer server.Close()

	client := newTestClient(server.URL)
	if want := "memobird-go/" + Version; client.UserAgent() != want {
		t.Fatalf("default UserAgent() = %q, want %q", client.UserAgent(), want)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			client.SetUserAgent("custom-agent")
			client.SetUserAgent("")
		}
	}()

	for i := 0; i < 50; i++ {
		if _, err := client.convertToMonochrome(t.Context(), "AA=="); err != nil {
			t.Fatalf("request failed: %v", err)
		}
	}
	<-done

	if ua, _ := got.Load().(string); ua != "memobird-go/"+Version && ua != "custom-agent" {
		t.Fatalf("unexpected User-Agent sent: %q", ua)
	}

	client.SetUserAgent("")
	if want := "memobird-go/" + Version; client.UserAgent() != want {
		t.Fatalf("UserAgent() after reset = %q, want %q", client.UserAgent(), want)
	}
}

func TestClient_BaseURLTrailingSlashIsNormalized(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/home/setuserbind" {
			t.Errorf("path = %q, want /home/setuserbind", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"showapi_res_code":1,"showapi_userid":7}`))
	})
	defer server.Close()

	client, err := NewClient(Config{
		AccessKey: "k",
		DeviceID:  "d",
		BaseURL:   server.URL + "///",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.BindUser(t.Context(), "u"); err != nil {
		t.Fatalf("BindUser() error = %v", err)
	}
}

func TestClient_RejectsInvalidBaseURL(t *testing.T) {
	for _, baseURL := range []string{"example.com", "ftp://example.com", "https://"} {
		t.Run(baseURL, func(t *testing.T) {
			if _, err := NewClient(Config{AccessKey: "k", DeviceID: "d", BaseURL: baseURL}); err == nil {
				t.Fatalf("NewClient(BaseURL=%q) error = nil, want validation failure", baseURL)
			}
		})
	}
}
