package memobird

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
