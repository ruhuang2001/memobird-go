# Local end-to-end checks

Run from the repository root:

```sh
GOTOOLCHAIN=go1.26.11 make e2e
```

The suite uses a local HTTP server in place of the Memobird API. No printer
credentials, browser, database, or external print service are needed. It does
not verify physical paper output or the vendor's image conversion algorithm.

## Coverage

- `TestE2EPrintWorkflow`: binding, local text rendering, image conversion and
  submission requests, and polling the simulated print status.
- `TestE2EFontRendering`: embedded and custom Go fonts, invalid and truncated
  fonts, and a successful render after a rejected font.
- The `textrender` suite: wrapping, normalization, glyph coverage, measurement,
  and truncation behavior.
- The runner rejects browser, database, and config-loader dependencies in
  `go list -deps ./memobird`.

These checks do not prove that an arbitrary CJK font covers your text. Supply
your own font and check it with `textrender.FontHasGlyphs`; `PrintText` performs
that check automatically.

## Artifacts

`make e2e` writes to `build/review-e2e/`:

- `workflow-print-*.png`: image payloads received by the conversion stand-in.
- `workflow-receipt.json`: accepted submissions and final simulated status.
- `font-*.png` and `font-report.json`: font samples, dimensions, decoded-pixel
  hashes, timing, and allocation observations.
- `client-dependencies.txt`: the API client's dependency graph.
- `test.log`: test output.

For a custom output directory, invoke the tests directly (this does not run the
runner's dependency check):

```sh
MEMOBIRD_E2E=1 MEMOBIRD_E2E_ARTIFACTS=/absolute/output/directory \
  GOTOOLCHAIN=go1.26.11 go test -race -count=1 -v ./textrender ./e2e
```

To compare font rendering across dependency upgrades, run `TestE2EFontRendering`
on both revisions and compare `Name`, `Width`, `Height`, and `PixelSHA256` in
their reports. Timing and allocation measurements can vary and are not resource
limits or proof against malicious fonts.

For build, lint, race, and vulnerability checks, see
[contributor instructions](../CONTRIBUTING.md#before-opening-a-pull-request).
