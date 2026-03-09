package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestStorageReturnsZeroValuesWhenEmpty(t *testing.T) {
	store := newTestStorage(t)
	t.Cleanup(func() { _ = store.Close() })

	userID, deviceID, err := store.GetUserBinding(context.Background())
	if err != nil {
		t.Fatalf("GetUserBinding() error = %v", err)
	}
	if userID != 0 || deviceID != "" {
		t.Fatalf("GetUserBinding() = (%d, %q), want zero values", userID, deviceID)
	}
}

func TestStorageSaveAndLoadBinding(t *testing.T) {
	store := newTestStorage(t)
	t.Cleanup(func() { _ = store.Close() })

	if err := store.SaveUserBinding(context.Background(), 12, "device-a"); err != nil {
		t.Fatalf("SaveUserBinding() error = %v", err)
	}

	userID, deviceID, err := store.GetUserBinding(context.Background())
	if err != nil {
		t.Fatalf("GetUserBinding() error = %v", err)
	}
	if userID != 12 || deviceID != "device-a" {
		t.Fatalf("GetUserBinding() = (%d, %q), want (12, %q)", userID, deviceID, "device-a")
	}
}

func TestStorageSaveReplacesExistingBinding(t *testing.T) {
	store := newTestStorage(t)
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	if err := store.SaveUserBinding(ctx, 12, "device-a"); err != nil {
		t.Fatalf("first SaveUserBinding() error = %v", err)
	}
	if err := store.SaveUserBinding(ctx, 34, "device-b"); err != nil {
		t.Fatalf("second SaveUserBinding() error = %v", err)
	}

	userID, deviceID, err := store.GetUserBinding(ctx)
	if err != nil {
		t.Fatalf("GetUserBinding() error = %v", err)
	}
	if userID != 34 || deviceID != "device-b" {
		t.Fatalf("GetUserBinding() = (%d, %q), want (34, %q)", userID, deviceID, "device-b")
	}
}

func newTestStorage(t *testing.T) *Storage {
	t.Helper()

	store, err := New(filepath.Join(t.TempDir(), "memobird.db"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	return store
}
