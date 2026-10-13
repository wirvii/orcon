# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Operations targets (SPEC-015; ADR 0083). Included by the Makefile (mk/README.md).
# None of these targets is a prerequisite of `check`: they run in
# .github/workflows/ops.yml through `make ops-check`. Later cuts of SPEC-015 (O9, O10)
# add their targets here and to the prerequisites of ops-check.

.PHONY: docs-generate docs-generate-check ops-check

# REFERENCE_DIR is where the references generated from the code live.
REFERENCE_DIR ?= docs/reference

# docs-generate writes the references that come from the code: the configuration
# reference in English and Spanish and the JSON Schema of the configuration file.
docs-generate:
	go run ./cmd/orcon docs generate --out $(REFERENCE_DIR)

# docs-generate-check fails when a generated reference is not up to date with the
# code (E-CONF-3). It generates into a temporary directory and compares only the
# files the generator writes, because the directory also holds hand-written pages.
docs-generate-check:
	set -e; \
	tmp=$$(mktemp -d); \
	trap 'rm -rf "$$tmp"' EXIT INT TERM; \
	go run ./cmd/orcon docs generate --out "$$tmp" >/dev/null; \
	status=0; \
	for f in $$(cd "$$tmp" && find . -type f | sort); do \
		if ! diff -u "$(REFERENCE_DIR)/$$f" "$$tmp/$$f"; then \
			echo "docs-generate-check: $(REFERENCE_DIR)/$$f is out of date; run 'make docs-generate'" >&2; \
			status=1; \
		fi; \
	done; \
	exit $$status

# ops-check runs the operations checks that need no external service.
ops-check: docs-generate-check
