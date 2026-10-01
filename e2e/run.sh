#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export MEMOBIRD_E2E=1
export MEMOBIRD_E2E_ARTIFACTS="$PWD/build/review-e2e"
mkdir -p "$MEMOBIRD_E2E_ARTIFACTS"
go list -deps ./memobird > "$MEMOBIRD_E2E_ARTIFACTS/client-dependencies.txt"
if grep -E 'github.com/chromedp/|memobird-go/renderer$' "$MEMOBIRD_E2E_ARTIFACTS/client-dependencies.txt"; then
  echo 'Core client unexpectedly depends on Chrome' >&2
  exit 1
fi
go test -race -count=1 -v ./renderer ./textrender ./e2e 2>&1 | tee "$MEMOBIRD_E2E_ARTIFACTS/test.log"
