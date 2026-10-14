# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Brand asset checks (SPEC-018). Included by the Makefile (mk/README.md).
# SPEC-018 section B1 expressly asks for brand-check to be a prerequisite of
# `check`; because this file is included at the end of the Makefile, the
# prerequisite runs after the ones the Makefile lists.

.PHONY: brand-check

check: brand-check

# brand-check enforces the brand asset rules (SPEC-018 BC-1..BC-6): brand SVGs are
# true vectors (element and attribute allowlist), wired copies are identical,
# mascot cut-outs are board pixels, the reference boards are unchanged and no
# unapproved asset reaches the app.
brand-check:
	cd tools && $(TOOLS_ENV) go build -o ../$(BIN)/brandcheck ./brandcheck
	$(BIN)/brandcheck all
