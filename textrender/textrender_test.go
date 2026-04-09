package textrender

import (
	"bytes"
	"encoding/base64"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func TestRenderPNGDefaults(t *testing.T) {
	data, err := RenderPNG("Hello,\nMemobird!", Options{})
	if err != nil {
		t.Fatalf("RenderPNG() error = %v", err)
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("png.Decode() error = %v", err)
	}

	if img.Bounds().Dx() != defaultWidth {
		t.Fatalf("width = %d, want %d", img.Bounds().Dx(), defaultWidth)
	}
	if img.Bounds().Dy() <= 0 {
		t.Fatalf("height = %d, want > 0", img.Bounds().Dy())
	}

	bg := color.RGBAModel.Convert(img.At(0, 0)).(color.RGBA)
	if bg != (color.RGBA{255, 255, 255, 255}) {
		t.Fatalf("top-left pixel = %#v, want white background", bg)
	}
}

func TestRenderPNGCustomOptions(t *testing.T) {
	opts := Options{
		Width:      240,
		Padding:    12,
		FontSize:   18,
		LineHeight: 1.6,
		MaxLines:   2,
		Foreground: color.RGBA{0, 0, 255, 255},
		Background: color.RGBA{255, 240, 220, 255},
	}

	data, err := RenderPNG("this is a longer line that should wrap across multiple lines", opts)
	if err != nil {
		t.Fatalf("RenderPNG() error = %v", err)
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("png.Decode() error = %v", err)
	}

	if img.Bounds().Dx() != opts.Width {
		t.Fatalf("width = %d, want %d", img.Bounds().Dx(), opts.Width)
	}

	bg := color.RGBAModel.Convert(img.At(0, 0)).(color.RGBA)
	wantBG := color.RGBA{255, 240, 220, 255}
	if bg != wantBG {
		t.Fatalf("top-left pixel = %#v, want %#v", bg, wantBG)
	}
}

func TestRenderBase64PNG(t *testing.T) {
	encoded, err := RenderBase64PNG("hello world", Options{Width: 220})
	if err != nil {
		t.Fatalf("RenderBase64PNG() error = %v", err)
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("DecodeString() error = %v", err)
	}

	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("png.Decode() error = %v", err)
	}
}

func TestRenderPNGValidation(t *testing.T) {
	tests := []struct {
		name        string
		text        string
		opts        Options
		errorSubstr string
	}{
		{name: "empty text", text: "", opts: Options{}, errorSubstr: "text is required"},
		{name: "negative width", text: "ok", opts: Options{Width: -1}, errorSubstr: "width must be greater than zero"},
		{name: "negative padding", text: "ok", opts: Options{Padding: -1}, errorSubstr: "padding must be zero or greater"},
		{name: "width too small for padding", text: "ok", opts: Options{Width: 20, Padding: 10}, errorSubstr: "too small for padding"},
		{name: "invalid font size", text: "ok", opts: Options{FontSize: -1}, errorSubstr: "font size must be greater than zero"},
		{name: "invalid line height", text: "ok", opts: Options{LineHeight: -1}, errorSubstr: "line height must be greater than zero"},
		{name: "invalid max lines", text: "ok", opts: Options{MaxLines: -1}, errorSubstr: "max lines must be zero or greater"},
		{name: "invalid dpi", text: "ok", opts: Options{DPI: -1}, errorSubstr: "dpi must be greater than zero"},
		{name: "invalid font data", text: "ok", opts: Options{FontData: []byte("not-a-font")}, errorSubstr: "failed to parse font data"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := RenderPNG(tt.text, tt.opts)
			if err == nil || !strings.Contains(err.Error(), tt.errorSubstr) {
				t.Fatalf("RenderPNG() error = %v, want substring %q", err, tt.errorSubstr)
			}
		})
	}
}

func TestWrapTextHonorsMaxLines(t *testing.T) {
	opts, err := normalizeOptions("ok", Options{Width: 120, MaxLines: 2})
	if err != nil {
		t.Fatalf("normalizeOptions() error = %v", err)
	}

	face, err := loadFontFace(opts)
	if err != nil {
		t.Fatalf("loadFontFace() error = %v", err)
	}

	lines := wrapText("one two three four five six seven eight nine", face, opts.Width-2*opts.Padding, opts.MaxLines)
	if len(lines) != 2 {
		t.Fatalf("len(lines) = %d, want 2", len(lines))
	}
}

func TestWrapParagraphBreaksLongTokens(t *testing.T) {
	measure := func(line string) int {
		return len([]rune(line)) * 10
	}

	lines := wrapParagraph("supercalifragilisticexpialidocious", measure, 40)
	if len(lines) <= 1 {
		t.Fatalf("len(lines) = %d, want > 1", len(lines))
	}
	for _, line := range lines {
		if len([]rune(line)) > 4 {
			t.Fatalf("line %q exceeds expected wrapped width", line)
		}
	}
}
