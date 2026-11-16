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

	if _, err := client.PrintHTML(ctx, "<h1>Hello, Memobird!</h1>"); err != nil {
		log.Fatal(err)
	}
}
```

For Chinese, render the text to an image with `textrender` and submit it with
`client.PrintImage`. The service-side renderers do not print Chinese correctly.
