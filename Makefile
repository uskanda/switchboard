# Verification entry point: `make verify` runs every check this repository has. CI runs the same target.
SHELL := /usr/bin/env bash

GO ?= go

.PHONY: verify verify-fmt verify-vet verify-test verify-web build install restart deploy-check deploy

# The systemd units `switchboard service install` (and its --relay/--inbox/--web variants) put in place.
SERVICES := switchboard-relay.service switchboard-inbox.service switchboard-agent.service switchboard-web.service
PREFIX ?= $(HOME)/.local

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

install: build ## Build and copy both binaries to $(PREFIX)/bin, where the units run them from
	@install -D -m 0755 bin/switchboard $(PREFIX)/bin/switchboard
	@install -D -m 0755 bin/switchboard-web $(PREFIX)/bin/switchboard-web
	@echo "install: $(PREFIX)/bin"

# Units that are not installed on this host are skipped, so the target works with any subset.
restart: ## Restart every installed switchboard service (asks for the sudo password)
	@units="$$(for u in $(SERVICES); do systemctl cat "$$u" >/dev/null 2>&1 && echo "$$u"; done)"; \
	if [ -z "$$units" ]; then echo "restart: no switchboard service is installed"; exit 1; fi; \
	sudo systemctl restart $$units && systemctl --no-pager --lines=0 status $$units

# A deploy from another branch puts a build the main branch does not have into service. On a terminal the target
# asks before it builds; otherwise it warns and goes on.
deploy-check:
	@branch="$$(git rev-parse --abbrev-ref HEAD)"; \
	if [ "$$branch" != "main" ]; then \
		echo "deploy: warning: the checkout is on '$$branch', not main" >&2; \
		if [ -t 0 ]; then \
			read -r -p "deploy: deploy this branch anyway? [y/N] " answer; \
			[ "$$answer" = "y" ] || { echo "deploy: stopped" >&2; exit 1; }; \
		fi; \
	fi

deploy: deploy-check install restart ## Install the current build and restart every service
