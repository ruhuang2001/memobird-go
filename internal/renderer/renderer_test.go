package renderer

import (
	"bytes"
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
		description string
	}{
		{
			name:        "valid HTTPS URL",
			url:         "https://example.com",
			wantError:   false,
			description: "should accept valid HTTPS URL",
		},
		{
			name:        "valid HTTP URL",
			url:         "http://example.com",
			wantError:   false,
			description: "should accept valid HTTP URL",
		},
		{
			name:        "valid HTTPS URL with path",
			url:         "https://example.com/path/to/page",
			wantError:   false,
			description: "should accept valid URL with path",
		},
		{
			name:        "invalid URL format",
			url:         "not-a-valid-url",
			wantError:   true,
			description: "should reject invalid URL format",
		},
		{
			name:        "FTP protocol",
			url:         "ftp://example.com",
			wantError:   true,
			description: "should reject non-HTTP protocols",
		},
		{
			name:        "file protocol",
			url:         "file:///etc/passwd",
			wantError:   true,
			description: "should reject file:// protocol",
		},
		{
			name:        "no scheme",
			url:         "example.com",
			wantError:   true,
			description: "should reject URL without scheme",
		},
		{
			name:        "localhost is allowed",
			url:         "http://localhost:8080",
			wantError:   false,
			description: "should accept localhost for development",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url)
			if (err != nil) != tt.wantError {
				t.Errorf("%s: ValidateURL() error = %v, wantError %v", tt.description, err, tt.wantError)
			}
		})
	}
}

func TestNewRenderer(t *testing.T) {
	tests := []struct {
		name        string
		timeout     time.Duration
		wantTimeout time.Duration
		description string
	}{
		{
			name:        "custom timeout",
			timeout:     60 * time.Second,
			wantTimeout: 60 * time.Second,
			description: "should use provided timeout",
		},
		{
			name:        "zero timeout uses default",
			timeout:     0,
			wantTimeout: 30 * time.Second,
			description: "should use default timeout when zero is provided",
		},
		{
			name:        "negative timeout",
			timeout:     -10 * time.Second,
			wantTimeout: -10 * time.Second,
			description: "should accept negative timeout (edge case)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := New(tt.timeout)
			if r.timeout != tt.wantTimeout {
				t.Errorf("%s: timeout = %v, want %v", tt.description, r.timeout, tt.wantTimeout)
			}
			if r.renderDelay != 500*time.Millisecond {
				t.Errorf("%s: renderDelay = %v, want %v", tt.description, r.renderDelay, 500*time.Millisecond)
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
		description     string
	}{
		{
			name:            "custom options",
			timeout:         45 * time.Second,
			renderDelay:     200 * time.Millisecond,
			wantTimeout:     45 * time.Second,
			wantRenderDelay: 200 * time.Millisecond,
			description:     "should use provided options",
		},
		{
			name:            "zero timeout uses default",
			timeout:         0,
			renderDelay:     100 * time.Millisecond,
			wantTimeout:     30 * time.Second,
			wantRenderDelay: 100 * time.Millisecond,
			description:     "should use default timeout when zero",
		},
		{
			name:            "zero delay uses default",
			timeout:         20 * time.Second,
			renderDelay:     0,
			wantTimeout:     20 * time.Second,
			wantRenderDelay: 500 * time.Millisecond,
			description:     "should use default delay when zero",
		},
		{
			name:            "both zero use defaults",
			timeout:         0,
			renderDelay:     0,
			wantTimeout:     30 * time.Second,
			wantRenderDelay: 500 * time.Millisecond,
			description:     "should use defaults when both are zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewWithOptions(tt.timeout, tt.renderDelay)
			if r.timeout != tt.wantTimeout {
				t.Errorf("%s: timeout = %v, want %v", tt.description, r.timeout, tt.wantTimeout)
			}
			if r.renderDelay != tt.wantRenderDelay {
				t.Errorf("%s: renderDelay = %v, want %v", tt.description, r.renderDelay, tt.wantRenderDelay)
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
		{
			name:      "within limits",
			width:     PrinterWidth,
			height:    MaxRenderHeight,
			wantError: false,
		},
		{
			name:        "height exceeds limit",
			width:       PrinterWidth,
			height:      MaxRenderHeight + 1,
			wantError:   true,
			errorSubstr: "render height",
		},
		{
			name:        "pixel count exceeds limit",
			width:       PrinterWidth + 1,
			height:      MaxRenderHeight,
			wantError:   true,
			errorSubstr: "render area",
		},
		{
			name:        "invalid zero width",
			width:       0,
			height:      100,
			wantError:   true,
			errorSubstr: "invalid render bounds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRenderBounds(tt.width, tt.height)
			if (err != nil) != tt.wantError {
				t.Errorf("validateRenderBounds() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if tt.errorSubstr != "" && (err == nil || !strings.Contains(err.Error(), tt.errorSubstr)) {
				t.Errorf("validateRenderBounds() error = %v, want substring %q", err, tt.errorSubstr)
			}
		})
	}
}

func TestRendererClose(t *testing.T) {
	r := New(5 * time.Second)

	r.Close()
	r.Close()

	if _, err := r.getURLBrowserContext(); err == nil {
		t.Error("expected getURLBrowserContext to fail after close")
	}

	if _, err := r.getHTMLBrowserContext(); err == nil {
		t.Error("expected getHTMLBrowserContext to fail after close")
	}
}

// Benchmark for ProcessImageForPrint
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

// createTestImageBenchmark creates a simple test PNG image for benchmarking
func createTestImageBenchmark(width, height int) (string, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Create a gradient pattern
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
