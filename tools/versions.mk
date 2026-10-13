# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Pinned versions of the binary tools installed into .bin/ by `make tools`.
# Go-module tools are pinned as `tool` directives in tools/go.mod instead.
# To upgrade: change one pin, run `make check`, note any rule or output change.

GOLANGCI_LINT_VERSION := v2.14.0
SQLC_VERSION := v1.31.1

# Release tooling (orcon-contributing, ADR 0081), installed by scripts/release/install-tools.sh
# into .bin/. Each tool has a version and the sha256 of the downloaded file per platform
# (goreleaser: the .tar.gz archive; the others: the binary). The installer refuses a
# file with another sum. To upgrade, change the version and all four sums together.
GORELEASER_VERSION := v2.18.3
GORELEASER_SHA256_linux_amd64 := b8dee4728bca1aba0d71a832a17cd0a3799f75372878e7ffa36aef2343bce0bc
GORELEASER_SHA256_linux_arm64 := fa7011a7f8de3b504326b8552cf21c708ed3089519b01d372da32ef4e8d298d7
GORELEASER_SHA256_darwin_amd64 := dbc39347d9ca25a2da9bcc03633f39130bb3a67c9f2a0da86ba15b8b00bf86ce
GORELEASER_SHA256_darwin_arm64 := 821c58ab8c31d57bb331cab9f1a703eb3673bff4fce0db9cc3c4a038af14f76f

COSIGN_VERSION := v3.1.3
COSIGN_SHA256_linux_amd64 := 4629c757b7618056f8ddd7e2625ae9fdd94c0372a65049520bc7d9df9efc7f71
COSIGN_SHA256_linux_arm64 := c5d324e091826b0d7a78eb16fef316450b4eb9aaec045611c08ba06f5e73220a
COSIGN_SHA256_darwin_amd64 := 2347488e5d5b25336644024dfeca5601b190e91197a71a917bda44744aff106c
COSIGN_SHA256_darwin_arm64 := 5cf948c2f4dfe59687bdd0b8523709067383e03982cc543475c8a7dc70e92a76

GITSIGN_VERSION := v0.17.1
GITSIGN_SHA256_linux_amd64 := 69213a8a0813a151e5a47d0060862952ff833a845d57309dff76f7ba6600abae
GITSIGN_SHA256_linux_arm64 := 477018736a80b36e703dd58db8d6e158a2c1b8b727af0ab8ffdcce9fdf610ada
GITSIGN_SHA256_darwin_amd64 := 32c469eee2859694a67df0efee1a17f2574600ad73769a81315f360ee75f89e7
GITSIGN_SHA256_darwin_arm64 := 4994e9629aae6a3fea9c0f770f279b273bad3e86ac8ebd3e9dc90f317428589d

SLSA_VERIFIER_VERSION := v2.7.1
SLSA_VERIFIER_SHA256_linux_amd64 := 946dbec729094195e88ef78e1734324a27869f03e2c6bd2f61cbc06bd5350339
SLSA_VERIFIER_SHA256_linux_arm64 := 5d3b2349ede7bfec19e7a21569f18b9f7410145ad12e9584b175370669e14061
SLSA_VERIFIER_SHA256_darwin_amd64 := 4baf25415727821f847a38bccedc86c3e5b17cbfc2eb534cd554feb6c856d6f1
SLSA_VERIFIER_SHA256_darwin_arm64 := 39abfcf5f1d690c3e889ce3d2d6a8b87711424d83368511868d414e8f8bcb05c
