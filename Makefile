.PHONY: build run test clean help

# Build variables
BINARY_NAME=memobird
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"

# Default target
help:
	@echo "Memobird Playground"
	@echo ""
	@echo "Usage:"
	@echo "  make build              Build the binary"
	@echo "  make run                Run the application"
	@echo "  make test               Run tests"
	@echo "  make clean              Clean build artifacts"
	@echo ""
	@echo "Commands:"
	@echo "  make bind USER=myuser      Bind device with user identifier"
	@echo "  make print-url URL=...     Print from URL (text mode)"
	@echo "  make print-html HTML=..   Print HTML content (text mode)"
	@echo "  make print-url-img URL=.. Print from URL as image"
	@echo "  make print-html-img HTML. Print HTML as image"

# Build the binary
build:
	go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd

# Run the application
run: build
	./$(BINARY_NAME) -config config.yaml

# Run tests
test:
	go test -v -race ./...

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -f *.db

# Bind device
bind: build
ifndef USER
	$(error USER is required. Usage: make bind USER=myuser)
endif
	./$(BINARY_NAME) -config config.yaml -bind $(USER)

# Print webpage from URL
print-url: build
ifndef URL
	$(error URL is required. Usage: make print-url URL='https://example.com')
endif
	./$(BINARY_NAME) -config config.yaml -print-url "$(URL)"

# Print raw HTML
print-html: build
ifndef HTML
	$(error HTML is required. Usage: make print-html HTML='<html>...</html>')
endif
	./$(BINARY_NAME) -config config.yaml -print-html "$(HTML)"

# Print webpage from URL as image
print-url-img: build
ifndef URL
	$(error URL is required. Usage: make print-url-img URL='https://example.com')
endif
	./$(BINARY_NAME) -config config.yaml -print-url-img "$(URL)"

# Print HTML as image
print-html-img: build
ifndef HTML
	$(error HTML is required. Usage: make print-html-img HTML='<html>...</html>')
endif
	./$(BINARY_NAME) -config config.yaml -print-html-img "$(HTML)"

# Download dependencies
deps:
	go mod download
	go mod tidy

# Format code
fmt:
	go fmt ./...
