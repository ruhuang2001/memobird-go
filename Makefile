.PHONY: test fmt deps clean help

help:
	@echo "Memobird Playground"
	@echo ""
	@echo "Targets:"
	@echo "  make test   Run tests"
	@echo "  make fmt    Format Go files"
	@echo "  make deps   Download and tidy dependencies"
	@echo "  make clean  Remove local database files"

test:
	go test -race ./...

fmt:
	go fmt ./...

deps:
	go mod download
	go mod tidy

clean:
	rm -f *.db
