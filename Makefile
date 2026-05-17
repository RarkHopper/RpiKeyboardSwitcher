.PHONY: all clean build fmt fmt-check lint lint-config vet test race-test check mod-check script-check python-fmt python-check python-runtime-check packer-check vagrant-check packer-utm-plugin vagrant-utm-plugin e2e-box cuse-check ci e2e

GOLANGCI_LINT := go tool golangci-lint
PACKER ?= packer
UTM_APP ?= /Applications/UTM.app
QEMU_IMG ?= $(shell find -L "$(UTM_APP)" -path '*/qemu-img.framework/Versions/*/qemu-img' -type f 2>/dev/null | head -n 1)
VAGRANT ?= vagrant
LOCAL_GOOS ?= $(shell go env GOOS)
LOCAL_GOARCH ?= $(shell go env GOARCH)
RPI_GOOS ?= linux
RPI_GOARCH ?= arm64
SHELLCHECK ?= shellcheck
UV ?= uv
TOOLS_PYTHON ?= 3.12
TOOLS_DIR := tools
TOOLS_UV := $(UV) --project $(TOOLS_DIR) --directory $(TOOLS_DIR)
SHELL_SCRIPTS := scripts/hid-e2e.sh scripts/install-packer-utm-plugin.sh scripts/install-vagrant-utm-plugin.sh scripts/provision-e2e-vm.sh
PYTHON_TOOLS := hci-proxy.py bluez-agent.py bluez-pair.py
PYTHON_SOURCES := $(PYTHON_TOOLS) lib stubs
CUSE_TOOL := tools/hidraw-cuse.c
E2E_BOX_NAME ?= rpi-keyboard-switcher/e2e-ubuntu-24.04-arm64
E2E_BOX_FILE ?= dist/boxes/rpi-keyboard-switcher-e2e-utm.box
E2E_BOX_STAMP ?= dist/boxes/.rpi-keyboard-switcher-e2e-utm.added
E2E_BOX_INPUTS := packer/e2e-utm.pkr.hcl packer/cloud-init/meta-data packer/cloud-init/network-config packer/cloud-init/user-data scripts/provision-e2e-vm.sh tools/pyproject.toml tools/uv.lock
PACKER_UTM_PLUGIN_STAMP ?= dist/packer/.packer-utm-plugin-v4.0.0.installed

all: build

clean:
	rm -rf dist tools/.mypy_cache tools/.ruff_cache tools/.venv tools/__pycache__ tools/lib/__pycache__

build:
	mkdir -p dist
	GOOS=$(LOCAL_GOOS) GOARCH=$(LOCAL_GOARCH) go build -o dist/kbd ./cmd/kbd
	GOOS=$(RPI_GOOS) GOARCH=$(RPI_GOARCH) go build -o dist/kbd-rpi ./cmd/kbd-rpi
	GOOS=$(RPI_GOOS) GOARCH=$(RPI_GOARCH) go build -o dist/kbd-hid ./cmd/kbd-hid

fmt:
	$(GOLANGCI_LINT) fmt
	$(TOOLS_UV) run --locked --managed-python --python $(TOOLS_PYTHON) ruff format $(PYTHON_SOURCES)

fmt-check:
	$(GOLANGCI_LINT) fmt --diff
	$(TOOLS_UV) run --locked --managed-python --python $(TOOLS_PYTHON) ruff format --check $(PYTHON_SOURCES)

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

check: lint-config fmt-check lint vet test python-check

mod-check:
	go mod tidy
	git diff --exit-code -- go.mod go.sum

script-check:
	bash -n $(SHELL_SCRIPTS)
	$(SHELLCHECK) $(SHELL_SCRIPTS)

python-fmt:
	$(TOOLS_UV) run --locked --managed-python --python $(TOOLS_PYTHON) ruff format $(PYTHON_SOURCES)

python-check:
	$(TOOLS_UV) lock --check --python $(TOOLS_PYTHON) --managed-python
	$(TOOLS_UV) run --locked --managed-python --python $(TOOLS_PYTHON) ruff check $(PYTHON_SOURCES)
	$(TOOLS_UV) run --locked --managed-python --python $(TOOLS_PYTHON) mypy $(PYTHON_SOURCES)
	$(TOOLS_UV) run --locked --managed-python --python $(TOOLS_PYTHON) pyright
	$(TOOLS_UV) run --locked --managed-python --python $(TOOLS_PYTHON) python -m compileall -q $(PYTHON_SOURCES)

python-runtime-check:
	$(TOOLS_UV) run --locked --managed-python --python $(TOOLS_PYTHON) --extra runtime --no-dev python -c 'import dbus; import gi; from gi.repository import GLib; print(GLib.MainLoop)'

packer-check:
	$(PACKER) fmt -check packer
	$(PACKER) init packer
	$(PACKER) validate packer

vagrant-check:
	ruby -c Vagrantfile

$(PACKER_UTM_PLUGIN_STAMP): scripts/install-packer-utm-plugin.sh
	PACKER=$(PACKER) scripts/install-packer-utm-plugin.sh
	mkdir -p $(dir $@)
	touch $@

packer-utm-plugin: $(PACKER_UTM_PLUGIN_STAMP)

vagrant-utm-plugin:
	VAGRANT=$(VAGRANT) scripts/install-vagrant-utm-plugin.sh

$(E2E_BOX_FILE): $(E2E_BOX_INPUTS)
	mkdir -p $(dir $@)
	$(PACKER) init packer
	PACKER=$(PACKER) scripts/install-packer-utm-plugin.sh
	PATH="$(dir $(QEMU_IMG)):$$PATH" $(PACKER) build -force packer/e2e-utm.pkr.hcl

$(E2E_BOX_STAMP): $(E2E_BOX_FILE)
	$(VAGRANT) box add --force --name $(E2E_BOX_NAME) $(E2E_BOX_FILE)
	mkdir -p $(dir $@)
	touch $@

e2e-box: $(E2E_BOX_STAMP)

ifeq ($(LOCAL_GOOS),linux)
cuse-check:
	pkg-config --exists fuse3
	cc -Wall -Wextra -fsyntax-only $$(pkg-config --cflags fuse3) $(CUSE_TOOL)
else
cuse-check:
	@echo "skip cuse-check: fuse3 CUSE check requires Linux ($(LOCAL_GOOS))"
endif

ci: check race-test build mod-check script-check python-runtime-check packer-check vagrant-check cuse-check

e2e: vagrant-utm-plugin e2e-box
	VAGRANT=$(VAGRANT) scripts/hid-e2e.sh
