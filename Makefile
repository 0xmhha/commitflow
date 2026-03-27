BINARY_NAME := commitflow
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X github.com/0xmhha/commitflow/cmd.Version=$(VERSION) \
	-X github.com/0xmhha/commitflow/cmd.GitCommit=$(GIT_COMMIT) \
	-X github.com/0xmhha/commitflow/cmd.BuildDate=$(BUILD_DATE)"

.PHONY: build clean test lint install help

build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY_NAME) .

install:
	CGO_ENABLED=0 go install $(LDFLAGS) .

test:
	go test ./... -v -race

test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

lint:
	go vet ./...

clean:
	rm -rf bin/ coverage.out coverage.html

tidy:
	go mod tidy

help:
	@echo "Available targets:"
	@echo "  build       - Build the binary"
	@echo "  install     - Install the binary"
	@echo "  test        - Run tests with race detector"
	@echo "  test-cover  - Run tests with coverage report"
	@echo "  lint        - Run go vet"
	@echo "  clean       - Remove build artifacts"
	@echo "  tidy        - Run go mod tidy"

.DEFAULT_GOAL := build
