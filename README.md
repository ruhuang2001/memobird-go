# Memobird Go

A Go library for binding a Memobird device and printing through the official Memobird Web API.

## Packages

- `memobird`: Memobird Web API client and high-level printing workflows
- `config`: optional YAML and environment config loader
- `renderer`: optional local browser renderer for turning URL/HTML content into page images before printing
- `textrender`: optional pure Go text-to-PNG renderer for text-first printing without Chrome
- `storage`: optional SQLite binding store
- `formatter`: optional HTML-to-GBK helper used by the client

## Install

```bash
go get github.com/ruhuang2001/memobird-go
```

If the GitHub repository is still on the old `memobird-playground` slug during the rename window, update the repository name first so the module path and repository URL stay aligned.

## Quick start

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/ruhuang2001/memobird-go/memobird"
	"github.com/ruhuang2001/memobird-go/storage"
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
	store, err := storage.New("./memobird.db")
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

	if _, err := client.PrintHTML(ctx, "<h1>Hello, Memobird!</h1>"); err != nil {
		log.Fatal(err)
	}
}
```

## High-level workflows

`memobird.Client` is the main library entry point. It keeps the low-level API methods and the higher-level bind/print workflows in one place.

### Binding

```go
resp, err := client.BindAndRemember(ctx, "your-user-identifying-string")
resp, err := client.BindAndPersist(ctx, store, "your-user-identifying-string")
loaded, err := client.RestoreUserBinding(ctx, store)
```

### Remote print

```go
resp, err := client.PrintURL(ctx, "https://example.com")
resp, err := client.PrintHTML(ctx, "<p>Hello</p>")
```

### Local render then print

```go
r := renderer.New(30 * time.Second)
defer r.Close()

responses, err := client.PrintURLAsImages(ctx, r, "https://example.com")
```

For modern pages, dynamic dashboards, or non-trivial CSS, prefer `PrintURLAsImages` and `PrintHTMLAsImages`. They render locally and usually produce more predictable output than the Memobird server-side HTML/URL renderer.

## What `renderer` is for

`renderer` is not the Memobird API client. Its only job is local rendering:

- `renderer.RenderURLToImages(...)` opens a real browser and captures one or more screenshots of a page.
- `renderer.RenderHTMLToImages(...)` renders raw HTML in a real browser and captures screenshots.
- `memobird.Client.PrintURLAsImages(...)` and `PrintHTMLAsImages(...)` then post-process those screenshots and submit them to the Memobird API.

This package exists because browser rendering and Memobird API calls solve different problems:

- `memobird` knows how to bind users and send print jobs to the official API.
- `renderer` knows how to turn complex web content into printable images with better fidelity than the Memobird server-side renderer.
- the `renderer` URL entrypoints validate `http`/`https` URLs with a non-empty host before opening Chrome.

If you only need to print plain HTML through the Memobird API, you can skip `renderer` and call `PrintHTML(...)` or `PrintURL(...)`.

## Text-First Rendering Without Chrome

If your input is plain text, a summary, a receipt-style note, or another controlled template, you can skip Chrome entirely and use `textrender`.

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

`textrender` is a good fit when:

- the content starts as plain text instead of HTML
- you want deterministic layout without a browser dependency
- you are generating compact notes or fixed-format templates

Current `textrender` API:

- `RenderPNG(text, opts)` returns PNG bytes
- `RenderBase64PNG(text, opts)` returns a base64 PNG payload ready to pass to `client.PrintImage(...)`

`textrender.Options` supports width, padding, font size, line height, max lines, DPI, foreground/background colors, and optional custom TTF font data.

## Optional config loader

```go
cfg, err := config.Load("config.yaml")
if err != nil {
	log.Fatal(err)
}

client, err := memobird.NewClient(cfg.Memobird)
if err != nil {
	log.Fatal(err)
}
```

`config.Load` also supports environment-only startup.

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

`storage.Store` implements `memobird.BindingStore`, and other Go projects can provide their own store implementation as long as it exposes the same two methods.

## Example config

```yaml
memobird:
  access_key: "your-access-key"
  device_id: "your-device-id"
  user_id: 0
  timeout: 30s

storage:
  db_path: "./memobird.db"
```

## Notes

- `renderer` is optional and only needed for local URL/HTML rendering.
- `textrender` is optional and only needed for pure Go text-to-image rendering.
- `storage` is optional and only needed if you want to persist bindings.
- `memobird.Client` is safe for concurrent use after construction.
- `PrintURL` and `PrintHTML` use Memobird server-side rendering.
- `PrintURLAsImages` and `PrintHTMLAsImages` use local rendering and are the recommended path for modern pages.
- The high-level `memobird.Client` URL workflows only accept `http` and `https` URLs with a non-empty host.
- `renderer.RenderURLToImage(...)` and `renderer.RenderURLToImages(...)` also validate `http` and `https` URLs with a non-empty host.
- `renderer.ValidateURL(...)` is exported if you want to pre-validate URLs yourself.
- `textrender` uses an embedded Go font by default. For broader glyph coverage, including many non-Latin scripts, provide your own TTF in `textrender.Options.FontData`.

## Runtime requirements

- `PrintHTMLAsImages` and `PrintURLAsImages` require a locally available Chrome or Chromium browser because `renderer` uses `chromedp` under the hood.
- Headless Chrome must be able to start in the target environment. On minimal servers or containers, install Chromium and its required system libraries before using `renderer`.
- If you cannot provide Chrome/Chromium, stick to `PrintHTML` and `PrintURL`, but expect lower fidelity on modern pages because rendering happens on the Memobird side.

## Renderer Split

The rendering split is now:

- keep `memobird.Client` as the API-facing entry point
- use `renderer` for browser-based URL/HTML rendering
- use `textrender` for pure Go text-first image generation

## Project layout

```text
memobird-go/
├── config/
├── formatter/
├── memobird/
├── renderer/
├── textrender/
├── storage/
└── examples/
```
