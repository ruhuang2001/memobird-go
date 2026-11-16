#!/usr/bin/env bash
set -euo pipefail

TMPFILE="$(mktemp "${TMPDIR:-/tmp}/memobird_quickstart.XXXXXX.go")"
trap 'rm -f "$TMPFILE"' EXIT

cat <<'EOF' >"$TMPFILE"
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
EOF

go run "$TMPFILE"
