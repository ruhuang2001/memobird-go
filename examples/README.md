# Examples

This directory contains small examples for using the Go library.

## Files

- `quickstart.sh`: shell wrapper that writes a temporary Go program with inline HTML and runs it against the library

## Example usage

The library is meant to be imported from Go code.

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/ruhuang2001/memobird-go/memobird"
	"github.com/ruhuang2001/memobird-go/renderer"
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

	r := renderer.New(30 * time.Second)
	defer r.Close()
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

	if _, err := client.PrintHTMLAsImages(ctx, r, "<h1>Hello</h1>"); err != nil {
		log.Fatal(err)
	}
}
```

The example uses `renderer`, so it expects Chrome or Chromium to be installed locally. If your environment does not provide a browser, switch to `client.PrintHTML(...)` or `client.PrintURL(...)`.
