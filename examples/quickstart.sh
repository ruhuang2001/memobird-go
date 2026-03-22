#!/usr/bin/env bash
set -euo pipefail

# Option 1: use config.yaml
./memobird -config config.yaml -print-html-img "$(cat examples/sample-note.html)"

# Option 2: env-only mode
# export MEMOBIRD_ACCESS_KEY="your-access-key"
# export MEMOBIRD_DEVICE_ID="your-device-id"
# export MEMOBIRD_USER_ID="12345"
# ./memobird -print-html-img "$(cat examples/sample-note.html)"
