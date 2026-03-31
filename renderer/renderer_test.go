package renderer

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantError   bool
		errorSubstr string
	}{
		{name: "valid HTTPS URL", url: "https://example.com", wantError: false},
		{name: "valid HTTP URL", url: "http://example.com", wantError: false},
		{name: "valid HTTPS URL with path", url: "https://example.com/path/to/page", wantError: false},
		{name: "invalid URL format", url: "not-a-valid-url", wantError: true, errorSubstr: "unsupported URL scheme"},
		{name: "FTP protocol", url: "ftp://example.com", wantError: true, errorSubstr: "unsupported URL scheme"},
		{name: "file protocol", url: "file:///etc/passwd", wantError: true, errorSubstr: "unsupported URL scheme"},
		{name: "no scheme", url: "example.com", wantError: true, errorSubstr: "unsupported URL scheme"},
		{name: "localhost is allowed", url: "http://localhost:8080", wantError: false},
		{name: "missing host in absolute URL", url: "https:///missing-host", wantError: true, errorSubstr: "host is required"},
		{name: "hostless HTTP URL", url: "http:///path-only", wantError: true, errorSubstr: "host is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url)
			if (err != nil) != tt.wantError {
				t.Fatalf("ValidateURL() error = %v, wantError %v", err, tt.wantError)
			}
			if tt.errorSubstr != "" && (err == nil || !strings.Contains(err.Error(), tt.errorSubstr)) {
				t.Fatalf("ValidateURL() error = %v, want substring %q", err, tt.errorSubstr)
			}
		})
	}
}

func TestNewRenderer(t *testing.T) {
	tests := []struct {
		name        string
		timeout     time.Duration
		wantTimeout time.Duration
	}{
		{name: "custom timeout", timeout: 60 * time.Second, wantTimeout: 60 * time.Second},
		{name: "zero timeout uses default", timeout: 0, wantTimeout: 30 * time.Second},
		{name: "negative timeout", timeout: -10 * time.Second, wantTimeout: -10 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New(tt.timeout)
			if r.timeout != tt.wantTimeout {
				t.Errorf("timeout = %v, want %v", r.timeout, tt.wantTimeout)
			}
			if r.renderDelay != 500*time.Millisecond {
				t.Errorf("renderDelay = %v, want %v", r.renderDelay, 500*time.Millisecond)
			}
		})
	}
}

func TestNewRendererWithOptions(t *testing.T) {
	tests := []struct {
		name            string
		timeout         time.Duration
		renderDelay     time.Duration
		wantTimeout     time.Duration
		wantRenderDelay time.Duration
	}{
		{name: "custom options", timeout: 45 * time.Second, renderDelay: 200 * time.Millisecond, wantTimeout: 45 * time.Second, wantRenderDelay: 200 * time.Millisecond},
		{name: "zero timeout uses default", timeout: 0, renderDelay: 100 * time.Millisecond, wantTimeout: 30 * time.Second, wantRenderDelay: 100 * time.Millisecond},
		{name: "zero delay uses default", timeout: 20 * time.Second, renderDelay: 0, wantTimeout: 20 * time.Second, wantRenderDelay: 500 * time.Millisecond},
		{name: "both zero use defaults", timeout: 0, renderDelay: 0, wantTimeout: 30 * time.Second, wantRenderDelay: 500 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewWithOptions(tt.timeout, tt.renderDelay)
			if r.timeout != tt.wantTimeout {
				t.Errorf("timeout = %v, want %v", r.timeout, tt.wantTimeout)
			}
			if r.renderDelay != tt.wantRenderDelay {
				t.Errorf("renderDelay = %v, want %v", r.renderDelay, tt.wantRenderDelay)
			}
		})
	}
}

func TestPrinterWidth(t *testing.T) {
	if PrinterWidth != 400 {
		t.Errorf("PrinterWidth = %d, want 400", PrinterWidth)
	}
}

func TestRenderBoundaryConstants(t *testing.T) {
	if MaxRenderHeight != 2000 {
		t.Errorf("MaxRenderHeight = %d, want 2000", MaxRenderHeight)
	}

	if MaxRenderPixels != PrinterWidth*MaxRenderHeight {
		t.Errorf("MaxRenderPixels = %d, want %d", MaxRenderPixels, PrinterWidth*MaxRenderHeight)
	}
}

func TestValidateRenderBounds(t *testing.T) {
	tests := []struct {
		name        string
		width       int
		height      int
		wantError   bool
		errorSubstr string
	}{
		{name: "within limits", width: PrinterWidth, height: MaxRenderHeight, wantError: false},
		{name: "height exceeds limit", width: PrinterWidth, height: MaxRenderHeight + 1, wantError: true, errorSubstr: "render height"},
		{name: "pixel count exceeds limit", width: PrinterWidth + 1, height: MaxRenderHeight, wantError: true, errorSubstr: "render area"},
		{name: "invalid zero width", width: 0, height: 100, wantError: true, errorSubstr: "invalid render bounds"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRenderBounds(tt.width, tt.height)
			if (err != nil) != tt.wantError {
				t.Fatalf("validateRenderBounds() error = %v, wantError %v", err, tt.wantError)
			}
			if tt.errorSubstr != "" && (err == nil || !strings.Contains(err.Error(), tt.errorSubstr)) {
				t.Fatalf("validateRenderBounds() error = %v, want substring %q", err, tt.errorSubstr)
			}
		})
	}
}

func TestSplitRenderBounds(t *testing.T) {
	tests := []struct {
		name        string
		bounds      renderBounds
		want        []renderSegment
		errorSubstr string
	}{
		{
			name:   "single segment within limits",
			bounds: renderBounds{Width: PrinterWidth, Height: 1200},
			want:   []renderSegment{{OffsetY: 0, Height: 1200}},
		},
		{
			name:   "multiple segments for long page",
			bounds: renderBounds{Width: PrinterWidth, Height: 2500},
			want:   []renderSegment{{OffsetY: 0, Height: 2000}, {OffsetY: 2000, Height: 500}},
		},
		{
			name:   "width lowers segment height",
			bounds: renderBounds{Width: 500, Height: 1884},
			want:   []renderSegment{{OffsetY: 0, Height: 1600}, {OffsetY: 1600, Height: 284}},
		},
		{
			name:        "invalid dimensions",
			bounds:      renderBounds{Width: 0, Height: 10},
			errorSubstr: "invalid render bounds",
		},
		{
			name:        "unsupported width",
			bounds:      renderBounds{Width: MaxRenderPixels + 1, Height: 1},
			errorSubstr: "render width",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments, err := splitRenderBounds(tt.bounds)
			if tt.errorSubstr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errorSubstr) {
					t.Fatalf("splitRenderBounds() error = %v, want substring %q", err, tt.errorSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("splitRenderBounds() error = %v", err)
			}
			if len(segments) != len(tt.want) {
				t.Fatalf("len(segments) = %d, want %d", len(segments), len(tt.want))
			}
			for i := range tt.want {
				if segments[i] != tt.want[i] {
					t.Fatalf("segments[%d] = %+v, want %+v", i, segments[i], tt.want[i])
				}
			}
		})
	}
}

func TestSingleImageResult(t *testing.T) {
	if _, err := singleImageResult(nil); err == nil {
		t.Fatal("expected empty image set to fail")
	}
	if got, err := singleImageResult([]string{"one"}); err != nil || got != "one" {
		t.Fatalf("singleImageResult() = (%q, %v), want (%q, nil)", got, err, "one")
	}
	if _, err := singleImageResult([]string{"one", "two"}); err == nil || !strings.Contains(err.Error(), "pagination") {
		t.Fatalf("singleImageResult() error = %v, want pagination error", err)
	}
}

func TestRendererClose(t *testing.T) {
	r := New(5 * time.Second)

	r.Close()
	r.Close()

	if _, err := r.getBrowserContext(urlRendererKind); err == nil {
		t.Error("expected getBrowserContext(url) to fail after close")
	}
	if _, err := r.getBrowserContext(htmlRendererKind); err == nil {
		t.Error("expected getBrowserContext(html) to fail after close")
	}
}

func TestSessionForKind(t *testing.T) {
	r := New(5 * time.Second)

	if _, _, err := r.sessionForKind("unknown"); err == nil {
		t.Fatal("expected unknown renderer kind to fail")
	}
}

func TestNewTaskContextCancelsWithRequest(t *testing.T) {
	r := New(5 * time.Second)
	requestCtx, requestCancel := context.WithCancel(context.Background())
	defer requestCancel()

	taskCtx, cancel := r.newTaskContext(context.Background(), requestCtx)
	defer cancel()

	requestCancel()

	select {
	case <-taskCtx.Done():
	case <-time.After(500 * time.Millisecond):
		t.Fatal("task context did not cancel after request context cancellation")
	}
}

func BenchmarkProcessImageForPrint(b *testing.B) {
	input, err := createTestImageBenchmark(800, 600)
	if err != nil {
		b.Fatalf("failed to create test image: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ProcessImageForPrint(input)
	}
}

func createTestImageBenchmark(width, height int) (string, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			gray := uint8((x + y) * 255 / (width + height))
			img.Set(x, y, color.RGBA{gray, gray, gray, 255})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
