# Memobird Playground

A collection of tools for interacting with Memobird thermal printers.

## Features

- Print from URL
- Print HTML content

## Prerequisites

- Go 1.22+
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
# Print from URL
make print-url URL="https://example.com"

# Print HTML content
make print-html HTML="<html><body>Hello, Memobird!</body></html>"
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
        print from URL immediately and exit
  -print-html string
        print HTML content immediately and exit
  -version
        show version
```

### Print from URL

```bash
./memobird -config config.yaml -print-url "https://example.com"
```

### Print HTML Content

```bash
./memobird -config config.yaml -print-html "<html><body>Hello, Memobird!</body></html>"
```

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
│   ├── formatter/           # GBK Base64 encoding
│   ├── memobird/            # API client
│   └── storage/             # SQLite persistence
├── config.example.yaml
└── Makefile
```

## API Reference

The Memobird API client supports:

| Method | Description |
|--------|-------------|
| `BindUser` | Bind device to user identifier |
| `PrintFromURL` | Print webpage by URL |
| `PrintFromHTML` | Print HTML content |
| `GetPrintStatus` | Check print status |
| `ConvertToMonochrome` | Convert image to printable format |

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
