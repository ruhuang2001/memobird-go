# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Breaking: every print now goes through an image. The vendor's service-side
  renderers do not decode GBK, so `PrintHTML` and `PrintURL` turned Chinese into
  mojibake; both are removed rather than left as a path that works only for Latin
  text. Two calls cover printing now:
  - `PrintText` renders text with `textrender` and prints the image.
  - `PrintImage` prints a PNG the caller prepared.
  Deliberately left unreleased as v0.1.0; `memobird.Version` stays at 0.0.4.

### Added

- `PrintText(ctx, text, textrender.Options)` is the single text entry point. It
  takes the same options as `textrender`, including `FontData`.
- `ErrMissingGlyphs` reports text the font cannot draw, so a slip never prints as
  a row of blank boxes.

### Removed

- `PrintHTML`, `PrintURL`, `ConvertToMonochrome`, and the `formatter` and
  `printutil` internal packages. Image conversion is a step inside `PrintImage`
  and is no longer a public call: an image printed without it cannot reach paper.
- The `renderer`, `storage`, and `config` packages, along with
  `PrintURLAsImages`, `PrintHTMLAsImages`, `BindingStore`, `BindAndPersist`,
  `PersistUserBinding`, `RestoreUserBinding`, `Config.PrintInterval`, and the
  `ErrRendererRequired`, `ErrNoPagesRendered`, and `ErrBindingStoreRequired`
  sentinels. Rendering that needs a browser belongs to the caller; bindings are
  the caller's to persist through `Config.UserID`.
- The module now requires only `golang.org/x/image`; `x/text` remains an indirect
  dependency of `x/image`.

## [v0.0.4] - 2026-11-15

### Added

- `textrender.Normalize` removes characters that cost paper or corrupt layout
  without changing the words: control characters, zero-width characters,
  byte-order marks, soft hyphens, and non-breaking or fixed-width spaces. It also
  collapses repeated spaces, more than one blank line, and leading and trailing
  blank lines. `RenderPNG` and `Measure` normalize by default; set
  `Options.SkipNormalize` to opt out.
- `textrender.Measure` reports `Layout{Lines, Height, Truncated}` without encoding
  an image, so a caller can reject text that exceeds the paper before allocating
  or uploading anything. `Layout.Truncated` reports when `MaxLines` would cut the
  text short, which rendering previously hid.
- `textrender.FontHasGlyphs` reports whether a font covers some text. Uncovered
  runes print as blank boxes, so a caller can pick a font with the needed coverage
  instead of discovering the gap on paper.

### Changed

- `Normalize` never removes a visible character, including one the loaded font
  cannot draw. Whether a glyph exists is a property of the font rather than of the
  text, so that decision is left to `FontHasGlyphs`.

## [v0.0.3] - 2026-11-15

### Added

- `PrintFlagUnknown`, `PrintFlagPending`, and `PrintFlagPrinted` name the values
  `/home/getprintstatus` returns for `printflag`. The API documents none of them;
  the values and their meanings come from probing the live service against a
  real device.
- `PrintStatusResponse.IsUnknown` reports a print content ID the API has no
  record of, and `PrintStatusResponse.Terminal` reports whether the status can
  still change.
- `ErrUnknownPrintID` is returned by `WaitForPrint` for such an ID, which is
  identified with `errors.Is`.

### Fixed

- `WaitForPrint` stops on an unknown print content ID instead of polling until
  its timeout. Before this change a caller waited out the whole timeout for a
  job that would never print.
- The `WaitForPrint` documentation states that a device that is offline keeps
  its job pending in the cloud and prints it after reconnecting. The cloud
  reports no failure for that case, so the caller's timeout is the only bound
  on how long a pending job waits.

## [v0.0.2] - 2026-11-12

### Added

- `renderer.Config.StartupTimeout` bounds the first Chrome launch, defaulting
  to 45s. It is independent of the per-render `Timeout`.

### Fixed

- `renderer` no longer applies the per-render `Timeout` to the first Chrome
  launch. A cold start on a loaded or virtualized machine can take longer than
  one render; the old behavior reported `context deadline exceeded` and
  discarded a browser that had started fine. The launch deadline is reported
  as a wrapped `context.DeadlineExceeded`, and a caller's context still cancels
  the launch.
- The Chrome DevTools WebSocket wait follows `StartupTimeout` instead of
  chromedp's fixed 20s, so hosts that are merely slow no longer fail with
  `websocket url timeout reached`.

### Changed

- The end-to-end harness reads `MEMOBIRD_E2E_DISABLE_SANDBOX=1` so CI can run
  Chrome where no sandbox is available. The renderer still enables the sandbox
  by default.

## [v0.0.1] - 2026-11-12

First tagged release. The module path is `github.com/ruhuang2001/memobird-go`.

### Added

- `memobird.Client.WaitForPrint` polls a submitted print job until the printer
  reports it printed, with a configurable interval and timeout. `WaitOptions`
  defaults to a 1s interval and a 2m timeout.
- `memobird.Config.PrintInterval` paces page submissions in
  `PrintURLAsImages` and `PrintHTMLAsImages`; zero submits pages back to back.
- `renderer.Config` and `renderer.NewWithConfig` configure the render timeout,
  render delay, and headless sandbox. The sandbox is now enabled by default, so
  pass `DisableSandbox: true` for containers that forbid Chrome's namespace
  sandbox.
- `memobird.Version` reports the library version.
- Exported sentinel errors `ErrUserIDNotConfigured`, `ErrBindingStoreRequired`,
  `ErrRendererRequired`, and `ErrNoPagesRendered`, all testable with `errors.Is`.
- `memobird.APIError` carries the action, response code, and message when the
  remote API rejects a request; `errors.As` now distinguishes API rejections
  from transport failures.
- Package documentation and runnable examples for the client, `WaitForPrint`,
  and error handling.

### Changed

- `memobird.Client` bounds API response bodies at 1 MiB and validates a custom
  `BaseURL` for an `http`/`https` scheme and non-empty host.
- `Config.BaseURL` trailing slashes are normalized, and a caller-supplied
  `HTTPClient` is cloned when a `Config.Timeout` is set, instead of being
  mutated.
- `PrintResponse.SmartGuid` is now `SmartGUID`; the JSON field `smartGuid` is
  unchanged.
- `BindUser` rejects a blank user identifier instead of sending it upstream.
- `memobird.Client` is safe for concurrent use, including `SetUserAgent`, which
  previously raced with in-flight requests.

### Fixed

- Response bodies are always closed, and the close error is reported.
- Multi-page print workflows no longer submit every page with no delay.
- `renderer.ProcessImageForPrint` and `textrender` no longer rely on implicit
  conversions flagged by static analysis.

### Requirements

- Go 1.26.11 or newer, matching the toolchain in `go.mod`.

[Unreleased]: https://github.com/ruhuang2001/memobird-go/compare/v0.0.4...HEAD
[v0.0.4]: https://github.com/ruhuang2001/memobird-go/compare/v0.0.3...v0.0.4
[v0.0.3]: https://github.com/ruhuang2001/memobird-go/compare/v0.0.2...v0.0.3
[v0.0.2]: https://github.com/ruhuang2001/memobird-go/compare/v0.0.1...v0.0.2
[v0.0.1]: https://github.com/ruhuang2001/memobird-go/releases/tag/v0.0.1
