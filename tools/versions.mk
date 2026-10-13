# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Pinned versions of the binary tools installed into .bin/ by `make tools`.
# Go-module tools are pinned as `tool` directives in tools/go.mod instead.
# To upgrade: change one pin, run `make check`, note any rule or output change.

GOLANGCI_LINT_VERSION := v2.14.0
SQLC_VERSION := v1.31.1
