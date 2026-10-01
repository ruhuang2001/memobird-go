.PHONY: test e2e fmt deps clean help

help:
	@echo "Memobird Playground"
	@echo ""
	@echo "Targets:"
	@echo "  make test   Run tests"
	@echo "  make e2e    Run local Chrome E2E and save artifacts"
	@echo "  make fmt    Format Go files"
	@echo "  make deps   Download and tidy dependencies"
	@echo "  make clean  Remove local database files"

test:
	go test -race ./...

e2e:
	bash e2e/run.sh

fmt:
	go fmt ./...

deps:
	go mod download
	go mod tidy

clean:
	rm -f *.db
