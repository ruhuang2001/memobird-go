// Package textrender turns text into a PNG a thermal printer can print, without
// a browser or a font server.
//
// RenderPNG lays text out at a fixed paper width, wrapping on rune boundaries and
// spaces, and encodes the result. Text is cleaned first by Normalize unless
// Options.SkipNormalize is set. Measure reports the same layout without encoding
// an image, so a caller can reject text that is too tall for the paper before
// allocating or uploading anything.
//
// # Fonts
//
// The bundled font covers Latin text only. Pass Options.FontData to print another
// script, usually a CJK face; without one, characters the font cannot draw are
// laid out as blank boxes rather than dropped. FontHasGlyphs reports whether a
// given font covers some text, which lets a caller choose a font before
// rendering instead of after seeing the result.
//
//	png, err := textrender.RenderBase64PNG(text, textrender.Options{
//		FontData: cjkFontBytes,
//	})
//
// # Layout
//
// Width, Padding, FontSize, LineHeight, and DPI default to a 384px slip with 16px
// padding and 20px text. MaxLines truncates rather than fails, because it exists
// to bound a preview; a caller printing paper should check Layout.Truncated from
// Measure instead.
package textrender
