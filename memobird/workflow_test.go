package memobird

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

type stubBindingStore struct {
	savedUserID   int
	savedDeviceID string
	saveErr       error
	loadUserID    int
	loadDeviceID  string
	loadErr       error
}

func (s *stubBindingStore) SaveUserBinding(_ context.Context, userID int, deviceID string) error {
	if s.saveErr != nil {
		return s.saveErr
	}

	s.savedUserID = userID
	s.savedDeviceID = deviceID
	return nil
}

func (s *stubBindingStore) GetUserBinding(_ context.Context) (int, string, error) {
	if s.loadErr != nil {
		return 0, "", s.loadErr
	}

	return s.loadUserID, s.loadDeviceID, nil
}

type stubImageRenderer struct {
	urlImage  string
	htmlImage string
	urlErr    error
	htmlErr   error
}

func (r *stubImageRenderer) RenderURLToImage(_ context.Context, _ string) (string, error) {
	if r.urlErr != nil {
		return "", r.urlErr
	}
	return r.urlImage, nil
}

func (r *stubImageRenderer) RenderHTMLToImage(_ context.Context, _ string) (string, error) {
	if r.htmlErr != nil {
		return "", r.htmlErr
	}
	return r.htmlImage, nil
}

type pagedStubImageRenderer struct {
	stubImageRenderer
	urlImages  []string
	htmlImages []string
}

func (r *pagedStubImageRenderer) RenderURLToImages(_ context.Context, _ string) ([]string, error) {
	if r.urlErr != nil {
		return nil, r.urlErr
	}
	return r.urlImages, nil
}

func (r *pagedStubImageRenderer) RenderHTMLToImages(_ context.Context, _ string) ([]string, error) {
	if r.htmlErr != nil {
		return nil, r.htmlErr
	}
	return r.htmlImages, nil
}

func createBase64PNG(t *testing.T, width, height int) string {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			shade := uint8((x + y) * 255 / (width + height))
			img.Set(x, y, color.RGBA{shade, shade, shade, 255})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestClient_BindAndPersistStoresBinding(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		resp := BindResponse{
			BaseResponse: BaseResponse{ShowAPIResCode: 1, ShowAPIResError: "ok"},
			UserID:       456,
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := newTestClient(server.URL)
	client.SetUserID(0)

	store := &stubBindingStore{}
	resp, err := client.BindAndPersist(context.Background(), store, "test-user")
	if err != nil {
		t.Fatalf("BindAndPersist() error = %v", err)
	}
	if resp.UserID != 456 {
		t.Fatalf("resp.UserID = %d, want 456", resp.UserID)
	}
	if client.GetUserID() != 456 {
		t.Fatalf("client.GetUserID() = %d, want 456", client.GetUserID())
	}
	if store.savedUserID != 456 || store.savedDeviceID != "test-device" {
		t.Fatalf("saved binding = (%d, %q), want (456, %q)", store.savedUserID, store.savedDeviceID, "test-device")
	}
}

func TestClient_BindAndPersistSurfacesPersistenceFailure(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		resp := BindResponse{
			BaseResponse: BaseResponse{ShowAPIResCode: 1, ShowAPIResError: "ok"},
			UserID:       456,
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := newTestClient(server.URL)
	client.SetUserID(0)

	store := &stubBindingStore{saveErr: fmt.Errorf("disk full")}
	resp, err := client.BindAndPersist(context.Background(), store, "test-user")
	if err == nil {
		t.Fatal("BindAndPersist() error = nil, want persistence failure")
	}
	if resp == nil || resp.UserID != 456 {
		t.Fatalf("resp = %+v, want successful bind response", resp)
	}
	if client.GetUserID() != 456 {
		t.Fatalf("client.GetUserID() = %d, want 456", client.GetUserID())
	}
}

func TestClient_BindAndPersistRequiresStoreBeforeBinding(t *testing.T) {
	var bindCalls atomic.Int32
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		bindCalls.Add(1)
		http.Error(w, "unexpected bind", http.StatusInternalServerError)
	})
	defer server.Close()

	client := newTestClient(server.URL)
	client.SetUserID(0)

	resp, err := client.BindAndPersist(context.Background(), nil, "test-user")
	if err == nil || err.Error() != "binding store is required" {
		t.Fatalf("BindAndPersist() error = %v, want binding store is required", err)
	}
	if resp != nil {
		t.Fatalf("resp = %+v, want nil", resp)
	}
	if bindCalls.Load() != 0 {
		t.Fatalf("bindCalls = %d, want 0", bindCalls.Load())
	}
	if client.GetUserID() != 0 {
		t.Fatalf("client.GetUserID() = %d, want 0", client.GetUserID())
	}
}

func TestClient_PersistUserBindingRequiresUserID(t *testing.T) {
	client := newTestClient("http://example.com")
	client.SetUserID(0)

	err := client.PersistUserBinding(context.Background(), &stubBindingStore{})
	if err == nil || err.Error() != "user_id not configured" {
		t.Fatalf("PersistUserBinding() error = %v, want user_id not configured", err)
	}
}

func TestClient_RestoreUserBindingLoadsMatchingDevice(t *testing.T) {
	client := newTestClient("http://example.com")
	client.SetUserID(0)

	loaded, err := client.RestoreUserBinding(context.Background(), &stubBindingStore{
		loadUserID:   789,
		loadDeviceID: "test-device",
	})
	if err != nil {
		t.Fatalf("RestoreUserBinding() error = %v", err)
	}
	if !loaded {
		t.Fatal("loaded = false, want true")
	}
	if client.GetUserID() != 789 {
		t.Fatalf("client.GetUserID() = %d, want 789", client.GetUserID())
	}
}

func TestClient_RestoreUserBindingReturnsFalseWhenStoreIsEmpty(t *testing.T) {
	client := newTestClient("http://example.com")
	client.SetUserID(0)

	loaded, err := client.RestoreUserBinding(context.Background(), &stubBindingStore{})
	if err != nil {
		t.Fatalf("RestoreUserBinding() error = %v", err)
	}
	if loaded {
		t.Fatal("loaded = true, want false")
	}
	if client.GetUserID() != 0 {
		t.Fatalf("client.GetUserID() = %d, want 0", client.GetUserID())
	}
}

func TestClient_RestoreUserBindingRejectsDifferentDevice(t *testing.T) {
	client := newTestClient("http://example.com")
	client.SetUserID(0)

	loaded, err := client.RestoreUserBinding(context.Background(), &stubBindingStore{
		loadUserID:   789,
		loadDeviceID: "other-device",
	})
	if err == nil {
		t.Fatal("RestoreUserBinding() error = nil, want device mismatch")
	}
	if loaded {
		t.Fatal("loaded = true, want false")
	}
	if client.GetUserID() != 0 {
		t.Fatalf("client.GetUserID() = %d, want 0", client.GetUserID())
	}
}

func TestClient_PrintHTMLAsImagesRequiresRenderer(t *testing.T) {
	client := newTestClient("http://example.com")

	var render *stubImageRenderer
	responses, err := client.PrintHTMLAsImages(context.Background(), render, "<p>Hello</p>")
	if err == nil || err.Error() != "image renderer is required" {
		t.Fatalf("PrintHTMLAsImages() error = %v, want image renderer is required", err)
	}
	if responses != nil {
		t.Fatalf("responses = %+v, want nil", responses)
	}
}

func TestClient_PrintURLAsImagesReturnsPartialResponsesOnProcessingFailure(t *testing.T) {
	var convertCalls atomic.Int32
	var printCalls atomic.Int32
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/home/getSignalBase64Pic":
			convertCalls.Add(1)
			_ = json.NewEncoder(w).Encode(ImageConvertResponse{
				BaseResponse: BaseResponse{ShowAPIResCode: 1, ShowAPIResError: "ok"},
				Result:       "signal-bitmap",
			})
		case "/home/printpaper":
			printCalls.Add(1)
			_ = json.NewEncoder(w).Encode(PrintResponse{
				BaseResponse:   BaseResponse{ShowAPIResCode: 1, ShowAPIResError: "ok"},
				Result:         1,
				PrintContentID: 321,
			})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	})
	defer server.Close()

	client := newTestClient(server.URL)
	render := &pagedStubImageRenderer{
		urlImages: []string{
			createBase64PNG(t, 32, 32),
			"not-base64",
		},
	}

	responses, err := client.PrintURLAsImages(context.Background(), render, "https://example.com")
	if err == nil {
		t.Fatal("PrintURLAsImages() error = nil, want processing failure")
	}
	if !strings.Contains(err.Error(), "failed to process page 2/2") {
		t.Fatalf("PrintURLAsImages() error = %v, want page 2 processing failure", err)
	}
	if len(responses) != 1 {
		t.Fatalf("len(responses) = %d, want 1", len(responses))
	}
	if responses[0].PrintContentID != 321 {
		t.Fatalf("responses[0].PrintContentID = %d, want 321", responses[0].PrintContentID)
	}
	if convertCalls.Load() != 1 {
		t.Fatalf("convertCalls = %d, want 1", convertCalls.Load())
	}
	if printCalls.Load() != 1 {
		t.Fatalf("printCalls = %d, want 1", printCalls.Load())
	}
}
