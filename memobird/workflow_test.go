package memobird

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestClient_WaitForPrintPollsUntilPrinted(t *testing.T) {
	var calls atomic.Int32
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		flag := 0
		if n >= 3 {
			flag = 1
		}
		if _, err := fmt.Fprintf(w, `{"showapi_res_code":1,"printflag":%d,"printcontentid":9}`, flag); err != nil {
			t.Errorf("write response: %v", err)
		}
	})
	defer server.Close()

	client := newTestClient(server.URL)
	status, err := client.WaitForPrint(t.Context(), 9, WaitOptions{Interval: time.Millisecond, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("WaitForPrint() error = %v", err)
	}
	if !status.IsPrinted() || status.PrintContentID != 9 {
		t.Fatalf("status = %+v, want printed job 9", status)
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("poll count = %d, want 3", got)
	}
}

func TestClient_WaitForPrintStopsOnUnknownID(t *testing.T) {
	var calls atomic.Int32
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"showapi_res_code":1,"printflag":-1,"printcontentid":9}`))
	})
	defer server.Close()

	client := newTestClient(server.URL)
	status, err := client.WaitForPrint(t.Context(), 9, WaitOptions{Interval: time.Millisecond, Timeout: 5 * time.Second})
	if !errors.Is(err, ErrUnknownPrintID) {
		t.Fatalf("WaitForPrint() error = %v, want ErrUnknownPrintID", err)
	}
	if status == nil || !status.IsUnknown() {
		t.Fatalf("status = %+v, want unknown status", status)
	}
	// An unknown ID is terminal, so a single poll must settle it.
	if got := calls.Load(); got != 1 {
		t.Fatalf("poll count = %d, want 1", got)
	}
}

func TestClient_WaitForPrintTimesOut(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"showapi_res_code":1,"printflag":0,"printcontentid":9}`))
	})
	defer server.Close()

	client := newTestClient(server.URL)
	start := time.Now()
	status, err := client.WaitForPrint(t.Context(), 9, WaitOptions{Interval: 5 * time.Millisecond, Timeout: 40 * time.Millisecond})
	if err == nil {
		t.Fatal("WaitForPrint() error = nil, want timeout")
	}
	if status == nil || status.IsPrinted() {
		t.Fatalf("status = %+v, want last unprinted status", status)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("timeout took %v, want it bounded by opts.Timeout", elapsed)
	}
}

func TestClient_WaitForPrintRejectsInvalidID(t *testing.T) {
	client := newTestClient("https://example.invalid")
	if _, err := client.WaitForPrint(t.Context(), 0, WaitOptions{}); err == nil {
		t.Fatal("WaitForPrint(id=0) error = nil, want validation failure")
	}
}

func TestClient_WaitForPrintRequiresBoundUser(t *testing.T) {
	client := newTestClient("https://example.invalid")
	client.SetUserID(0)
	if _, err := client.WaitForPrint(t.Context(), 9, WaitOptions{}); !errors.Is(err, ErrUserIDNotConfigured) {
		t.Fatalf("WaitForPrint() error = %v, want ErrUserIDNotConfigured", err)
	}
}

func TestClient_PrintImageRejectsUnboundUser(t *testing.T) {
	var serverCalls atomic.Int32
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		serverCalls.Add(1)
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	})
	defer server.Close()

	client := newTestClient(server.URL)
	client.SetUserID(0)

	if _, err := client.PrintImage(context.Background(), "img"); !errors.Is(err, ErrUserIDNotConfigured) {
		t.Fatalf("PrintImage() error = %v, want ErrUserIDNotConfigured", err)
	}
	if serverCalls.Load() != 0 {
		t.Fatalf("serverCalls = %d, want 0", serverCalls.Load())
	}
}

func TestClient_BindAndRememberStoresUserID(t *testing.T) {
	server := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"showapi_res_code":1,"showapi_userid":456}`))
	})
	defer server.Close()

	client := newTestClient(server.URL)
	client.SetUserID(0)

	resp, err := client.BindAndRemember(context.Background(), "test-user")
	if err != nil {
		t.Fatalf("BindAndRemember() error = %v", err)
	}
	if resp.UserID != 456 {
		t.Fatalf("resp.UserID = %d, want 456", resp.UserID)
	}
	if client.GetUserID() != 456 {
		t.Fatalf("client.GetUserID() = %d, want 456", client.GetUserID())
	}
}
