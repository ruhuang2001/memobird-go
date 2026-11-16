# memobird-go

[![CI](https://github.com/ruhuang2001/memobird-go/actions/workflows/ci.yml/badge.svg)](https://github.com/ruhuang2001/memobird-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ruhuang2001/memobird-go.svg)](https://pkg.go.dev/github.com/ruhuang2001/memobird-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/ruhuang2001/memobird-go)](https://goreportcard.com/report/github.com/ruhuang2001/memobird-go)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A Go client for the official Memobird thermal printer Web API. Bind a device,
print text as an image, or let the service render HTML or a URL, and poll until
the job is on paper.

## Install

```bash
go get github.com/ruhuang2001/memobird-go@v0.0.4
```

Requires Go 1.26.11 or newer.

## Quick start

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/ruhuang2001/memobird-go/memobird"
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

	printed, err := client.PrintHTML(ctx, "<h1>Hello, Memobird!</h1>")
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

| Package | Purpose | Extra dependency |
| --- | --- | --- |
| `memobird` | Web API client, binding, print calls, `WaitForPrint` | none |
| `textrender` | Pure Go text-to-PNG rendering for receipts and notes | `x/image`, `x/text` |

`memobird` depends only on the standard library. The module has no browser,
database, or configuration-file dependency: bindings are yours to persist, and
content that needs a browser is rendered by the caller and submitted with
`PrintImage`.

## Print paths

| Content | Call | Where rendering happens |
| --- | --- | --- |
| Plain HTML, simple CSS, Latin text | `PrintHTML` | Memobird service |
| Public URL | `PrintURL` | Memobird service |
| Generated text, including Chinese | `textrender` + `PrintImage` | In process |
| Any prepared PNG | `PrintImage` | Caller |

### Chinese text

Chinese printed through the service-side renderers does not come out correctly,
so render the text locally and print the image:

```go
png, err := textrender.RenderBase64PNG("今天要买的三样东西", textrender.Options{
	FontData: cjkFont, // any TTF or OTF with the glyphs; no embedded CJK font
})
if err != nil {
	log.Fatal(err)
}
if _, err := client.PrintImage(ctx, png); err != nil {
	log.Fatal(err)
}
```

### Binding

```go
resp, err := client.BindAndRemember(ctx, "your-user-identifying-string")
```

`SetUserID` / `GetUserID` read and write the bound user without the network. To
keep the binding across restarts, store `resp.UserID` yourself and set
`Config.UserID` on the next start.

### Waiting for the printer

`WaitForPrint` polls `GetPrintStatus` until `printflag` reports printed, then
returns the final status. It stops on context cancellation, on the configured
timeout, on API errors, and when the API has no record of the print content ID.

```go
status, err := client.WaitForPrint(ctx, printContentID, memobird.WaitOptions{
	Interval: time.Second,    // default 1s
	Timeout:  2 * time.Minute, // default 2m
})
```

A powered-off device keeps a job pending in the cloud and prints it after it
reconnects, so `Timeout` is the only bound on how long a caller waits.

### Text without a browser

```go
imgBase64, err := textrender.RenderBase64PNG("Hello from pure Go", textrender.Options{
	Width:    384,
	Padding:  16,
	FontSize: 20,
})
if err != nil {
	log.Fatal(err)
}

if _, err := client.PrintImage(ctx, imgBase64); err != nil {
	log.Fatal(err)
}
```

`textrender.Options` covers width, padding, font size, line height, max lines,
DPI, foreground and background colours, and custom TTF/OTF font data. The default
embedded font covers Latin; provide `FontData` for broader scripts.

Text is normalized before layout unless `Options.SkipNormalize` is set:
control characters, zero-width characters, byte-order marks, and non-breaking
spaces are removed or replaced, runs of spaces collapse, and more than one blank
line collapses to one. No visible character is ever removed, including one the
font cannot draw, because whether a glyph exists depends on the font the caller
chose.

Two calls help before printing paper:

```go
// Does the font cover this text? An uncovered rune draws as a blank box.
covered, _ := textrender.FontHasGlyphs(text, cjkFont)
if !covered {
	// Pick a font with the needed glyphs before rendering.
}

// Will it fit, and is MaxLines about to cut it short?
layout, _ := textrender.Measure(text, textrender.Options{FontData: cjkFont, MaxLines: 40})
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
patch release of this library. `go.mod` pins it deliberately: CI, `govulncheck`,
and local development all use the same compiler.

## License

MIT — see [LICENSE](LICENSE).
