.PHONY: fmt lint lint-config test check

GOLANGCI_LINT := go tool golangci-lint
GO_FILES := $(shell find . -name '*.go' -not -path './.git/*')

fmt:
	$(GOLANGCI_LINT) fmt

lint:
ifeq ($(strip $(GO_FILES)),)
	@echo "no Go files to lint"
else
	$(GOLANGCI_LINT) run ./...
endif

lint-config:
	$(GOLANGCI_LINT) config verify

test:
ifeq ($(strip $(GO_FILES)),)
	@echo "no Go files to test"
else
	go test ./...
endif

check: lint-config lint test
