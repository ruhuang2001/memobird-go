# Memobird Playground

A collection of tools for interacting with Memobird thermal printers.

## Features

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

### 4. Print

```bash
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

Image rendering includes:
- Automatic resizing to 384px width (thermal printer spec)
- Floyd-Steinberg dithering for high-contrast monochrome
- 2x scale factor for sharp text

## Configuration

See [config.example.yaml](config.example.yaml) for all options.

```yaml
memobird:
  access_key: "your-access-key"
  device_id: "your-device-id"
  user_id: 12345  # from -bind command

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
| `PrintFromURL` | Print webpage by URL (text mode) |
| `PrintFromHTML` | Print HTML content (text mode) |
| `PrintImage` | Print base64-encoded PNG image |
| `PrintImageProcessed` | Print pre-processed monochrome image |
| `GetPrintStatus` | Check print status |

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

// Process image for thermal printer (384px, monochrome, dithered)
processed, err := renderer.ProcessImageForPrint(imgBase64)

// Validate URL (http/https only)
err := renderer.ValidateURL("https://example.com")
```

## Environment Variables

Configuration can be overridden with environment variables:

| Variable | Description |
|----------|-------------|
| `MEMOBIRD_MEMOBIRD_ACCESS_KEY` | Access key |
| `MEMOBIRD_MEMOBIRD_DEVICE_ID` | Device ID |
| `MEMOBIRD_MEMOBIRD_USER_ID` | User ID |
| `MEMOBIRD_STORAGE_DB_PATH` | Database path |

## License

MIT
