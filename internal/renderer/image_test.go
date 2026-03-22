package renderer

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

// createTestImage creates a simple test PNG image.
func createTestImage(width, height int) (string, error) {
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

// TestProcessImageForPrint verifies end-to-end image preprocessing for printer output.
func TestProcessImageForPrint(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		height    int
		wantError bool
	}{
		{name: "standard image", width: 800, height: 600, wantError: false},
		{name: "small image", width: 100, height: 100, wantError: false},
		{name: "large image", width: 1920, height: 1080, wantError: false},
		{name: "tall image", width: 400, height: 2000, wantError: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := createTestImage(tt.width, tt.height)
			if err != nil {
				t.Fatalf("failed to create test image: %v", err)
			}

			output, err := ProcessImageForPrint(input)
			if (err != nil) != tt.wantError {
				t.Fatalf("ProcessImageForPrint() error = %v, wantError %v", err, tt.wantError)
			}

			if tt.wantError {
				return
			}

			data, err := base64.StdEncoding.DecodeString(output)
			if err != nil {
				t.Fatalf("output is not valid base64: %v", err)
			}

			img, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("output is not valid PNG: %v", err)
			}

			bounds := img.Bounds()
			if bounds.Dx() != TargetWidth {
				t.Fatalf("output width = %d, want %d", bounds.Dx(), TargetWidth)
			}

			expectedHeight := (TargetWidth * tt.height) / tt.width
			if bounds.Dy() < 1 || abs(bounds.Dy()-expectedHeight) > 2 {
				t.Fatalf("output height = %d, expected approximately %d", bounds.Dy(), expectedHeight)
			}

			if _, ok := img.(*image.Gray); !ok {
				t.Fatalf("output is not grayscale (image.Gray)")
			}
		})
	}
}

// TestProcessImageForPrintInvalidInput verifies malformed payloads are rejected.
func TestProcessImageForPrintInvalidInput(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantError   bool
		errorSubstr string
	}{
		{
			name:        "invalid base64",
			input:       "not-valid-base64!!!",
			wantError:   true,
			errorSubstr: "failed to decode base64",
		},
		{
			name:        "valid base64 but not PNG",
			input:       base64.StdEncoding.EncodeToString([]byte("not a png")),
			wantError:   true,
			errorSubstr: "failed to decode PNG config",
		},
		{
			name:        "empty string",
			input:       "",
			wantError:   true,
			errorSubstr: "payload is empty",
		},
		{
			name:        "oversized encoded payload",
			input:       strings.Repeat("A", MaxEncodedImageLength+1),
			wantError:   true,
			errorSubstr: "encoded image payload exceeds max",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ProcessImageForPrint(tt.input)
			if (err != nil) != tt.wantError {
				t.Fatalf("ProcessImageForPrint() error = %v, wantError %v", err, tt.wantError)
			}
			if tt.errorSubstr != "" && (err == nil || !strings.Contains(err.Error(), tt.errorSubstr)) {
				t.Fatalf("ProcessImageForPrint() error = %v, want substring %q", err, tt.errorSubstr)
			}
		})
	}
}

func TestProcessImageForPrintRejectsExcessiveOutputHeight(t *testing.T) {
	input, err := createTestImage(1, 10)
	if err != nil {
		t.Fatalf("failed to create test image: %v", err)
	}

	_, err = ProcessImageForPrint(input)
	if err == nil {
		t.Fatal("expected excessive output height to fail")
	}
	if !strings.Contains(err.Error(), "processed image height") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTrimTrailingWhitespaceRemovesBlankTail(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 8; x++ {
			c := color.RGBA{255, 255, 255, 255}
			if y < 4 {
				c = color.RGBA{0, 0, 0, 255}
			}
			img.Set(x, y, c)
		}
	}

	trimmed := trimTrailingWhitespace(img)
	if trimmed.Bounds().Dy() != 4 {
		t.Fatalf("trimmed height = %d, want 4", trimmed.Bounds().Dy())
	}
}

func TestProcessImageForPrintTrimsTrailingWhitespace(t *testing.T) {
	input, err := createImageWithTrailingWhitespace(100, 100, 40)
	if err != nil {
		t.Fatalf("createImageWithTrailingWhitespace() error = %v", err)
	}

	output, err := ProcessImageForPrint(input)
	if err != nil {
		t.Fatalf("ProcessImageForPrint() error = %v", err)
	}

	data, err := base64.StdEncoding.DecodeString(output)
	if err != nil {
		t.Fatalf("DecodeString() error = %v", err)
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("png.Decode() error = %v", err)
	}

	naiveHeight := (TargetWidth * 100) / 100
	if img.Bounds().Dy() >= naiveHeight {
		t.Fatalf("trimmed height = %d, want less than %d after whitespace crop", img.Bounds().Dy(), naiveHeight)
	}
}

func TestValidateSourceImageBounds(t *testing.T) {
	tests := []struct {
		name        string
		width       int
		height      int
		wantError   bool
		errorSubstr string
	}{
		{name: "valid bounds", width: 100, height: 200, wantError: false},
		{name: "invalid width", width: 0, height: 10, wantError: true, errorSubstr: "invalid image dimensions"},
		{name: "pixel limit exceeded", width: 4000, height: 3000, wantError: true, errorSubstr: "source image area"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSourceImageBounds(tt.width, tt.height)
			if (err != nil) != tt.wantError {
				t.Fatalf("validateSourceImageBounds() error = %v, wantError %v", err, tt.wantError)
			}
			if tt.errorSubstr != "" && (err == nil || !strings.Contains(err.Error(), tt.errorSubstr)) {
				t.Fatalf("validateSourceImageBounds() error = %v, want substring %q", err, tt.errorSubstr)
			}
		})
	}
}

func TestValidateProcessedImageBounds(t *testing.T) {
	tests := []struct {
		name        string
		height      int
		wantError   bool
		errorSubstr string
	}{
		{name: "valid height", height: 100, wantError: false},
		{name: "invalid height", height: 0, wantError: true, errorSubstr: "invalid processed image height"},
		{name: "height exceeds limit", height: MaxRenderHeight + 1, wantError: true, errorSubstr: "processed image height"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProcessedImageBounds(tt.height)
			if (err != nil) != tt.wantError {
				t.Fatalf("validateProcessedImageBounds() error = %v, wantError %v", err, tt.wantError)
			}
			if tt.errorSubstr != "" && (err == nil || !strings.Contains(err.Error(), tt.errorSubstr)) {
				t.Fatalf("validateProcessedImageBounds() error = %v, want substring %q", err, tt.errorSubstr)
			}
		})
	}
}

// TestDitherToMonochrome verifies dithering produces stable monochrome output.
func TestDitherToMonochrome(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
	}{
		{name: "white image", pattern: "white"},
		{name: "black image", pattern: "black"},
		{name: "gradient", pattern: "gradient"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			width, height := 100, 100
			img := image.NewRGBA(image.Rect(0, 0, width, height))

			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					var c color.RGBA
					switch tt.pattern {
					case "white":
						c = color.RGBA{255, 255, 255, 255}
					case "black":
						c = color.RGBA{0, 0, 0, 255}
					case "gradient":
						gray := uint8((x + y) * 255 / (width + height))
						c = color.RGBA{gray, gray, gray, 255}
					}
					img.Set(x, y, c)
				}
			}

			result := ditherToMonochrome(img)
			if result.Bounds().Dx() != width || result.Bounds().Dy() != height {
				t.Fatalf("output dimensions mismatch: got %dx%d, want %dx%d", result.Bounds().Dx(), result.Bounds().Dy(), width, height)
			}

			if tt.pattern == "white" {
				for y := 0; y < height; y++ {
					for x := 0; x < width; x++ {
						if result.GrayAt(x, y).Y != 255 {
							t.Fatalf("white image produced non-white pixel at (%d,%d): %d", x, y, result.GrayAt(x, y).Y)
						}
					}
				}
			}
			if tt.pattern == "black" {
				for y := 0; y < height; y++ {
					for x := 0; x < width; x++ {
						if result.GrayAt(x, y).Y != 0 {
							t.Fatalf("black image produced non-black pixel at (%d,%d): %d", x, y, result.GrayAt(x, y).Y)
						}
					}
				}
			}

			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					v := result.GrayAt(x, y).Y
					if v != 0 && v != 255 {
						t.Fatalf("monochrome image has intermediate value at (%d,%d): %d", x, y, v)
					}
				}
			}
		})
	}
}

// TestTargetWidth verifies the processed image width matches printer requirements.
func TestTargetWidth(t *testing.T) {
	if TargetWidth != 384 {
		t.Errorf("TargetWidth = %d, want 384", TargetWidth)
	}
}

func TestImageLimits(t *testing.T) {
	if MaxEncodedImageLength <= MaxDecodedImageBytes {
		t.Fatalf("expected encoded limit to exceed decoded limit")
	}
	if MaxSourceImagePixels <= 0 {
		t.Fatalf("MaxSourceImagePixels = %d, want positive", MaxSourceImagePixels)
	}
}

// abs returns the absolute value of an integer.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func createImageWithTrailingWhitespace(width, height, contentRows int) (string, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := color.RGBA{255, 255, 255, 255}
			if y < contentRows {
				c = color.RGBA{0, 0, 0, 255}
			}
			img.Set(x, y, c)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
