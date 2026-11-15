package textrender

import (
	"fmt"
	"unicode"

	"golang.org/x/image/font/opentype"
)

// FontHasGlyphs reports whether fontData can draw every rune in text. Pass the
// same bytes as Options.FontData; nil means the bundled font.
//
// It exists because Normalize deliberately keeps characters the font cannot
// draw. A caller that must not print a blank box checks this first and supplies a
// font with the needed coverage, usually a CJK face, instead of having the text
// silently removed. Characters Normalize drops, such as control characters, are
// ignored here too.
func FontHasGlyphs(text string, fontData []byte) (bool, error) {
	if len(fontData) == 0 {
		fontData = bundledFontData()
	}
	parsed, err := opentype.Parse(fontData)
	if err != nil {
		return false, fmt.Errorf("failed to parse font data: %w", err)
	}
	for _, r := range text {
		// Skip everything Normalize removes, plus whitespace: none of it needs a
		// glyph, so none of it can be missing coverage.
		if r == '\t' || unicode.IsSpace(r) || unicode.IsControl(r) || isInvisible(r) {
			continue
		}
		index, ok := parsed.GlyphIndex(nil, r)
		if ok != nil || index == 0 {
			return false, nil
		}
	}
	return true, nil
}
