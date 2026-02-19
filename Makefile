.PHONY: build test clean run help

BINARY_NAME=rediscli
VERSION?=dev
GO=go
LDFLAGS=-ldflags "-s -w -X main.Version=$(VERSION)"

help:
	@echo "Available targets:"
	@echo "  build      - Build the binary"
	@echo "  test       - Run tests"
	@echo "  clean      - Clean build artifacts"
	@echo "  run        - Build and run"

build:
	$(GO) build $(LDFLAGS) -o $(BINARY_NAME) .

test:
	$(GO) test -v -race -coverprofile=coverage.out ./...

clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME)-* coverage.out

run: build
	./$(BINARY_NAME)
