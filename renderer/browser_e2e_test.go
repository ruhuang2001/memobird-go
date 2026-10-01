package renderer

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// Failure scenarios, recorded before changing the implementation:
//   - Sequential and concurrent renders allocate independent Chrome processes.
//   - Canceling a request shuts down the shared browser or poisons the next render.
//   - A canceled cold-start request continues allocating Chrome until the renderer timeout.
//   - Browser startup failure or a dead session prevents later requests from recovering.
//   - Inline scripts, external scripts, or the load event never run before capture.
//   - Long URL pages lose their final content when split into PNGs.
//
// Run with MEMOBIRD_E2E=1 go test -race ./renderer -run TestBrowserE2E -count=1 -v.
// PNG evidence is retained in build/review-e2e (or MEMOBIRD_E2E_ARTIFACTS).
func TestBrowserE2E(t *testing.T) {
	if os.Getenv("MEMOBIRD_E2E") != "1" {
		t.Skip("set MEMOBIRD_E2E=1 to run real Chrome E2E")
	}
	artifactDir := os.Getenv("MEMOBIRD_E2E_ARTIFACTS")
	if artifactDir == "" {
		artifactDir = filepath.Join("..", "build", "review-e2e")
	}
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatal(err)
	}
	save := func(t *testing.T, name, encoded string) image.Image {
		t.Helper()
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(artifactDir, name+".png"), data, 0o644); err != nil {
			t.Fatal(err)
		}
		return img
	}
	checkColor := func(t *testing.T, img image.Image, x, y int, want color.NRGBA) {
		t.Helper()
		if got := color.NRGBAModel.Convert(img.At(x, y)); got != want {
			t.Fatalf("pixel (%d,%d) = %v; want %v", x, y, got, want)
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /script.js", func(w http.ResponseWriter, r *http.Request) {
		// Longer than renderDelay: navigation must wait for resource loading.
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/javascript")
		fmt.Fprint(w, `document.body.insertAdjacentHTML('beforeend', '<div style="position:fixed;left:100px;top:0;width:100px;height:100px;background:rgb(0,255,0)"></div>');`)
	})
	mux.HandleFunc("GET /long", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><body style="height:2700px;background:linear-gradient(red 0 2000px, blue 2000px)"><div>Local pagination E2E</div><script src="/script.js"></script></body></html>`)
	})
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	renderer := NewWithOptions(15*time.Second, 20*time.Millisecond)
	t.Cleanup(renderer.Close)
	externalScript := "data:text/javascript;base64," + base64.StdEncoding.EncodeToString([]byte(`document.body.insertAdjacentHTML('beforeend', '<div style="position:fixed;left:100px;top:0;width:100px;height:100px;background:rgb(0,255,0)"></div>');`))
	html := fmt.Sprintf(`<script>window.addEventListener('load', () => { document.body.insertAdjacentHTML('beforeend', '<div style="position:fixed;left:0;top:0;width:100px;height:100px;background:red"></div>'); });</script><script src="%s"></script>`, externalScript)
	var shared *chromedp.Browser

	t.Run("scripts_and_sequential_reuse", func(t *testing.T) {
		for i := range 2 {
			encoded, err := renderer.RenderHTMLToImage(t.Context(), html)
			if err != nil {
				t.Fatal(err)
			}
			img := save(t, fmt.Sprintf("html-script-%d", i), encoded)
			checkColor(t, img, 20, 20, color.NRGBA{R: 255, A: 255})
			checkColor(t, img, 120, 20, color.NRGBA{G: 255, A: 255})
			browser := chromedp.FromContext(renderer.htmlSession.browserCtx).Browser
			if browser == nil {
				t.Fatal("shared parent browser was never initialized")
			}
			if shared != nil && shared != browser {
				t.Fatal("sequential render replaced the shared browser")
			}
			shared = browser
		}
	})
	t.Run("concurrent_reuse", func(t *testing.T) {
		type result struct {
			image string
			err   error
		}
		results := make(chan result, 3)
		for range 3 {
			go func() {
				encoded, err := renderer.RenderHTMLToImage(t.Context(), html)
				results <- result{encoded, err}
			}()
		}
		for i := range 3 {
			result := <-results
			if result.err != nil {
				t.Error(result.err)
				continue
			}
			img := save(t, fmt.Sprintf("html-concurrent-%d", i), result.image)
			checkColor(t, img, 20, 20, color.NRGBA{R: 255, A: 255})
		}
		if browser := chromedp.FromContext(renderer.htmlSession.browserCtx).Browser; browser == nil || browser != shared {
			t.Fatal("concurrent renders did not reuse the shared browser")
		}
	})
	t.Run("pagination_and_request_cancellation", func(t *testing.T) {
		images, err := renderer.RenderURLToImages(t.Context(), server.URL+"/long")
		if err != nil {
			t.Fatal(err)
		}
		if len(images) < 2 {
			t.Fatalf("got %d pages; want multiple", len(images))
		}
		for i, encoded := range images {
			img := save(t, fmt.Sprintf("url-page-%d", i), encoded)
			if i == 0 {
				// URL rendering uses a device scale of two.
				checkColor(t, img, 240, 40, color.NRGBA{G: 255, A: 255})
			}
			if i == len(images)-1 {
				checkColor(t, img, 50, img.Bounds().Dy()-30, color.NRGBA{B: 255, A: 255})
			}
		}
		browser := chromedp.FromContext(renderer.urlSession.browserCtx).Browser
		request, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()
		if _, err := renderer.RenderURLToImages(request, server.URL+"/slow"); err == nil {
			t.Fatal("canceled request unexpectedly succeeded")
		}
		if _, err := renderer.RenderURLToImages(t.Context(), server.URL+"/long"); err != nil {
			t.Fatalf("render after cancellation: %v", err)
		}
		if browser == nil || chromedp.FromContext(renderer.urlSession.browserCtx).Browser != browser {
			t.Fatal("request cancellation replaced the browser")
		}
	})
	t.Run("dead_browser_recovery", func(t *testing.T) {
		if renderer.htmlSession == nil {
			t.Fatal("missing HTML session")
		}
		renderer.htmlSession.browserCancel()
		encoded, err := renderer.RenderHTMLToImage(t.Context(), html)
		if err != nil {
			t.Fatal(err)
		}
		checkColor(t, save(t, "html-recovered", encoded), 20, 20, color.NRGBA{R: 255, A: 255})
	})
	t.Run("cold_start_cancellation", func(t *testing.T) {
		r := NewWithOptions(15*time.Second, time.Millisecond)
		t.Cleanup(r.Close)
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
		defer cancel()
		started := time.Now()
		if _, err := r.RenderHTMLToImage(ctx, html); err == nil {
			t.Fatal("canceled cold start unexpectedly succeeded")
		}
		if elapsed := time.Since(started); elapsed > time.Second {
			t.Fatalf("canceled cold start took %v", elapsed)
		}
		if _, err := r.RenderHTMLToImage(t.Context(), html); err != nil {
			t.Fatalf("recovery after canceled cold start: %v", err)
		}
	})

	t.Run("startup_failure_recovery", func(t *testing.T) {
		r := NewWithOptions(-time.Second, time.Millisecond)
		t.Cleanup(r.Close)
		if _, err := r.RenderHTMLToImage(t.Context(), html); err == nil {
			t.Fatal("expired startup timeout unexpectedly succeeded")
		}
		r.timeout = 15 * time.Second
		encoded, err := r.RenderHTMLToImage(t.Context(), html)
		if err != nil {
			t.Fatal(err)
		}
		checkColor(t, save(t, "html-startup-recovered", encoded), 20, 20, color.NRGBA{R: 255, A: 255})
	})
}
