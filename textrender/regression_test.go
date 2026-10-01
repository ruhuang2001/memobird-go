package textrender

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// Failure scenarios written before the fix:
// - A fitting sentence (including exact width) wraps at its last space.
// - MaxLines: 1 drops words from that fitting sentence.
// - A long paragraph continues measuring text after its line budget is filled.
// - Earlier paragraphs and blank lines are not deducted from that budget.
// - Narrow widths, Unicode, and whitespace lose text or stop making progress.
// Repeat: go test ./textrender -run 'TestTextRenderingE2E|TestWrapTextBoundedWork' -v
// PNG evidence: ../build/review-e2e/text-*.png

func TestTextRenderingE2E(t *testing.T) {
	const sentence = "hello world"
	full, err := RenderPNG(sentence, Options{Width: 240})
	if err != nil {
		t.Fatal(err)
	}
	capped, err := RenderPNG(sentence, Options{Width: 240, MaxLines: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(full, capped) {
		t.Fatal("fitting sentence changed when capped to one line")
	}

	for name, text := range map[string]string{
		"fitting": sentence,
		"wrapped": "alpha beta gamma delta epsilon zeta eta theta iota kappa lambda mu",
		"unicode": "café déjà vu\nΚαλημέρα κόσμε\nПривет мир",
	} {
		data, err := RenderPNG(text, Options{Width: 240, MaxLines: 3})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := png.Decode(bytes.NewReader(data)); err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join("..", "build", "review-e2e")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "text-"+name+".png"), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
}

type countingFace struct {
	font.Face
	advances int
}

func (f *countingFace) GlyphAdvance(r rune) (fixed.Int26_6, bool) {
	f.advances++
	return f.Face.GlyphAdvance(r)
}

func TestWrapTextBoundedWork(t *testing.T) {
	cfg, err := normalizeOptions("ok", Options{})
	if err != nil {
		t.Fatal(err)
	}
	face, err := loadFontFace(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer face.Close()
	measured := &countingFace{Face: face}

	for _, tc := range []struct {
		name            string
		text            string
		width, maxLines int
		want            []string
	}{
		{"fits", "hello world", 1000, 0, []string{"hello world"}},
		{"exact fit", "hello world", font.MeasureString(face, "hello world").Ceil(), 0, []string{"hello world"}},
		{"bounded paragraph", strings.Repeat("word ", 2000), 60, 1, []string{"word"}},
		{"prior paragraph", "ok\n" + strings.Repeat("word ", 2000), 60, 2, []string{"ok", "word"}},
		{"blank line", "\n" + strings.Repeat("word ", 2000), 60, 2, []string{"", "word"}},
		{"unicode narrow", "éΩ中", 1, 0, []string{"é", "Ω", "中"}},
		{"whitespace", "a\t  b   ", 1000, 0, []string{"a\t  b"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			measured.advances = 0
			if got := wrapText(tc.text, measured, tc.width, tc.maxLines); !slices.Equal(got, tc.want) {
				t.Fatalf("lines = %q, want %q", got, tc.want)
			}
			if measured.advances > 1000 {
				t.Fatalf("measured %d glyphs after a small line budget; want at most 1000", measured.advances)
			}
		})
	}
}
