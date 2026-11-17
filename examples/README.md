# Examples

The library is meant to be imported from Go code. Start with the
[quick start](../README.md#quick-start), then see the
[Chinese text](../README.md#chinese-text) and
[image printing](../README.md#images-and-pre-rendered-pages) examples.

## Files

- `quickstart.sh`: writes a temporary Go program and runs it against the real
  Memobird API. It contains placeholder credentials; edit them before running
  from the repository root with `bash examples/quickstart.sh`. With valid
  credentials it can print paper. It is not an offline test.
- `sample-note.html`: input for a caller-owned HTML-to-PNG renderer. The SDK
  does not render HTML; submit the resulting PNG with `PrintImage`.

## Run without a printer

From the repository root:

```sh
GOTOOLCHAIN=go1.26.11 go test ./memobird -run '^Example' -count=1 -v
GOTOOLCHAIN=go1.26.11 make e2e
```

The executable examples in `memobird/example_test.go` use local HTTP stand-ins,
as does the [E2E suite](../e2e/README.md). Neither command submits a real print.

Every print goes through an image. `client.PrintText` renders the text first;
`client.PrintImage` takes a PNG you prepared. For Chinese, pass a font with the
needed glyphs as `textrender.Options.FontData`, because the bundled font covers
Latin only and `PrintText` returns `memobird.ErrMissingGlyphs` otherwise.
