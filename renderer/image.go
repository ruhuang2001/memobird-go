package renderer

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"

	"golang.org/x/image/draw"
)

const (
	// TargetWidth is the target width for Memobird thermal printer output.
	TargetWidth = 384

	// MaxEncodedImageLength bounds the incoming base64 payload size before decoding.
	MaxEncodedImageLength = 12 * 1024 * 1024

	// MaxDecodedImageBytes bounds the decoded PNG payload size in memory.
	MaxDecodedImageBytes = 8 * 1024 * 1024

	// MaxSourceImagePixels bounds source image size before resize/dither work begins.
	MaxSourceImagePixels = 8_000_000

	// TrailingWhiteRowLumaThreshold controls bottom-whitespace cropping for paginated screenshots.
	TrailingWhiteRowLumaThreshold = 250
)

// ProcessImageForPrint takes a base64 PNG, resizes to 384px width, and converts to 1-bit monochrome.
func ProcessImageForPrint(imgBase64 string) (string, error) {
	if imgBase64 == "" {
		return "", fmt.Errorf("image payload is empty")
	}
	if len(imgBase64) > MaxEncodedImageLength {
		return "", fmt.Errorf("encoded image payload exceeds max %d bytes", MaxEncodedImageLength)
	}

	imgData, err := base64.StdEncoding.DecodeString(imgBase64)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}
	if len(imgData) > MaxDecodedImageBytes {
		return "", fmt.Errorf("decoded image payload exceeds max %d bytes", MaxDecodedImageBytes)
	}

	cfg, err := png.DecodeConfig(bytes.NewReader(imgData))
	if err != nil {
		return "", fmt.Errorf("failed to decode PNG config: %w", err)
	}
	if err := validateSourceImageBounds(cfg.Width, cfg.Height); err != nil {
		return "", err
	}

	src, err := png.Decode(bytes.NewReader(imgData))
	if err != nil {
		return "", fmt.Errorf("failed to decode PNG: %w", err)
	}

	bounds := src.Bounds()
	srcWidth := bounds.Dx()
	srcHeight := bounds.Dy()
	newHeight := (TargetWidth * srcHeight) / srcWidth
	if err := validateProcessedImageBounds(newHeight); err != nil {
		return "", err
	}

	dst := image.NewRGBA(image.Rect(0, 0, TargetWidth, newHeight))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)
	dst = trimTrailingWhitespace(dst)
	mono := ditherToMonochrome(dst)

	var buf bytes.Buffer
	if err := png.Encode(&buf, mono); err != nil {
		return "", fmt.Errorf("failed to encode PNG: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func validateSourceImageBounds(width, height int) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid image dimensions: %dx%d", width, height)
	}

	pixelCount := int64(width) * int64(height)
	if pixelCount > MaxSourceImagePixels {
		return fmt.Errorf("source image area %dpx exceeds max %dpx (%dx%d)", pixelCount, MaxSourceImagePixels, width, height)
	}

	return nil
}

func validateProcessedImageBounds(height int) error {
	if height <= 0 {
		return fmt.Errorf("invalid processed image height: %d", height)
	}
	if height > MaxRenderHeight {
		return fmt.Errorf("processed image height %dpx exceeds max %dpx", height, MaxRenderHeight)
	}

	return nil
}

func trimTrailingWhitespace(img *image.RGBA) *image.RGBA {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w == 0 || h == 0 {
		return img
	}

	keepHeight := h
	for keepHeight > 1 && isTrailingWhiteRow(img, keepHeight-1, w) {
		keepHeight--
	}
	if keepHeight == h {
		return img
	}

	trimmed := image.NewRGBA(image.Rect(0, 0, w, keepHeight))
	for y := 0; y < keepHeight; y++ {
		srcStart := y * img.Stride
		srcEnd := srcStart + w*4
		dstStart := y * trimmed.Stride
		copy(trimmed.Pix[dstStart:dstStart+w*4], img.Pix[srcStart:srcEnd])
	}

	return trimmed
}

func isTrailingWhiteRow(img *image.RGBA, y, width int) bool {
	rowStart := y * img.Stride
	for x := 0; x < width; x++ {
		offset := rowStart + x*4
		r := img.Pix[offset]
		g := img.Pix[offset+1]
		b := img.Pix[offset+2]
		lum := 0.299*float32(r) + 0.587*float32(g) + 0.114*float32(b)
		if lum < TrailingWhiteRowLumaThreshold {
			return false
		}
	}

	return true
}

// ditherToMonochrome converts image to 1-bit using Floyd-Steinberg dithering.
func ditherToMonochrome(img *image.RGBA) *image.Gray {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	stride := w + 2

	gray := make([]float32, h*stride)
	for y := 0; y < h; y++ {
		base := y * stride
		pixBase := y * img.Stride
		for x := 0; x < w; x++ {
			offset := pixBase + x*4
			r := img.Pix[offset]
			g := img.Pix[offset+1]
			b := img.Pix[offset+2]
			lum := 0.299*float32(r) + 0.587*float32(g) + 0.114*float32(b)
			gray[base+x+1] = lum
		}
	}

	result := image.NewGray(bounds)

	for y := 0; y < h; y++ {
		row := y * stride
		nextRow := row + stride
		for x := 0; x < w; x++ {
			idx := row + x + 1
			oldPixel := gray[idx]
			var newPixel float32
			if oldPixel > 128 {
				newPixel = 255
			} else {
				newPixel = 0
			}
			result.SetGray(x, y, color.Gray{Y: uint8(newPixel)})

			err := oldPixel - newPixel

			gray[idx+1] += err * 7 / 16
			if y+1 < h {
				gray[nextRow+x] += err * 3 / 16
				gray[nextRow+x+1] += err * 5 / 16
				gray[nextRow+x+2] += err * 1 / 16
			}
		}
	}

	return result
}
