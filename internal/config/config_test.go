package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadFromFile(t *testing.T) {
	t.Setenv("MEMOBIRD_ACCESS_KEY", "")
	t.Setenv("MEMOBIRD_DEVICE_ID", "")
	t.Setenv("MEMOBIRD_USER_ID", "")
	t.Setenv("MEMOBIRD_BASE_URL", "")
	t.Setenv("MEMOBIRD_TIMEOUT_SEC", "")
	t.Setenv("MEMOBIRD_STORAGE_DB_PATH", "")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("memobird:\n  access_key: file-ak\n  device_id: file-device\n  user_id: 42\n  timeout_sec: 15\nstorage:\n  db_path: ./file.db\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Memobird.AccessKey != "file-ak" {
		t.Fatalf("AccessKey = %q, want %q", cfg.Memobird.AccessKey, "file-ak")
	}
	if cfg.Memobird.DeviceID != "file-device" {
		t.Fatalf("DeviceID = %q, want %q", cfg.Memobird.DeviceID, "file-device")
	}
	if cfg.Memobird.UserID != 42 {
		t.Fatalf("UserID = %d, want 42", cfg.Memobird.UserID)
	}
	if cfg.Memobird.Timeout() != 15*time.Second {
		t.Fatalf("Timeout = %v, want %v", cfg.Memobird.Timeout(), 15*time.Second)
	}
	if cfg.Storage.DBPath != "./file.db" {
		t.Fatalf("DBPath = %q, want %q", cfg.Storage.DBPath, "./file.db")
	}
}

func TestLoadAllowsEnvOnlyConfiguration(t *testing.T) {
	t.Setenv("MEMOBIRD_ACCESS_KEY", "env-ak")
	t.Setenv("MEMOBIRD_DEVICE_ID", "env-device")
	t.Setenv("MEMOBIRD_USER_ID", "7")
	t.Setenv("MEMOBIRD_BASE_URL", "http://example.com")
	t.Setenv("MEMOBIRD_TIMEOUT_SEC", "12")
	t.Setenv("MEMOBIRD_STORAGE_DB_PATH", "/tmp/memobird.db")

	cfg, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Memobird.AccessKey != "env-ak" || cfg.Memobird.DeviceID != "env-device" {
		t.Fatalf("env config not applied: %+v", cfg.Memobird)
	}
	if cfg.Memobird.UserID != 7 {
		t.Fatalf("UserID = %d, want 7", cfg.Memobird.UserID)
	}
	if cfg.Memobird.GetBaseURL() != "http://example.com" {
		t.Fatalf("BaseURL = %q, want %q", cfg.Memobird.GetBaseURL(), "http://example.com")
	}
	if cfg.Memobird.Timeout() != 12*time.Second {
		t.Fatalf("Timeout = %v, want %v", cfg.Memobird.Timeout(), 12*time.Second)
	}
	if cfg.Storage.DBPath != "/tmp/memobird.db" {
		t.Fatalf("DBPath = %q, want %q", cfg.Storage.DBPath, "/tmp/memobird.db")
	}
}

func TestLoadEnvOverridesFile(t *testing.T) {
	t.Setenv("MEMOBIRD_ACCESS_KEY", "env-ak")
	t.Setenv("MEMOBIRD_DEVICE_ID", "env-device")

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := []byte("memobird:\n  access_key: file-ak\n  device_id: file-device\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Memobird.AccessKey != "env-ak" {
		t.Fatalf("AccessKey = %q, want %q", cfg.Memobird.AccessKey, "env-ak")
	}
	if cfg.Memobird.DeviceID != "env-device" {
		t.Fatalf("DeviceID = %q, want %q", cfg.Memobird.DeviceID, "env-device")
	}
}

func TestLoadValidationFailure(t *testing.T) {
	t.Setenv("MEMOBIRD_ACCESS_KEY", "")
	t.Setenv("MEMOBIRD_DEVICE_ID", "")

	_, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want validation error")
	}
}
