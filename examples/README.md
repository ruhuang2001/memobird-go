# Examples

The library is meant to be imported from Go code.

## Files

- `quickstart.sh`: shell wrapper that writes a temporary Go program and runs it against a local API stand-in

## Example usage

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

	if _, err := client.BindAndRemember(ctx, "your-user-identifying-string"); err != nil {
		log.Fatal(err)
	}

	if _, err := client.PrintText(ctx, "Hello, Memobird!", textrender.Options{}); err != nil {
		log.Fatal(err)
	}
}
```

Every print goes through an image. `client.PrintText` renders the text first;
`client.PrintImage` takes a PNG you prepared. For Chinese, pass a font with the
needed glyphs as `textrender.Options.FontData`, because the bundled font covers
Latin only and `PrintText` returns `memobird.ErrMissingGlyphs` otherwise.
