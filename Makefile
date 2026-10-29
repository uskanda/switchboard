# Verification entry point: `make verify` runs every check this repository has. CI runs the same target.
SHELL := /usr/bin/env bash

GO ?= go

.PHONY: verify verify-fmt verify-vet verify-test build

verify: verify-fmt verify-vet verify-test ## Run every check that applies to this repository
	@echo "verify: ok"

verify-fmt: ## Fail when a Go file is not gofmt-formatted
	@out="$$(gofmt -l $$(git ls-files '*.go'))"; \
	if [ -n "$$out" ]; then echo "[fmt] not formatted:"; echo "$$out"; exit 1; fi; \
	echo "[fmt] ok"

verify-vet:
	@$(GO) vet ./... && echo "[vet] ok"

verify-test:
	@$(GO) test ./... && echo "[test] ok"

build: ## Build bin/switchboard
	@$(GO) build -o bin/switchboard ./cmd/switchboard
