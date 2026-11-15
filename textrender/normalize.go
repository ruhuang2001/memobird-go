package textrender

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Normalize prepares text for a thermal printer without changing what it says.
//
// A printer renders a fixed bitmap, so characters that a console would ignore
// still consume paper or corrupt the layout. Normalize removes exactly those and
// leaves the words alone:
//
//   - invalid UTF-8 sequences and control characters are dropped (a tab becomes
//     one space);
//   - zero-width and byte-order marks are dropped;
//   - fixed-width and non-breaking spaces become ordinary spaces;
//   - runs of spaces collapse and trailing spaces are trimmed per line;
//   - more than one consecutive blank line collapses to one;
//   - leading and trailing blank lines are removed.
//
// It never removes a visible character, including ones the loaded font cannot
// draw. Whether a glyph exists is a property of the font the caller chose, not of
// the text, so that decision belongs with FontHasGlyphs rather than here. It is
// idempotent, and an empty result means there was nothing to print.
func Normalize(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	lines := strings.Split(text, "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		normalized = append(normalized, normalizeLine(line))
	}

	return strings.TrimSpace(collapseBlankLines(normalized))
}

func normalizeLine(line string) string {
	var builder strings.Builder
	builder.Grow(len(line))
	previousSpace := false
	for _, r := range line {
		if r == utf8.RuneError {
			continue
		}
		if r == '\t' {
			r = ' '
		}
		if unicode.IsControl(r) || isInvisible(r) {
			continue
		}
		if isFixedWidthSpace(r) {
			r = ' '
		}
		if r == ' ' {
			if previousSpace {
				continue
			}
			previousSpace = true
		} else {
			previousSpace = false
		}
		builder.WriteRune(r)
	}
	return strings.TrimRight(builder.String(), " ")
}

func collapseBlankLines(lines []string) string {
	kept := make([]string, 0, len(lines))
	blank := 0
	for _, line := range lines {
		if line == "" {
			blank++
			// A single blank line separates paragraphs; more only wastes paper.
			if blank > 1 {
				continue
			}
		} else {
			blank = 0
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// isInvisible reports runes that occupy a position without drawing anything.
// They make line breaks look wrong on paper even though they are valid text.
func isInvisible(r rune) bool {
	switch r {
	case '\u200B', '\u200C', '\u200D', '\u2060', '\uFEFF', '\u00AD':
		return true
	}
	return false
}

func isFixedWidthSpace(r rune) bool {
	switch r {
	case '\u00A0', '\u2007', '\u202F', '\u2009', '\u200A', '\u3000':
		return true
	}
	return false
}
