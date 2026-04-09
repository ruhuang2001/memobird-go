# Memobird Go

A Go library for binding a Memobird device and printing through the official Memobird Web API.

## Packages

- `memobird`: core client and high-level workflows
- `config`: optional YAML and environment config loader
- `renderer`: optional Chrome-based HTML/URL renderer
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

`memobird.Client` keeps both low-level API methods and convenience workflows in the same package.

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
- `storage` is optional and only needed if you want to persist bindings.
- `memobird.Client` is safe for concurrent use after construction.
- `PrintURL` and `PrintHTML` use Memobird server-side rendering.
- `PrintURLAsImages` and `PrintHTMLAsImages` use local rendering and are the recommended path for modern pages.
- Only `http` and `https` URLs with a non-empty host are accepted.

## Runtime requirements

- `PrintHTMLAsImages` and `PrintURLAsImages` require a locally available Chrome or Chromium browser because `renderer` uses `chromedp` under the hood.
- Headless Chrome must be able to start in the target environment. On minimal servers or containers, install Chromium and its required system libraries before using `renderer`.
- If you cannot provide Chrome/Chromium, stick to `PrintHTML` and `PrintURL`, but expect lower fidelity on modern pages because rendering happens on the Memobird side.

## Project layout

```text
memobird-go/
├── config/
├── formatter/
├── memobird/
├── renderer/
├── storage/
└── examples/
```
