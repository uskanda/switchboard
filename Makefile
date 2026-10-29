# Verification entry point. `make verify` is the convention the harness looks for: the Stop hook `verify-gate`
# and the `verify` skill both call this target, and nothing is called done until it passes.
# This repository is still in the design phase: it has no code, no tests, and no CI yet.
SHELL := /usr/bin/env bash

.PHONY: verify verify-lint verify-test

verify: verify-lint verify-test ## Run every check that applies to this repository
	@echo "verify: ok"

verify-lint:
	@echo "[lint] skipped (no code yet; add the linter with the first code change)"

verify-test:
	@echo "[test] skipped (no test suite yet; add tests with the first code change)"
