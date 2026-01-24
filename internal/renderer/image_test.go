package renderer

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// createTestImage creates a simple test PNG image
func createTestImage(width, height int) (string, error) {
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

func TestProcessImageForPrint(t *testing.T) {
	tests := []struct {
		name        string
		width       int
		height      int
		wantError   bool
		description string
	}{
		{
			name:        "standard image",
			width:       800,
			height:      600,
			wantError:   false,
			description: "should successfully process standard size image",
		},
		{
			name:        "small image",
			width:       100,
			height:      100,
			wantError:   false,
			description: "should successfully process small image",
		},
		{
			name:        "large image",
			width:       1920,
			height:      1080,
			wantError:   false,
			description: "should successfully process large image",
		},
		{
			name:        "tall image",
			width:       400,
			height:      2000,
			wantError:   false,
			description: "should successfully process tall image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := createTestImage(tt.width, tt.height)
			if err != nil {
				t.Fatalf("failed to create test image: %v", err)
			}

			output, err := ProcessImageForPrint(input)
			if (err != nil) != tt.wantError {
				t.Errorf("%s: ProcessImageForPrint() error = %v, wantError %v", tt.description, err, tt.wantError)
				return
			}

			if !tt.wantError {
				// Verify output is valid base64
				data, err := base64.StdEncoding.DecodeString(output)
				if err != nil {
					t.Errorf("%s: output is not valid base64: %v", tt.description, err)
				}

				// Verify output is valid PNG
				img, err := png.Decode(bytes.NewReader(data))
				if err != nil {
					t.Errorf("%s: output is not valid PNG: %v", tt.description, err)
				}

				// Verify output dimensions
				bounds := img.Bounds()
				if bounds.Dx() != TargetWidth {
					t.Errorf("%s: output width = %d, want %d", tt.description, bounds.Dx(), TargetWidth)
				}

				// Verify aspect ratio is approximately maintained
				expectedHeight := (TargetWidth * tt.height) / tt.width
				if bounds.Dy() < 1 || abs(bounds.Dy()-expectedHeight) > 2 {
					t.Errorf("%s: output height = %d, expected approximately %d", tt.description, bounds.Dy(), expectedHeight)
				}

				// Verify output is grayscale
				if _, ok := img.(*image.Gray); !ok {
					t.Errorf("%s: output is not grayscale (image.Gray)", tt.description)
				}
			}
		})
	}
}

func TestProcessImageForPrintInvalidInput(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantError   bool
		description string
	}{
		{
			name:        "invalid base64",
			input:       "not-valid-base64!!!",
			wantError:   true,
			description: "should fail with invalid base64",
		},
		{
			name:        "valid base64 but not PNG",
			input:       base64.StdEncoding.EncodeToString([]byte("not a png")),
			wantError:   true,
			description: "should fail with non-PNG data",
		},
		{
			name:        "empty string",
			input:       "",
			wantError:   true,
			description: "should fail with empty input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ProcessImageForPrint(tt.input)
			if (err != nil) != tt.wantError {
				t.Errorf("%s: ProcessImageForPrint() error = %v, wantError %v", tt.description, err, tt.wantError)
			}
		})
	}
}

func TestDitherToMonochrome(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		description string
	}{
		{
			name:        "white image",
			pattern:     "white",
			description: "all pixels should be white (255)",
		},
		{
			name:        "black image",
			pattern:     "black",
			description: "all pixels should be black (0)",
		},
		{
			name:        "gradient",
			pattern:     "gradient",
			description: "should apply dithering to gradient",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			width, height := 100, 100
			img := image.NewRGBA(image.Rect(0, 0, width, height))

			// Create test pattern
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

			// Verify output is grayscale
			if result.Bounds().Dx() != width || result.Bounds().Dy() != height {
				t.Errorf("output dimensions mismatch: got %dx%d, want %dx%d",
					result.Bounds().Dx(), result.Bounds().Dy(), width, height)
			}

			// For pure white/black input, verify output
			if tt.pattern == "white" {
				for y := 0; y < height; y++ {
					for x := 0; x < width; x++ {
						if result.GrayAt(x, y).Y != 255 {
							t.Errorf("white image produced non-white pixel at (%d,%d): %d", x, y, result.GrayAt(x, y).Y)
						}
					}
				}
			}
			if tt.pattern == "black" {
				for y := 0; y < height; y++ {
					for x := 0; x < width; x++ {
						if result.GrayAt(x, y).Y != 0 {
							t.Errorf("black image produced non-black pixel at (%d,%d): %d", x, y, result.GrayAt(x, y).Y)
						}
					}
				}
			}

			// Verify all pixels are either 0 or 255 (1-bit)
			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					v := result.GrayAt(x, y).Y
					if v != 0 && v != 255 {
						t.Errorf("monochrome image has intermediate value at (%d,%d): %d", x, y, v)
					}
				}
			}
		})
	}
}

func TestTargetWidth(t *testing.T) {
	if TargetWidth != 384 {
		t.Errorf("TargetWidth = %d, want 384", TargetWidth)
	}
}

// abs returns the absolute value of an integer.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
