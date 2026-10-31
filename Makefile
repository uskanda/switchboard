# Verification entry point: `make verify` runs every check this repository has. CI runs the same target.
SHELL := /usr/bin/env bash

GO ?= go

.PHONY: verify verify-fmt verify-vet verify-test verify-web build

verify: verify-fmt verify-vet verify-test verify-web ## Run every check that applies to this repository
	@echo "verify: ok"

verify-fmt: ## Fail when a Go file is not gofmt-formatted
	@out="$$(gofmt -l $$(git ls-files --cached --others --exclude-standard '*.go'))"; \
	if [ -n "$$out" ]; then echo "[fmt] not formatted:"; echo "$$out"; exit 1; fi; \
	echo "[fmt] ok"

verify-vet:
	@$(GO) vet ./... && echo "[vet] ok"

verify-test:
	@$(GO) test ./... && echo "[test] ok"

# web/ is a module of its own (the web inbox): the front end is checked and built first, then embedded in Go.
verify-web:
	@cd web/ui && pnpm install --frozen-lockfile --silent && pnpm typecheck && pnpm lint && pnpm test && pnpm build
	@cd web && $(GO) vet ./... && $(GO) test ./... && echo "[web] ok"

build: ## Build bin/switchboard and bin/switchboard-web
	@$(GO) build -o bin/switchboard ./cmd/switchboard
	@cd web/ui && pnpm install --frozen-lockfile --silent && pnpm build
	@cd web && $(GO) build -o ../bin/switchboard-web ./cmd/switchboard-web
