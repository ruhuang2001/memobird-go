package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/ruhuang2001/memobird-playground/internal/config"
	"github.com/ruhuang2001/memobird-playground/internal/memobird"
	"github.com/ruhuang2001/memobird-playground/internal/storage"
)

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

func TestRunPrintURLValidatesBeforeNetworkCall(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := memobird.NewClient(&config.MemobirdConfig{AccessKey: "ak", DeviceID: "device-a"})
	client.SetUserID(5)

	err := runPrintURL(context.Background(), client, "mailto:test@example.com", logger)
	if err == nil {
		t.Fatal("runPrintURL() error = nil, want validation error")
	}
	if !errors.Is(err, err) && err.Error() == "" {
		t.Fatal("runPrintURL() returned empty error")
	}
}

func TestRunPrintURLRequiresBinding(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := memobird.NewClient(&config.MemobirdConfig{AccessKey: "ak", DeviceID: "device-a"})

	err := runPrintURL(context.Background(), client, "https://example.com", logger)
	if err == nil {
		t.Fatal("runPrintURL() error = nil, want binding error")
	}
}

func TestRunPrintHTMLRequiresBinding(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := memobird.NewClient(&config.MemobirdConfig{AccessKey: "ak", DeviceID: "device-a"})

	err := runPrintHTML(context.Background(), client, "<html></html>", logger)
	if err == nil {
		t.Fatal("runPrintHTML() error = nil, want binding error")
	}
}

func newTestStore(t *testing.T) *storage.Storage {
	t.Helper()

	store, err := storage.New(filepath.Join(t.TempDir(), "memobird.db"))
	if err != nil {
		t.Fatalf("storage.New() error = %v", err)
	}

	return store
}
