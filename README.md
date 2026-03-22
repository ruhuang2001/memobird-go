# Memobird Playground

A small Go CLI for binding a Memobird device and printing via the official Memobird Web API.

## What This Project Supports

- Bind device to a user identifier
- Print from URL via Memobird server-side rendering
- Print raw HTML via Memobird text/HTML endpoint
- Print URL as image via local Chrome rendering
- Print HTML as image via local Chrome rendering

Recommended default:
- Prefer `-print-url-img` and `-print-html-img`
- `-print-url` and `-print-html` depend on Memobird server-side rendering and modern pages may submit successfully but still print blank paper

## What This Project Intentionally Does Not Support

Plain `-print-text` mode has been removed.

Reason:
- The official PDF still documents `/home/printpaper` with `printcontent=T:...`.
- In practice, that path is unreliable on current devices/firmware and can accept the request but print blank paper.
- This project now keeps the more stable paths only.

Official reference:
- Memobird Web API PDF: <https://open.memobird.cn/upload/webapi.pdf>

## Prerequisites

- Go 1.24+
- Chrome or Chromium for image rendering
- Memobird device
- Access key from <http://open.memobird.cn>

## Quick Start

### 1. Configure credentials

```bash
cp config.example.yaml config.yaml
```

Or use environment variables:

```bash
export MEMOBIRD_ACCESS_KEY="your-access-key"
export MEMOBIRD_DEVICE_ID="your-device-id"
export MEMOBIRD_USER_ID="12345"
export MEMOBIRD_BASE_URL="http://open.memobird.cn"
export MEMOBIRD_TIMEOUT_SEC="30"
export MEMOBIRD_STORAGE_DB_PATH="./memobird.db"
```

### 2. Bind the device

```bash
make bind USER=my-unique-identifier
```

Or:

```bash
go run ./cmd -config config.yaml -bind my-unique-identifier
```

If `memobird.user_id` is `0`, the CLI will try to reuse the locally stored binding when the stored `device_id` matches the current device.

### 3. Print

```bash
# URL via Memobird server-side rendering
# Warning: may submit successfully but print blank paper on modern pages
make print-url URL="https://example.com"

# HTML via Memobird server-side rendering
# Warning: may submit successfully but print blank paper on current devices/firmware
make print-html HTML="<html><body>Hello, Memobird!</body></html>"

# Recommended: URL rendered locally to image, then printed
make print-url-img URL="https://example.com"

# Recommended: HTML rendered locally to image, then printed
make print-html-img HTML="<html><body><h1>Hello!</h1></body></html>"
```

## Supported Commands

```bash
./memobird -h

  -bind string
        bind user identifier (run once to get user_id)
  -config string
        path to config file
  -print-html string
        print HTML via Memobird server-side rendering (may print blank on current devices/firmware; prefer -print-html-img)
  -print-html-img string
        render HTML locally as image and print (recommended)
  -print-url string
        print from URL via Memobird server-side rendering (modern pages may submit successfully but print blank; prefer -print-url-img)
  -print-url-img string
        render URL locally as image and print (recommended)
  -version
        show version
```

Notes:
- `-print-url` and `-print-html` depend on Memobird server-side rendering.
- Modern webpages may submit successfully but still print blank paper in those modes.
- Prefer `-print-url-img` or `-print-html-img` for modern pages and reliable output.

Only one action flag may be used at a time. For example, `-print-url` and `-print-html` are mutually exclusive.

## Rendering Modes

### Remote text/HTML mode

- `-print-url`
- `-print-html`

These rely on Memobird's server-side endpoints.
They can return a successful submission while still producing blank paper for modern webpages or current device/firmware combinations.
Use them only for simpler content or when you specifically need the server-side path.

### Local image mode

- `-print-url-img`
- `-print-html-img`

Pipeline:
- Render with headless Chrome at 400px capture width
- Resize to 384px printer width
- Trim trailing blank rows from paginated screenshots to avoid wasting paper on white tail space
- Convert to 1-bit monochrome with Floyd-Steinberg dithering
- Submit image to Memobird for printer bitmap conversion

Use image mode when you need better Chinese rendering or layout fidelity.
Use image mode first when you are printing modern webpages and want the most reliable output.

## Configuration

See [config.example.yaml](/Users/ruhuang/Code/Github/memobird-playground/config.example.yaml).

```yaml
memobird:
  access_key: "your-access-key"
  device_id: "your-device-id"
  user_id: 12345

storage:
  db_path: "./memobird.db"
```

Configuration precedence:

1. Built-in defaults
2. `config.yaml`
3. Environment variables

Behavior notes:
- If you do not pass `-config`, env-only startup is supported.
- If you explicitly pass `-config /path/to/file.yaml`, that file must exist.

## Examples

- [examples/README.md](/Users/ruhuang/Code/Github/memobird-playground/examples/README.md)
- [examples/sample-note.html](/Users/ruhuang/Code/Github/memobird-playground/examples/sample-note.html)
- [examples/quickstart.sh](/Users/ruhuang/Code/Github/memobird-playground/examples/quickstart.sh)

## Project Structure

```text
memobird-playground/
├── cmd/
├── internal/
│   ├── config/
│   ├── formatter/
│   ├── memobird/
│   ├── renderer/
│   └── storage/
├── config.example.yaml
└── Makefile
```

## API Coverage

| Endpoint | Client Method | Notes |
|----------|---------------|-------|
| `/home/setuserbind` | `BindUser` | device binding |
| `/home/printpaperFromUrl` | `PrintFromURL` | Memobird server-side URL rendering; modern pages may still print blank |
| `/home/printpaperFromHtml` | `PrintFromHTML` | expects GBK + Base64 payload; may still print blank on current devices/firmware |
| `/home/getSignalBase64Pic` | `ConvertToMonochrome` | image conversion helper |
| `/home/printpaper` | `PrintImage` | image print path using `P:` prefix |
| `/home/getprintstatus` | `GetPrintStatus` | print status polling |

## Troubleshooting

- `user_id not configured`: run `make bind USER=...` first, or set `memobird.user_id` / `MEMOBIRD_USER_ID`.
- `config file ... not found`: you passed `-config` explicitly and the file does not exist.
- Rendering failures: ensure Chrome or Chromium is installed and runnable.
- URL validation failed: only `http` and `https` URLs with a non-empty host are accepted.
- `-print-url` / `-print-html` submitted successfully but printed blank paper: expected on some current Memobird server-side rendering paths; retry with `-print-url-img` or `-print-html-img`.
