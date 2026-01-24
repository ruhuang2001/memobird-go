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
	// Target width for Memobird thermal printer
	TargetWidth = 384
)

// ProcessImageForPrint takes a base64 PNG, resizes to 384px width, and converts to 1-bit monochrome
func ProcessImageForPrint(imgBase64 string) (string, error) {
	imgData, err := base64.StdEncoding.DecodeString(imgBase64)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	src, err := png.Decode(bytes.NewReader(imgData))
	if err != nil {
		return "", fmt.Errorf("failed to decode PNG: %w", err)
	}

	// Calculate new dimensions maintaining aspect ratio
	bounds := src.Bounds()
	srcWidth := bounds.Dx()
	srcHeight := bounds.Dy()
	newHeight := (TargetWidth * srcHeight) / srcWidth

	// Resize image using high-quality interpolation
	dst := image.NewRGBA(image.Rect(0, 0, TargetWidth, newHeight))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)

	// Convert to 1-bit monochrome with Floyd-Steinberg dithering for better text
	mono := ditherToMonochrome(dst)

	// Encode back to PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, mono); err != nil {
		return "", fmt.Errorf("failed to encode PNG: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// ditherToMonochrome converts image to 1-bit using Floyd-Steinberg dithering
func ditherToMonochrome(img *image.RGBA) *image.Gray {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Create grayscale version with error diffusion buffer
	gray := make([][]float64, h)
	for y := 0; y < h; y++ {
		gray[y] = make([]float64, w)
		for x := 0; x < w; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			// Convert to grayscale using luminance formula
			lum := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
			gray[y][x] = lum / 256.0 // Scale to 0-255 range
		}
	}

	result := image.NewGray(bounds)

	// Floyd-Steinberg dithering
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			oldPixel := gray[y][x]
			var newPixel float64
			if oldPixel > 128 {
				newPixel = 255
			} else {
				newPixel = 0
			}
			result.SetGray(x, y, color.Gray{Y: uint8(newPixel)})

			err := oldPixel - newPixel
			// Distribute error to neighboring pixels
			if x+1 < w {
				gray[y][x+1] += err * 7 / 16
			}
			if y+1 < h {
				if x > 0 {
					gray[y+1][x-1] += err * 3 / 16
				}
				gray[y+1][x] += err * 5 / 16
				if x+1 < w {
					gray[y+1][x+1] += err * 1 / 16
				}
			}
		}
	}

	return result
}
