.PHONY: build fmt fmt-check lint lint-config vet test race-test check mod-check script-check python-check vagrant-check cuse-check ci e2e

GOLANGCI_LINT := go tool golangci-lint
VAGRANT ?= vagrant
LOCAL_GOOS ?= $(shell go env GOOS)
LOCAL_GOARCH ?= $(shell go env GOARCH)
RPI_GOOS ?= linux
RPI_GOARCH ?= arm64
SHELLCHECK ?= shellcheck
PYTHON ?= python3
SHELL_SCRIPTS := scripts/hid-e2e.sh
PYTHON_TOOLS := tools/hci-proxy.py tools/bluez-agent.py tools/bluez-pair.py
CUSE_TOOL := tools/hidraw-cuse.c

build:
	mkdir -p dist
	GOOS=$(LOCAL_GOOS) GOARCH=$(LOCAL_GOARCH) go build -o dist/kbd ./cmd/kbd
	GOOS=$(RPI_GOOS) GOARCH=$(RPI_GOARCH) go build -o dist/kbd-rpi ./cmd/kbd-rpi
	GOOS=$(RPI_GOOS) GOARCH=$(RPI_GOARCH) go build -o dist/kbd-hid ./cmd/kbd-hid

fmt:
	$(GOLANGCI_LINT) fmt

fmt-check:
	$(GOLANGCI_LINT) fmt --diff

lint:
	$(GOLANGCI_LINT) run ./...

lint-config:
	$(GOLANGCI_LINT) config verify

vet:
	go vet ./...

test:
	go test ./...

race-test:
	go test -race ./...

check: lint-config fmt-check lint vet test

mod-check:
	go mod tidy
	git diff --exit-code -- go.mod go.sum

script-check:
	bash -n $(SHELL_SCRIPTS)
	$(SHELLCHECK) $(SHELL_SCRIPTS)

python-check:
	$(PYTHON) -c 'import pathlib, sys; [compile(pathlib.Path(path).read_text(), path, "exec") for path in sys.argv[1:]]' $(PYTHON_TOOLS)

vagrant-check:
	ruby -c Vagrantfile

cuse-check:
	pkg-config --exists fuse3
	cc -Wall -Wextra -fsyntax-only $$(pkg-config --cflags fuse3) $(CUSE_TOOL)

ci: check race-test build mod-check script-check python-check vagrant-check cuse-check

e2e:
	VAGRANT=$(VAGRANT) scripts/hid-e2e.sh
