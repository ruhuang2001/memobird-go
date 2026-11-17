# memobird-go

[![CI](https://github.com/ruhuang2001/memobird-go/actions/workflows/ci.yml/badge.svg)](https://github.com/ruhuang2001/memobird-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ruhuang2001/memobird-go.svg)](https://pkg.go.dev/github.com/ruhuang2001/memobird-go)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A Go client for the official Memobird thermal printer Web API. Bind a device,
print text or an image, and query the status reported by the service.

Every print goes through an image, avoiding the service-side text encoding path
reported to produce garbled Chinese. `PrintText` renders locally before calling
`PrintImage`, so callers do not need to choose a print path based on language.

## Install

```bash
go get github.com/ruhuang2001/memobird-go@v0.0.5
```

Requires Go 1.26.11 or newer.

**Upgrading from v0.0.4:** v0.0.5 includes breaking API removals despite its patch
version. See [migration notes](CHANGELOG.md#v005---2026-11-16). Existing
`textrender` + `PrintImage` integrations can continue unchanged.

## Quick start

You need a Memobird API access key, a device ID, and a stable user-identifying
string from your application. The following example contacts the real service
and can print paper. For a credential-free local check, use `make e2e`.

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/ruhuang2001/memobird-go/memobird"
	"github.com/ruhuang2001/memobird-go/textrender"
)

func main() {
	ctx := context.Background()

	client, err := memobird.NewClient(memobird.Config{
		AccessKey: "your-access-key",
		DeviceID:  "your-device-id",
		Timeout:   30 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Bind once per device. The returned user ID is kept on the client.
	if _, err := client.BindAndRemember(ctx, "your-user-identifying-string"); err != nil {
		log.Fatal(err)
	}

	printed, err := client.PrintText(ctx, "Hello, Memobird!", textrender.Options{})
	if err != nil {
		log.Fatal(err)
	}

	status, err := client.WaitForPrint(ctx, printed.PrintContentID, memobird.WaitOptions{})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("printed: %v", status.IsPrinted())
}
```

Run `go doc github.com/ruhuang2001/memobird-go/memobird` for the full API, or read
the [package reference](https://pkg.go.dev/github.com/ruhuang2001/memobird-go/memobird).

## Packages

| Package | Purpose | Dependency |
| --- | --- | --- |
| `memobird` | Web API client, binding, `PrintText`, `PrintImage`, `WaitForPrint` | `x/image` (through `textrender`) |
| `textrender` | Pure Go text-to-PNG rendering, also usable on its own | `x/image`, `x/text` |

The module has no browser, database, or configuration-file dependency: bindings
are yours to persist, and content that needs a browser is rendered by the caller
and submitted with `PrintImage`.

## Two print calls

| You have | Call |
| --- | --- |
| Text | `PrintText` |
| A PNG, or a page you rendered yourself | `PrintImage` |

Both end at the same place: `PrintImage` converts the image to the printer's
bitmap format and submits it. `PrintText` renders the string with `textrender`
first, so it takes the same `textrender.Options`.

### Chinese text

Chinese works through the same call; pass a font that covers it. The bundled font
is Latin-only, and `PrintText` returns `ErrMissingGlyphs` instead of printing a
blank box for every character the font cannot draw.

```go
// Add "os" to your imports. Supply a font you have permission to distribute.
cjkFont, err := os.ReadFile("/path/to/cjk-font.ttf")
if err != nil {
	log.Fatal(err)
}
_, err = client.PrintText(ctx, "今天要买的三样东西", textrender.Options{
	FontData: cjkFont, // a supported TTF/OTF covering every character in this text
})
if err != nil {
	log.Fatal(err)
}
```

There is no bundled CJK font, automatic font download, or font fallback. Keep
your text as UTF-8; you do not need to encode it as GBK.

### Images and pre-rendered pages

`PrintImage` takes a **base64 string**, not a file path, URL, or data URI. For a
PNG file, add `os` and `encoding/base64` to your imports:

```go
pngBytes, err := os.ReadFile("receipt.png")
if err != nil {
	log.Fatal(err)
}
printed, err := client.PrintImage(ctx, base64.StdEncoding.EncodeToString(pngBytes))
if err != nil {
	log.Fatal(err)
}
log.Printf("submitted print ID: %d", printed.PrintContentID)
```

Text rendering is local, but image conversion is not: `PrintImage` sends the
image to the vendor's conversion endpoint, then submits the returned bitmap.
Both print methods therefore need network access and send the image content to
the Memobird service.

For HTML or URLs, render to PNG outside this SDK and use `PrintImage`. Existing
`textrender.RenderBase64PNG` + `PrintImage` integrations do not need to change.

### Binding

```go
resp, err := client.BindAndRemember(ctx, "your-user-identifying-string")
```

`SetUserID` / `GetUserID` read and write the bound user without the network. To
keep the binding across restarts, store `resp.UserID` yourself and set
`Config.UserID` on the next start.
`BindUser` only returns the binding response; unlike `BindAndRemember`, it does
not update the client's user ID.

### Waiting for the printer

`WaitForPrint` polls `GetPrintStatus` until `printflag` reports printed, then
returns the final status. It stops on context cancellation, on the configured
timeout, on API errors, and when the API has no record of the print content ID.

```go
status, err := client.WaitForPrint(ctx, printContentID, memobird.WaitOptions{
	Interval: time.Second,     // default 1s
	Timeout:  2 * time.Minute, // default 2m
})
```

A device can remain pending while offline. The earlier of the context deadline
and `WaitOptions.Timeout` bounds the wait; canceling the wait does not cancel the
cloud job. A successful submission is not confirmation of paper output: use
`GetPrintStatus` or `WaitForPrint` to observe the service's reported status.

### Layout

`textrender.Options` covers width, padding, font size, line height, max lines,
DPI, foreground and background colours, and custom TTF/OTF font data. The default
embedded font covers Latin; provide `FontData` for broader scripts.

Text is normalized before layout unless `Options.SkipNormalize` is set:
control characters, zero-width characters, byte-order marks, and non-breaking
spaces are removed or replaced, runs of spaces collapse, and more than one blank
line collapses to one. No visible character is ever removed, including one the
font cannot draw, because whether a glyph exists depends on the font the caller
chose.

`MaxLines` truncates rather than returning an error, including when used with
`PrintText`. Check `Measure` first if losing content is unacceptable. The
standalone renderer does not reject missing glyphs; `PrintText` does.

Two calls help before printing paper:

```go
// Does the font cover this text? An uncovered rune draws as a blank box.
covered, err := textrender.FontHasGlyphs(text, cjkFont)
if err != nil {
	log.Fatal(err)
}
if !covered {
	// Pick a font with the needed glyphs before rendering.
}

// Will it fit, and is MaxLines about to cut it short?
layout, err := textrender.Measure(text, textrender.Options{FontData: cjkFont, MaxLines: 40})
if err != nil {
	log.Fatal(err)
}
if layout.Truncated {
	// Split the text instead of printing a slip that stops mid-sentence.
}
```

## Errors

Configuration and workflow failures return exported sentinels, testable with
`errors.Is`:

```go
if errors.Is(err, memobird.ErrUserIDNotConfigured) {
	// Bind or SetUserID first.
}
```

| Sentinel | Returned by |
| --- | --- |
| `ErrUserIDNotConfigured` | Any print call before a user is bound |
| `ErrUnknownPrintID` | `WaitForPrint` when the API has no record of the ID |
| `ErrMissingGlyphs` | `PrintText` when the font cannot draw the text |

A request the remote API rejects returns `*memobird.APIError` with `Action`,
`Code`, and `Message`. Transport and decoding failures are wrapped with context
instead, so `errors.As` separates the two:

```go
var apiErr *memobird.APIError
if errors.As(err, &apiErr) {
	log.Printf("%s rejected: code=%d message=%s", apiErr.Action, apiErr.Code, apiErr.Message)
}
```

`memobird.Version` reports the library version at build time.

## Concurrency

`memobird.Client` is safe for concurrent use once `NewClient` returns.
`SetUserID`, `SetUserAgent`, and all print methods may be called from multiple
goroutines.

## Print submissions are not idempotent

`PrintImage` submits one page per call. Nothing is retried automatically, because
a retry could print a page twice. A caller printing several pages should submit
them itself, space the calls out if the service needs it, and inspect each
response.

If submission times out or the connection drops, the service may already have
accepted the job. Do not blindly resubmit. If you have a `PrintContentID`, query
that job instead; a waiting timeout is not a reason to submit it again.

## Development

```bash
make test    # go test -race ./...
make e2e     # local workflow E2E against an API stand-in
make lint    # golangci-lint
```

The E2E never contacts a printer. It writes a rendered PNG, a submission receipt,
the `memobird` dependency list, and logs to `build/review-e2e/`.

## Go compatibility

| Go | Status |
| --- | --- |
| 1.26.11 | Minimum, declared in `go.mod`, runs CI |
| stable | Tested in CI |

The minimum tracks security fixes in the standard library, so it can rise in a
patch release of this library. CI tests both the declared minimum and Go stable.
Use `GOTOOLCHAIN=go1.26.11` locally to match the minimum-version CI job.

See [examples](examples/README.md), [contributor instructions](CONTRIBUTING.md),
and [local E2E coverage](e2e/README.md) for runnable checks.

## License

MIT — see [LICENSE](LICENSE).
