.PHONY: build fmt lint lint-config test check

GOLANGCI_LINT := go tool golangci-lint
LOCAL_GOOS ?= $(shell go env GOOS)
LOCAL_GOARCH ?= $(shell go env GOARCH)
RPI_GOOS ?= linux
RPI_GOARCH ?= arm64

build:
	mkdir -p dist
	GOOS=$(LOCAL_GOOS) GOARCH=$(LOCAL_GOARCH) go build -o dist/kbd ./cmd/kbd
	GOOS=$(RPI_GOOS) GOARCH=$(RPI_GOARCH) go build -o dist/kbd-rpi ./cmd/kbd-rpi
	GOOS=$(RPI_GOOS) GOARCH=$(RPI_GOARCH) go build -o dist/kbd-hid ./cmd/kbd-hid

fmt:
	$(GOLANGCI_LINT) fmt

lint:
	$(GOLANGCI_LINT) run ./...

lint-config:
	$(GOLANGCI_LINT) config verify

test:
	go test ./...

check: lint-config lint test
