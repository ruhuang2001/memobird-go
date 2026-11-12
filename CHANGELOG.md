# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/ruhuang2001/memobird-go/compare/v0.0.1...HEAD
[v0.0.1]: https://github.com/ruhuang2001/memobird-go/releases/tag/v0.0.1
