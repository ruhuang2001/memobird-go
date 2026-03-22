package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ruhuang2001/memobird-playground/internal/config"
	"github.com/ruhuang2001/memobird-playground/internal/memobird"
	"github.com/ruhuang2001/memobird-playground/internal/storage"
)

// TestInitUserBindingUsesConfiguredUserID verifies explicit config user IDs win over storage.
func TestInitUserBindingUsesConfiguredUserID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{
		Memobird: config.MemobirdConfig{
			AccessKey: "ak",
			DeviceID:  "device-a",
			UserID:    99,
		},
	}
	client := memobird.NewClient(&cfg.Memobird)
	store := newTestStore(t)
	t.Cleanup(func() { _ = store.Close() })

	if err := initUserBinding(context.Background(), cfg, client, store, logger); err != nil {
		t.Fatalf("initUserBinding() error = %v", err)
	}
	if got := client.GetUserID(); got != 99 {
		t.Fatalf("client.GetUserID() = %d, want 99", got)
	}
}

// TestInitUserBindingLoadsStoredBindingForMatchingDevice verifies stored bindings are reused for the same device.
func TestInitUserBindingLoadsStoredBindingForMatchingDevice(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{
		Memobird: config.MemobirdConfig{
			AccessKey: "ak",
			DeviceID:  "device-a",
		},
	}
	client := memobird.NewClient(&cfg.Memobird)
	store := newTestStore(t)
	t.Cleanup(func() { _ = store.Close() })

	if err := store.SaveUserBinding(context.Background(), 12, "device-a"); err != nil {
		t.Fatalf("SaveUserBinding() error = %v", err)
	}

	if err := initUserBinding(context.Background(), cfg, client, store, logger); err != nil {
		t.Fatalf("initUserBinding() error = %v", err)
	}
	if got := client.GetUserID(); got != 12 {
		t.Fatalf("client.GetUserID() = %d, want 12", got)
	}
}

// TestInitUserBindingIgnoresStoredBindingForDifferentDevice verifies mismatched stored bindings are ignored.
func TestInitUserBindingIgnoresStoredBindingForDifferentDevice(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{
		Memobird: config.MemobirdConfig{
			AccessKey: "ak",
			DeviceID:  "device-a",
		},
	}
	client := memobird.NewClient(&cfg.Memobird)
	store := newTestStore(t)
	t.Cleanup(func() { _ = store.Close() })

	if err := store.SaveUserBinding(context.Background(), 12, "device-b"); err != nil {
		t.Fatalf("SaveUserBinding() error = %v", err)
	}

	if err := initUserBinding(context.Background(), cfg, client, store, logger); err != nil {
		t.Fatalf("initUserBinding() error = %v", err)
	}
	if got := client.GetUserID(); got != 0 {
		t.Fatalf("client.GetUserID() = %d, want 0", got)
	}
}

// TestRequireBoundUser verifies the helper rejects missing user bindings and accepts configured ones.
func TestRequireBoundUser(t *testing.T) {
	client := memobird.NewClient(&config.MemobirdConfig{AccessKey: "ak", DeviceID: "device-a"})

	err := requireBoundUser(client)
	if err == nil {
		t.Fatal("requireBoundUser() error = nil, want error")
	}

	client.SetUserID(5)
	if err := requireBoundUser(client); err != nil {
		t.Fatalf("requireBoundUser() error = %v", err)
	}
}

// TestRunPrintURLValidatesBeforeNetworkCall verifies URL validation fails before any network dependency is needed.
func TestRunPrintURLValidatesBeforeNetworkCall(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := memobird.NewClient(&config.MemobirdConfig{AccessKey: "ak", DeviceID: "device-a"})
	client.SetUserID(5)

	err := runPrintURL(context.Background(), client, "mailto:test@example.com", logger)
	if err == nil {
		t.Fatal("runPrintURL() error = nil, want validation error")
	}
	if err.Error() == "" {
		t.Fatal("runPrintURL() returned empty error")
	}
}

// TestRunPrintURLRequiresBinding verifies URL printing refuses to run without a bound user.
func TestRunPrintURLRequiresBinding(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := memobird.NewClient(&config.MemobirdConfig{AccessKey: "ak", DeviceID: "device-a"})

	err := runPrintURL(context.Background(), client, "https://example.com", logger)
	if err == nil {
		t.Fatal("runPrintURL() error = nil, want binding error")
	}
}

// TestRunPrintHTMLRequiresBinding verifies HTML printing refuses to run without a bound user.
func TestRunPrintHTMLRequiresBinding(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := memobird.NewClient(&config.MemobirdConfig{AccessKey: "ak", DeviceID: "device-a"})

	err := runPrintHTML(context.Background(), client, "<html></html>", logger)
	if err == nil {
		t.Fatal("runPrintHTML() error = nil, want binding error")
	}
}

func TestRunPrintURLAsImageSubmitsPagesSequentially(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	renderer := fakeImageRenderer{
		urlPages: []string{mustCreateBase64PNG(t, 16, 16), mustCreateBase64PNG(t, 16, 24)},
	}
	printer := &fakeImagePrinter{
		userID:     5,
		contentIDs: []int{101, 102},
	}

	err := runPrintURLAsImage(context.Background(), printer, renderer, "https://example.com", logger)
	if err != nil {
		t.Fatalf("runPrintURLAsImage() error = %v", err)
	}

	if len(printer.printedImages) != 2 {
		t.Fatalf("printed page count = %d, want 2", len(printer.printedImages))
	}
	if !strings.Contains(logBuf.String(), "page_count=2") {
		t.Fatalf("logs = %q, want page_count=2", logBuf.String())
	}
	if !strings.Contains(logBuf.String(), "page=1") || !strings.Contains(logBuf.String(), "page=2") {
		t.Fatalf("logs = %q, want per-page logging", logBuf.String())
	}
}

func TestRunPrintHTMLAsImageReportsPageFailure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	renderer := fakeImageRenderer{
		htmlPages: []string{
			mustCreateBase64PNG(t, 16, 16),
			mustCreateBase64PNG(t, 16, 24),
		},
	}
	printer := &fakeImagePrinter{
		userID:     5,
		contentIDs: []int{201},
		failPage:   2,
	}

	err := runPrintHTMLAsImage(context.Background(), printer, renderer, "<html></html>", logger)
	if err == nil {
		t.Fatal("runPrintHTMLAsImage() error = nil, want page failure")
	}
	if !strings.Contains(err.Error(), "page 2/2") {
		t.Fatalf("runPrintHTMLAsImage() error = %v, want failing page context", err)
	}
	if len(printer.printedImages) != 1 {
		t.Fatalf("printed page count = %d, want 1 before failure", len(printer.printedImages))
	}
}

func TestRunPrintURLAsImageFallsBackToSinglePageRenderer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	renderer := fakeSinglePageRenderer{
		urlImage: mustCreateBase64PNG(t, 16, 16),
	}
	printer := &fakeImagePrinter{
		userID:     5,
		contentIDs: []int{301},
	}

	err := runPrintURLAsImage(context.Background(), printer, renderer, "https://example.com", logger)
	if err != nil {
		t.Fatalf("runPrintURLAsImage() error = %v", err)
	}
	if len(printer.printedImages) != 1 {
		t.Fatalf("printed page count = %d, want 1", len(printer.printedImages))
	}
}

// TestSelectActionRejectsConflictingFlags verifies only one action flag can be active at a time.
func TestSelectActionRejectsConflictingFlags(t *testing.T) {
	_, err := selectAction([]actionSpec{
		{name: "print-url", value: "https://example.com"},
		{name: "print-html", value: "<html></html>"},
	})
	if err == nil {
		t.Fatal("selectAction() error = nil, want conflict error")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("selectAction() error = %v, want conflict message", err)
	}
}

// TestSelectActionReturnsNilWithoutFlags verifies usage is shown when no action flag is set.
func TestSelectActionReturnsNilWithoutFlags(t *testing.T) {
	got, err := selectAction(nil)
	if err != nil {
		t.Fatalf("selectAction() error = %v", err)
	}
	if got != nil {
		t.Fatalf("selectAction() = %v, want nil", got)
	}
}

// newTestStore creates a temporary SQLite-backed store for command tests.
func newTestStore(t *testing.T) *storage.Storage {
	t.Helper()

	store, err := storage.New(filepath.Join(t.TempDir(), "memobird.db"))
	if err != nil {
		t.Fatalf("storage.New() error = %v", err)
	}

	return store
}

type fakeImagePrinter struct {
	userID        int
	contentIDs    []int
	failPage      int
	printedImages []string
}

func (f *fakeImagePrinter) GetUserID() int {
	return f.userID
}

func (f *fakeImagePrinter) PrintImage(_ context.Context, imgBase64 string) (*memobird.PrintResponse, error) {
	page := len(f.printedImages) + 1
	if f.failPage > 0 && page == f.failPage {
		return nil, fmt.Errorf("printer failure on page %d", page)
	}

	f.printedImages = append(f.printedImages, imgBase64)

	contentID := 1000 + page
	if len(f.contentIDs) >= page {
		contentID = f.contentIDs[page-1]
	}

	return &memobird.PrintResponse{PrintContentID: contentID, Result: 1}, nil
}

type fakeImageRenderer struct {
	urlPages  []string
	htmlPages []string
}

func (f fakeImageRenderer) RenderURLToImage(_ context.Context, _ string) (string, error) {
	if len(f.urlPages) == 0 {
		return "", fmt.Errorf("no URL pages configured")
	}
	return f.urlPages[0], nil
}

func (f fakeImageRenderer) RenderHTMLToImage(_ context.Context, _ string) (string, error) {
	if len(f.htmlPages) == 0 {
		return "", fmt.Errorf("no HTML pages configured")
	}
	return f.htmlPages[0], nil
}

func (f fakeImageRenderer) RenderURLToImages(_ context.Context, _ string) ([]string, error) {
	return append([]string(nil), f.urlPages...), nil
}

func (f fakeImageRenderer) RenderHTMLToImages(_ context.Context, _ string) ([]string, error) {
	return append([]string(nil), f.htmlPages...), nil
}

type fakeSinglePageRenderer struct {
	urlImage  string
	htmlImage string
}

func (f fakeSinglePageRenderer) RenderURLToImage(_ context.Context, _ string) (string, error) {
	return f.urlImage, nil
}

func (f fakeSinglePageRenderer) RenderHTMLToImage(_ context.Context, _ string) (string, error) {
	return f.htmlImage, nil
}

func mustCreateBase64PNG(t *testing.T, width, height int) string {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes())
}
