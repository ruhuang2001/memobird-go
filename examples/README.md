# Examples

This directory contains small examples for using the Go library.

## Files

- `sample-note.html`: printable HTML used by the examples
- `quickstart.sh`: shell wrapper that runs a tiny Go program against the library

## Example usage

The library is meant to be imported from Go code.

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/ruhuang2001/memobird-playground/memobird"
	"github.com/ruhuang2001/memobird-playground/renderer"
)

func main() {
	ctx := context.Background()
	client := memobird.NewClient(memobird.Config{
		AccessKey: "your-access-key",
		DeviceID:  "your-device-id",
		Timeout:   30 * time.Second,
	})

	r := renderer.New(30 * time.Second)
	defer r.Close()

	if _, err := client.BindAndRemember(ctx, "your-user-identifying-string"); err != nil {
		log.Fatal(err)
	}

	if _, err := client.PrintHTMLAsImages(ctx, r, "<h1>Hello</h1>"); err != nil {
		log.Fatal(err)
	}
}
```
