#!/usr/bin/env bash
set -euo pipefail

cat <<'EOF' >/tmp/memobird_quickstart.go
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

	if _, err := client.PrintHTMLAsImages(ctx, r, "<h1>Hello, Memobird!</h1>"); err != nil {
		log.Fatal(err)
	}
}
EOF

go run /tmp/memobird_quickstart.go
rm -f /tmp/memobird_quickstart.go
