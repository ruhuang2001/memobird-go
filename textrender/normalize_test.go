package textrender

import (
	"bytes"
	"image/png"
	"strings"
	"testing"
)

func TestNormalizeRemovesEncodingNoiseOnly(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{"control characters", "a\x00b\x07c", "abc"},
		{"tab becomes one space", "a\tb", "a b"},
		{"repeated spaces", "a     b", "a b"},
		{"zero width characters", "a\u200bb\u200dc\uFEFFd", "abcd"},
		{"non-breaking space", "a\u00a0b", "a b"},
		{"carriage returns", "a\r\nb\rc", "a\nb\nc"},
		{"trailing spaces per line", "a   \nb", "a\nb"},
		{"three blank lines collapse", "a\n\n\n\nb", "a\n\nb"},
		{"document edges", "\n\n  hello  \n\n", "hello"},
		// A bullet is content the font can draw, so it survives untouched.
		{"bullet preserved", "• milk\n• eggs", "• milk\n• eggs"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Normalize(tc.in); got != tc.want {
				t.Fatalf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeKeepsCharactersEvenWhenTheFontLacksThem(t *testing.T) {
	// Silence is worse than a blank box: deciding a glyph is missing is the
	// font's business, and the caller may swap in a CJK face afterwards.
	got := Normalize("hello 中文 world")
	if !strings.Contains(got, "中文") {
		t.Fatalf("Normalize() = %q, want CJK preserved", got)
	}
}

func TestNormalizeIsIdempotent(t *testing.T) {
	const input = "•  a\t b \n\n\n\n中 文\x00\r\n"
	once := Normalize(input)
	if twice := Normalize(once); once != twice {
		t.Fatalf("Normalize() is not idempotent: %q then %q", once, twice)
	}
}

func TestFontHasGlyphs(t *testing.T) {
	ok, err := FontHasGlyphs("plain ascii", nil)
	if err != nil || !ok {
		t.Fatalf("FontHasGlyphs(ascii) = %v, %v; want true", ok, err)
	}

	ok, err = FontHasGlyphs("中文", nil)
	if err != nil {
		t.Fatalf("FontHasGlyphs(cjk) error = %v", err)
	}
	if ok {
		t.Fatal("FontHasGlyphs(cjk) = true, want false for the Latin-only bundled font")
	}

	// Noise Normalize removes must not count as missing coverage.
	ok, err = FontHasGlyphs("a\tb\u200dc", nil)
	if err != nil || !ok {
		t.Fatalf("FontHasGlyphs(noise) = %v, %v; want true", ok, err)
	}

	if _, err := FontHasGlyphs("x", []byte("not a font")); err == nil {
		t.Fatal("FontHasGlyphs(bad font) error = nil, want parse error")
	}
}

func TestRenderPNGRejectsTextThatNormalizesToNothing(t *testing.T) {
	// Only whitespace and control characters would otherwise print a blank slip.
	if _, err := RenderPNG("\x00\x01 \t\n", Options{}); err == nil {
		t.Fatal("RenderPNG() error = nil, want no-renderable-lines error")
	}
}

func TestRenderPNGSkipNormalizeKeepsRawInput(t *testing.T) {
	// Without normalization the wrapped lines keep the raw characters, which is
	// what a caller that already prepared the text expects.
	raw, err := Measure("a\tb", Options{SkipNormalize: true})
	if err != nil {
		t.Fatalf("Measure() error = %v", err)
	}
	if raw.Lines != 1 {
		t.Fatalf("raw lines = %d, want 1", raw.Lines)
	}
	if _, err := RenderPNG("\x00\x01", Options{SkipNormalize: true}); err != nil {
		t.Fatalf("RenderPNG(SkipNormalize) error = %v, want the text rendered as given", err)
	}
}

func TestMeasureMatchesRenderedHeight(t *testing.T) {
	// The whole point of Measure is to predict RenderPNG: a disagreement means a
	// caller rejects text that fits, or sends a slip that exceeds the paper.
	for _, tc := range []struct {
		name string
		text string
		opts Options
	}{
		{"defaults", "Hello,\nMemobird!", Options{}},
		{"long wrapped paragraph", strings.Repeat("ink and paper ", 40), Options{}},
		{"blank lines", "one\n\nthree\n\n\nfive", Options{}},
		{"custom metrics", "a longer line that wraps several times over", Options{Width: 240, Padding: 12, FontSize: 18, LineHeight: 1.6}},
		{"max lines", strings.Repeat("word ", 100), Options{MaxLines: 3}},
		{"skip normalize", "a\tb\x00c", Options{SkipNormalize: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			layout, err := Measure(tc.text, tc.opts)
			if err != nil {
				t.Fatalf("Measure() error = %v", err)
			}
			data, err := RenderPNG(tc.text, tc.opts)
			if err != nil {
				t.Fatalf("RenderPNG() error = %v", err)
			}
			img, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("png.Decode() error = %v", err)
			}
			if got := img.Bounds().Dy(); got != layout.Height {
				t.Fatalf("rendered height = %d, measured = %d", got, layout.Height)
			}
		})
	}
}

func TestMeasureReportsTruncation(t *testing.T) {
	layout, err := Measure(strings.Repeat("word ", 100), Options{MaxLines: 3})
	if err != nil {
		t.Fatalf("Measure() error = %v", err)
	}
	if !layout.Truncated || layout.Lines != 3 {
		t.Fatalf("layout = %+v, want 3 lines marked truncated", layout)
	}

	fits, err := Measure("short", Options{MaxLines: 3})
	if err != nil {
		t.Fatalf("Measure() error = %v", err)
	}
	if fits.Truncated {
		t.Fatalf("layout = %+v, want no truncation", fits)
	}
}
