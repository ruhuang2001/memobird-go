package e2e

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ruhuang2001/memobird-go/memobird"
	"github.com/ruhuang2001/memobird-go/renderer"
	"github.com/ruhuang2001/memobird-go/storage"
)

func TestE2EPrintWorkflow(t *testing.T) {
	if os.Getenv("MEMOBIRD_E2E") != "1" {
		t.Skip("run make e2e with Chrome installed")
	}
	dir := os.Getenv("MEMOBIRD_E2E_ARTIFACTS")
	if dir == "" {
		dir = t.TempDir()
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	var converted, printed atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fail := func(err error) {
			t.Error(err)
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		if err := r.ParseForm(); err != nil {
			fail(err)
			return
		}
		if r.Method != http.MethodPost || r.Form.Get("ak") != "local-key" {
			fail(fmt.Errorf("invalid API request"))
			return
		}
		switch r.URL.Path {
		case "/home/setuserbind":
			fmt.Fprint(w, `{"showapi_res_code":1,"showapi_userid":42}`)
		case "/home/getSignalBase64Pic":
			data, err := base64.StdEncoding.DecodeString(r.Form.Get("imgBase64String"))
			if err != nil {
				fail(err)
				return
			}
			img, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				fail(err)
				return
			}
			if img.Bounds().Dx() != 384 || img.Bounds().Dy() > 2000 {
				fail(fmt.Errorf("unexpected print bounds: %v", img.Bounds()))
				return
			}
			index := converted.Add(1)
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("workflow-print-%d.png", index)), data, 0644); err != nil {
				fail(err)
				return
			}
			fmt.Fprint(w, `{"showapi_res_code":1,"result":"local-bitmap"}`)
		case "/home/printpaper":
			if r.Form.Get("userID") != "42" || r.Form.Get("memobirdID") != "local-device" || r.Form.Get("printcontent") != "P:local-bitmap" {
				fail(fmt.Errorf("invalid print submission: %v", r.Form))
				return
			}
			printed.Add(1)
			fmt.Fprint(w, `{"showapi_res_code":1,"result":1,"printcontentid":123}`)
		default:
			fail(fmt.Errorf("unexpected endpoint: %s", r.URL.Path))
		}
	}))
	t.Cleanup(server.Close)
	client, err := memobird.NewClient(memobird.Config{AccessKey: "local-key", DeviceID: "local-device", BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	store, err := storage.New(filepath.Join(t.TempDir(), "binding.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	if _, err := client.BindAndPersist(t.Context(), store, "local-user"); err != nil {
		t.Fatal(err)
	}
	client.SetUserID(0)
	if restored, err := client.RestoreUserBinding(t.Context(), store); err != nil || !restored {
		t.Fatalf("restore = %v, %v", restored, err)
	}
	render := renderer.New(20 * time.Second)
	t.Cleanup(render.Close)
	responses, err := client.PrintHTMLAsImages(t.Context(), render, `<h1>Local print receipt</h1><script>document.write('<p>Script generated content</p>')</script>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(responses) != 1 || converted.Load() != 1 || printed.Load() != 1 || responses[0].PrintContentID != 123 {
		t.Fatalf("unexpected workflow result: %v, conversions=%d submissions=%d", responses, converted.Load(), printed.Load())
	}
	receipt, err := json.MarshalIndent(responses, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "workflow-receipt.json"), receipt, 0644); err != nil {
		t.Fatal(err)
	}
}
