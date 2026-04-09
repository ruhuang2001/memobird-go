package textrender

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	imagedraw "image/draw"
	"math"
	"strings"
	"unicode"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"image/png"
)

const (
	defaultWidth      = 384
	defaultPadding    = 16
	defaultFontSize   = 20
	defaultLineHeight = 1.4
	defaultDPI        = 72
)

var (
	defaultForeground = color.Black
	defaultBackground = color.White
)

// Options configures pure-Go text rendering.
type Options struct {
	Width      int
	Padding    int
	FontSize   float64
	LineHeight float64
	MaxLines   int
	DPI        float64
	Foreground color.Color
	Background color.Color

	// FontData optionally overrides the built-in Go Regular font.
	// Supplying your own TTF is the simplest way to support glyph sets
	// outside the bundled font coverage.
	FontData []byte
}

type normalizedOptions struct {
	Width      int
	Padding    int
	FontSize   float64
	LineHeight float64
	MaxLines   int
	DPI        float64
	Foreground color.Color
	Background color.Color
	FontData   []byte
}

// RenderPNG renders text into a PNG image in memory.
func RenderPNG(text string, opts Options) ([]byte, error) {
	cfg, err := normalizeOptions(text, opts)
	if err != nil {
		return nil, err
	}

	face, err := loadFontFace(cfg)
	if err != nil {
		return nil, err
	}

	lines := wrapText(text, face, cfg.Width-2*cfg.Padding, cfg.MaxLines)
	if len(lines) == 0 {
		return nil, fmt.Errorf("text produced no renderable lines")
	}

	img := renderLines(lines, face, cfg)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("failed to encode PNG: %w", err)
	}

	return buf.Bytes(), nil
}

// RenderBase64PNG renders text into a PNG image and returns the base64 payload.
func RenderBase64PNG(text string, opts Options) (string, error) {
	pngBytes, err := RenderPNG(text, opts)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(pngBytes), nil
}

func normalizeOptions(text string, opts Options) (normalizedOptions, error) {
	if text == "" {
		return normalizedOptions{}, fmt.Errorf("text is required")
	}

	cfg := normalizedOptions{
		Width:      opts.Width,
		Padding:    opts.Padding,
		FontSize:   opts.FontSize,
		LineHeight: opts.LineHeight,
		MaxLines:   opts.MaxLines,
		DPI:        opts.DPI,
		Foreground: opts.Foreground,
		Background: opts.Background,
		FontData:   opts.FontData,
	}

	if cfg.Width == 0 {
		cfg.Width = defaultWidth
	}
	if cfg.Padding == 0 {
		cfg.Padding = defaultPadding
	}
	if cfg.FontSize == 0 {
		cfg.FontSize = defaultFontSize
	}
	if cfg.LineHeight == 0 {
		cfg.LineHeight = defaultLineHeight
	}
	if cfg.DPI == 0 {
		cfg.DPI = defaultDPI
	}
	if cfg.Foreground == nil {
		cfg.Foreground = defaultForeground
	}
	if cfg.Background == nil {
		cfg.Background = defaultBackground
	}
	if len(cfg.FontData) == 0 {
		cfg.FontData = goregular.TTF
	}

	switch {
	case cfg.Width <= 0:
		return normalizedOptions{}, fmt.Errorf("width must be greater than zero")
	case cfg.Padding < 0:
		return normalizedOptions{}, fmt.Errorf("padding must be zero or greater")
	case cfg.Width <= 2*cfg.Padding:
		return normalizedOptions{}, fmt.Errorf("width %d is too small for padding %d", cfg.Width, cfg.Padding)
	case cfg.FontSize <= 0:
		return normalizedOptions{}, fmt.Errorf("font size must be greater than zero")
	case cfg.LineHeight <= 0:
		return normalizedOptions{}, fmt.Errorf("line height must be greater than zero")
	case cfg.MaxLines < 0:
		return normalizedOptions{}, fmt.Errorf("max lines must be zero or greater")
	case cfg.DPI <= 0:
		return normalizedOptions{}, fmt.Errorf("dpi must be greater than zero")
	default:
		return cfg, nil
	}
}

func loadFontFace(opts normalizedOptions) (font.Face, error) {
	parsed, err := opentype.Parse(opts.FontData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse font data: %w", err)
	}

	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{
		Size:    opts.FontSize,
		DPI:     opts.DPI,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build font face: %w", err)
	}

	return face, nil
}

func renderLines(lines []string, face font.Face, opts normalizedOptions) *image.RGBA {
	metrics := face.Metrics()
	ascent := metrics.Ascent.Ceil()
	descent := metrics.Descent.Ceil()
	lineStep := maxInt(1, int(math.Ceil(float64(metrics.Height.Ceil())*opts.LineHeight)))
	height := 2*opts.Padding + ascent + descent
	if len(lines) > 1 {
		height += (len(lines) - 1) * lineStep
	}

	img := image.NewRGBA(image.Rect(0, 0, opts.Width, height))
	imagedraw.Draw(img, img.Bounds(), image.NewUniform(opts.Background), image.Point{}, imagedraw.Src)

	drawer := font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(opts.Foreground),
		Face: face,
	}

	baseline := opts.Padding + ascent
	for _, line := range lines {
		drawer.Dot = fixed.P(opts.Padding, baseline)
		drawer.DrawString(line)
		baseline += lineStep
	}

	return img
}

func wrapText(text string, face font.Face, maxWidth int, maxLines int) []string {
	measure := func(line string) int {
		var drawer font.Drawer
		drawer.Face = face
		return drawer.MeasureString(line).Ceil()
	}

	paragraphs := strings.Split(text, "\n")
	lines := make([]string, 0, len(paragraphs))

	for _, paragraph := range paragraphs {
		if paragraph == "" {
			lines = append(lines, "")
		} else {
			lines = append(lines, wrapParagraph(paragraph, measure, maxWidth)...)
		}

		if maxLines > 0 && len(lines) >= maxLines {
			return lines[:maxLines]
		}
	}

	return lines
}

func wrapParagraph(text string, measure func(string) int, maxWidth int) []string {
	if text == "" {
		return []string{""}
	}

	remaining := []rune(text)
	lines := make([]string, 0, 1)

	for len(remaining) > 0 {
		cut := bestWrapIndex(remaining, measure, maxWidth)
		line := strings.TrimRightFunc(string(remaining[:cut]), unicode.IsSpace)
		if line == "" {
			line = string(remaining[:cut])
		}
		lines = append(lines, line)
		remaining = trimLeadingWhitespace(remaining[cut:])
	}

	return lines
}

func bestWrapIndex(runes []rune, measure func(string) int, maxWidth int) int {
	lastFit := 0
	lastSpaceFit := 0

	for i := 1; i <= len(runes); i++ {
		if measure(string(runes[:i])) > maxWidth {
			break
		}
		lastFit = i
		if unicode.IsSpace(runes[i-1]) {
			lastSpaceFit = i
		}
	}

	switch {
	case lastFit == 0:
		return 1
	case lastSpaceFit > 0 && lastSpaceFit < len(runes):
		return lastSpaceFit
	default:
		return lastFit
	}
}

func trimLeadingWhitespace(runes []rune) []rune {
	for len(runes) > 0 && unicode.IsSpace(runes[0]) && runes[0] != '\n' {
		runes = runes[1:]
	}
	return runes
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
