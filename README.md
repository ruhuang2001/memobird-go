# Memobird Playground

A collection of tools for interacting with Memobird thermal printers.

## Features

- Print plain text (GBK text mode, experimental)
- Print from URL (text mode)
- Print HTML content (text mode)
- **Print from URL as image** (better text rendering)
- **Print HTML as image** (full layout support)

## Prerequisites

- Go 1.24+
- Chrome or Chromium (for image rendering)
- A Memobird device
- Access Key from [open.memobird.cn](http://open.memobird.cn)

## Quick Start

### 1. Get Your Credentials

1. Register as a developer at [open.memobird.cn](http://open.memobird.cn/user/index)
2. Get your `access_key` after approval
3. Double-click your Memobird device to print the `device_id`

### 2. Configure

```bash
cp config.example.yaml config.yaml
# Edit config.yaml with your credentials
```

### 3. Bind Device

First-time setup requires binding your device:

```bash
make bind USER=my-unique-identifier

# Or manually
go run ./cmd -config config.yaml -bind my-unique-identifier
```

This will output a `user_id` - add it to your `config.yaml`.

If `user_id` remains `0`, the app will try to load a previously saved binding from `storage.db_path`
when the stored `device_id` matches your current device.

### 4. Print

```bash
# Print plain text (GBK text mode, experimental)
make print-text TEXT="你好，世界"

# Print from URL (text mode)
make print-url URL="https://example.com"

# Print HTML content (text mode)
make print-html HTML="<html><body>Hello, Memobird!</body></html>"

# Print from URL as image (better rendering)
make print-url-img URL="https://example.com"

# Print HTML as image (full layout support)
make print-html-img HTML="<html><body><h1>Hello!</h1></body></html>"
```

## Installation

```bash
# Clone the repository
git clone https://github.com/ruhuang2001/memobird-playground.git
cd memobird-playground

# Download dependencies
go mod download

# Build
make build

# Run tests
make test
```

## Usage

### Command Line Options

```bash
./memobird -h

  -config string
        path to config file
  -bind string
        bind user identifier (run once to get user_id)
  -print-text string
        print plain text immediately and exit (GBK text mode, experimental)
  -print-url string
        print from URL immediately and exit (text mode)
  -print-html string
        print HTML content immediately and exit (text mode)
  -print-url-img string
        render URL as image and print (better text rendering)
  -print-html-img string
        render HTML as image and print (full layout support)
  -version
        show version
```

### Print from URL

```bash
# Text mode (original API)
./memobird -config config.yaml -print-url "https://example.com"

# Image mode (better rendering)
./memobird -config config.yaml -print-url-img "https://example.com"
./memobird -config config.yaml -print-url-img "http://localhost:8080"
```

### Print Plain Text

```bash
./memobird -config config.yaml -print-text "你好，世界"
./memobird -config config.yaml -print-text "第一行\n第二行"
```

Note: `-print-text` is experimental. Some device models/firmware may print blank output.
For stable Chinese output, prefer `-print-html-img` (image mode).

### Print HTML Content

```bash
# Text mode (original API)
./memobird -config config.yaml -print-html "<html><body>Hello, Memobird!</body></html>"

# Image mode (full layout support)
./memobird -config config.yaml -print-html-img "<html><body><h1>Hello!</h1></body></html>"
```

### Rendering Modes

| Mode | Description | Use Case |
|------|-------------|----------|
| Text | Original API, faster | Simple text, plain content |
| Image | Renders to PNG, better quality | Complex layouts, styling, local content |

Image mode pipeline:
- Render URL/HTML to PNG with headless Chrome (400px capture width)
- Post-process image to 384px width (thermal printer spec)
- Apply Floyd-Steinberg dithering for high-contrast monochrome output
- Submit processed image to Memobird API for printer bitmap conversion

Scale-factor note:
- URL image mode uses a 2x Chrome device scale factor for sharper text capture
- HTML image mode uses Chrome default scale factor

## Configuration

See [config.example.yaml](config.example.yaml) for all options.

```yaml
memobird:
  access_key: "your-access-key"
  device_id: "your-device-id"
  user_id: 12345  # from -bind command (or leave 0 to load from local storage)

storage:
  db_path: "./memobird.db"
```

## Project Structure

```
memobird-playground/
├── cmd/
│   └── main.go              # Entry point
├── internal/
│   ├── config/              # Configuration loading
│   ├── formatter/           # GBK/UTF-8 Base64 encoding
│   ├── memobird/            # API client
│   ├── renderer/            # HTML/Image rendering (chromedp)
│   └── storage/             # SQLite persistence
├── config.example.yaml
└── Makefile
```

## API Reference

The Memobird API client supports:

| Method | Description |
|--------|-------------|
| `BindUser` | Bind device to user identifier |
| `PrintText` | Print plain text via `/home/printpaper` (`T:` + GBK Base64, experimental by device model) |
| `PrintFromURL` | Print webpage by URL (text mode) |
| `PrintFromHTML` | Print HTML content (text mode) |
| `ConvertToMonochrome` | Convert base64 image to printer signal format |
| `PrintImage` | Print base64-encoded PNG image |
| `PrintImageProcessed` | Print pre-processed monochrome image |
| `GetPrintStatus` | Check print status |
| `SetUserID` | Set active user ID in client |
| `GetUserID` | Get active user ID from client |

### Endpoint Mapping

| Endpoint | Client Method(s) | Notes |
|----------|------------------|-------|
| `/home/setuserbind` | `BindUser` | First-time device binding |
| `/home/printpaper` | `PrintText`, `PrintImage`, `PrintImageProcessed` | Uses `printcontent` prefix: `T:` text, `P:` image |
| `/home/printpaperFromUrl` | `PrintFromURL` | Server-side fetch; external assets may fail to load |
| `/home/printpaperFromHtml` | `PrintFromHTML` | Expects HTML payload encoded as GBK + Base64 |
| `/home/getSignalBase64Pic` | `ConvertToMonochrome` | Converts image to printer signal bitmap |
| `/home/getprintstatus` | `GetPrintStatus` | Poll print status by `printcontentid` |

### Compatibility Notes

- `-print-text` is experimental: some Memobird models/firmware accept request but print blank paper.
- For stable Chinese output, prefer image mode (`-print-html-img` / `-print-url-img`).
- `PrintFromHTML` now uses GBK Base64 as required by official docs, but service-side rendering can still vary.
- If text HTML mode renders Chinese incorrectly, use HTML entities (`&#20013;&#25991;`) or switch to image mode.
- `PrintFromURL` text mode depends on server-side page fetching; dynamic JS and some images may not render.

### CLI Coverage

CLI currently exposes printing and binding flows. `GetPrintStatus` and `ConvertToMonochrome` are available in the
`internal/memobird` client for programmatic integration but are not exposed as standalone CLI flags yet.

### Renderer Package

The `renderer` package provides HTML-to-image conversion:

```go
import "github.com/ruhuang2001/memobird-playground/internal/renderer"

// Create renderer with 30s timeout
r := renderer.New(30 * time.Second)

// Render URL to PNG (base64)
imgBase64, err := r.RenderURLToImage(ctx, "https://example.com")

// Render HTML to PNG (base64)
imgBase64, err := r.RenderHTMLToImage(ctx, "<html>...</html>")

// Process image for thermal printer (resize to 384px, then monochrome+dither)
processed, err := renderer.ProcessImageForPrint(imgBase64)

// Validate URL (http/https only)
err := renderer.ValidateURL("https://example.com")
```

## Environment Variables

Environment variables override values loaded from config file.

Note: the current loader still requires a readable config file (`config.yaml` by default, or `-config` path).

Configuration keys that support environment overrides:

| Variable | Description |
|----------|-------------|
| `MEMOBIRD_MEMOBIRD_ACCESS_KEY` | Access key |
| `MEMOBIRD_MEMOBIRD_DEVICE_ID` | Device ID |
| `MEMOBIRD_MEMOBIRD_USER_ID` | User ID |
| `MEMOBIRD_MEMOBIRD_BASE_URL` | API base URL |
| `MEMOBIRD_MEMOBIRD_TIMEOUT_SEC` | Request timeout in seconds |
| `MEMOBIRD_STORAGE_DB_PATH` | Database path |

## License

MIT
