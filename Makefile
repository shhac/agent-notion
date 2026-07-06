BINARY := agent-notion
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test lint vet dev tidy

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/agent-notion

test:
	go test ./... -count=1

vet:
	go vet ./...

lint:
	golangci-lint run ./...

dev:
	go run ./cmd/agent-notion $(ARGS)

tidy:
	go mod tidy
