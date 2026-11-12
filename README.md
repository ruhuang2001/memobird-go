# memobird-go

[![CI](https://github.com/ruhuang2001/memobird-go/actions/workflows/ci.yml/badge.svg)](https://github.com/ruhuang2001/memobird-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ruhuang2001/memobird-go.svg)](https://pkg.go.dev/github.com/ruhuang2001/memobird-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/ruhuang2001/memobird-go)](https://goreportcard.com/report/github.com/ruhuang2001/memobird-go)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A Go client for the official Memobird thermal printer Web API. Bind a device,
print text, HTML, URLs, or images, and poll until the job is on paper.

## Install

```bash
go get github.com/ruhuang2001/memobird-go@v0.0.1
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
| `memobird` | Web API client, binding, print workflows, `WaitForPrint` | none |
| `formatter` | HTML-to-GBK base64 encoding used by `PrintHTML` | none |
| `textrender` | Pure Go text-to-PNG rendering for receipts and notes | none |
| `config` | Optional YAML/environment loader | `spf13/viper` |
| `storage` | Optional SQLite binding store | `modernc.org/sqlite` |
| `renderer` | Optional local Chrome rendering of URLs and HTML | `chromedp`, Chrome |

Importing `memobird` alone does not pull in Chrome: `go list -deps ./memobird`
contains neither `chromedp` nor `renderer`, and `make e2e` asserts this.

## Choosing a print path

The API can render content on the Memobird side, or this library can render it
locally and submit page images. Pick by content type:

| Content | Call | Where rendering happens | Fidelity on modern pages |
| --- | --- | --- | --- |
| Plain HTML, simple CSS | `PrintHTML` | Memobird service | Good |
| Public URL | `PrintURL` | Memobird service | Basic |
| Modern web page | `PrintURLAsImages` + `renderer` | Local Chrome | Highest |
| HTML with web fonts | `PrintHTMLAsImages` + `renderer` | Local Chrome | Highest |
| Generated text | `textrender` + `PrintImage` | In process | Deterministic, no Chrome |

### Binding

```go
resp, err := client.BindAndRemember(ctx, "your-user-identifying-string")
resp, err := client.BindAndPersist(ctx, store, "your-user-identifying-string")
loaded, err := client.RestoreUserBinding(ctx, store)
```

`SetUserID` / `GetUserID` read and write the bound user without the network.

### Remote rendering

```go
resp, err := client.PrintURL(ctx, "https://example.com")
resp, err := client.PrintHTML(ctx, "<p>Hello</p>")
```

### Local rendering then printing

```go
r, err := renderer.NewWithConfig(renderer.Config{
	Timeout:     30 * time.Second,
	RenderDelay: 500 * time.Millisecond,
})
if err != nil {
	log.Fatal(err)
}
defer r.Close()

responses, err := client.PrintURLAsImages(ctx, r, "https://example.com")
```

`renderer.Config` fields:

| Field | Default | Meaning |
| --- | --- | --- |
| `Timeout` | `30s` | Deadline for one browser render |
| `RenderDelay` | `500ms` | Wait after load so late content settles |
| `DisableSandbox` | `false` | Pass `--no-sandbox`; needed in containers that block Chrome's namespace sandbox |

`renderer.New(timeout)` and `renderer.NewWithOptions(...)` remain available and
behave as before.

### Waiting for the printer

`WaitForPrint` polls `GetPrintStatus` until `printflag` reports printed, then
returns the final status. It stops on context cancellation, on the configured
timeout, and on API errors.

```go
status, err := client.WaitForPrint(ctx, printContentID, memobird.WaitOptions{
	Interval: time.Second, // default 1s
	Timeout:  2 * time.Minute, // default 2m
})
```

### Pacing multi-page jobs

A long page renders to several images, and each image is a separate print
submission. `PrintInterval` spaces them out so the vendor API is not hit back to
back:

```go
client, err := memobird.NewClient(memobird.Config{
	AccessKey:     "your-access-key",
	DeviceID:      "your-device-id",
	PrintInterval: 2 * time.Second, // zero = no delay
})
```

Pages are not retried automatically: print submissions are not idempotent, and a
retry could print a page twice. Inspect the returned `[]*PrintResponse` for the
pages that did succeed.

### Text without Chrome

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
DPI, foreground and background colours, and custom TTF font data. The default
embedded font covers Latin; provide `FontData` for broader scripts.

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
| `ErrBindingStoreRequired` | `BindAndPersist`, `RestoreUserBinding` with a nil store |
| `ErrRendererRequired` | `PrintURLAsImages`, `PrintHTMLAsImages` with a nil renderer |
| `ErrNoPagesRendered` | Printing when local rendering produced no images |

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

## Optional configuration

```go
cfg, err := config.Load("config.yaml")
if err != nil {
	log.Fatal(err)
}

client, err := memobird.NewClient(cfg.Memobird)
```

Example `config.yaml`:

```yaml
memobird:
  access_key: "your-access-key"
  device_id: "your-device-id"
  user_id: 0
  timeout: 30s

storage:
  db_path: "./memobird.db"
```

## Optional persistence

```go
store, err := storage.New(cfg.Storage.DBPath)
if err != nil {
	log.Fatal(err)
}
defer store.Close()

loaded, err := client.RestoreUserBinding(ctx, store)
if err != nil {
	log.Fatal(err)
}
if !loaded {
	if _, err := client.BindAndPersist(ctx, store, "your-user-identifying-string"); err != nil {
		log.Fatal(err)
	}
}
```

`storage.Store` implements `memobird.BindingStore`. Any type with
`SaveUserBinding(ctx, userID, deviceID)` and `GetUserBinding(ctx)` works instead.

## Concurrency

`memobird.Client` is safe for concurrent use once `NewClient` returns.
`SetUserID`, `SetUserAgent`, and all print methods may be called from multiple
goroutines. A `renderer.Renderer` is also safe for concurrent use, though each
render allocates a browser session.

## Runtime requirements

- `PrintHTMLAsImages` and `PrintURLAsImages` need a local Chrome or Chromium,
  because `renderer` drives it through `chromedp`.
- In minimal containers, install Chromium and its shared libraries, and set
  `DisableSandbox: true` if the container blocks Chrome's sandbox.
- HTML submitted to local rendering executes scripts. Use absolute resource
  URLs or an explicit `<base>`; the render delay lets async content settle.
- Without Chrome, use `PrintHTML`, `PrintURL`, or `textrender`.

## Development

```bash
make test    # go test -race ./...
make e2e     # real Chrome E2E against a local API stand-in
make lint    # golangci-lint
```

The E2E never contacts a printer. It writes PNGs, a submission receipt, the
`memobird` dependency list, and logs to `build/review-e2e/`.

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
