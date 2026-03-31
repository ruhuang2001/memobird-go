# Memobird Playground

A Go library for binding a Memobird device and printing through the official Memobird Web API.

## Packages

- `memobird`: core client and high-level workflows
- `config`: optional YAML and environment config loader
- `renderer`: optional Chrome-based HTML/URL renderer
- `storage`: optional SQLite binding store
- `formatter`: optional HTML-to-GBK helper used by the client

## Install

```bash
go get github.com/ruhuang2001/memobird-playground
```

## Quick start

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/ruhuang2001/memobird-playground/memobird"
)

func main() {
	ctx := context.Background()

	client := memobird.NewClient(memobird.Config{
		AccessKey: "your-access-key",
		DeviceID:  "your-device-id",
		Timeout:   30 * time.Second,
	})

	if _, err := client.BindAndRemember(ctx, "your-user-identifying-string"); err != nil {
		log.Fatal(err)
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

## Optional config loader

```go
cfg, err := config.Load("config.yaml")
if err != nil {
	log.Fatal(err)
}

client := memobird.NewClient(cfg.Memobird)
```

`config.Load` also supports environment-only startup.

## Optional persistence

```go
store, err := storage.New(cfg.Storage.DBPath)
if err != nil {
	log.Fatal(err)
}
defer store.Close()
```

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
- `PrintURL` and `PrintHTML` use Memobird server-side rendering.
- `PrintURLAsImages` and `PrintHTMLAsImages` use local rendering and are the recommended path for modern pages.
- Only `http` and `https` URLs with a non-empty host are accepted.

## Project layout

```text
memobird-playground/
├── config/
├── formatter/
├── memobird/
├── renderer/
├── storage/
└── examples/
```
