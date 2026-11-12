.PHONY: test e2e lint fmt vet tidy deps clean help

help:
	@echo "memobird-go"
	@echo ""
	@echo "Targets:"
	@echo "  make test   Run tests with the race detector"
	@echo "  make e2e    Run local Chrome E2E and save artifacts in build/review-e2e"
	@echo "  make lint   Run golangci-lint"
	@echo "  make fmt    Format Go files"
	@echo "  make vet    Run go vet"
	@echo "  make tidy   Tidy and verify go.mod/go.sum"
	@echo "  make deps   Download dependencies"
	@echo "  make clean  Remove local database and build artifacts"

test:
	go test -race ./...

e2e:
	bash e2e/run.sh

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

tidy:
	go mod tidy

deps:
	go mod download

clean:
	rm -f *.db
	rm -rf build
