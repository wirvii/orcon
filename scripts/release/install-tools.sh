#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Installs the pinned release tools into .bin/ (orcon-contributing RL-12, ADR 0081).
# Versions and sha256 sums come from tools/versions.mk; a download whose sum
# differs from the pin is rejected and nothing is installed.
#
# Usage: sh scripts/release/install-tools.sh goreleaser|cosign|gitsign|slsa-verifier ...
set -eu

root=$(cd "$(dirname "$0")/../.." && pwd)
versions=$root/tools/versions.mk
bin=$root/.bin

if [ "$#" -eq 0 ]; then
  echo "usage: install-tools.sh goreleaser|cosign|gitsign|slsa-verifier ..." >&2
  exit 2
fi

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) echo "install-tools: unsupported operating system $(uname -s)" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) echo "install-tools: unsupported architecture $(uname -m)" >&2; exit 1 ;;
esac

# pin NAME prints the value of "NAME := value" in tools/versions.mk.
pin() {
  value=$(sed -n "s/^$1 := //p" "$versions")
  if [ -z "$value" ]; then
    echo "install-tools: $1 is not set in tools/versions.mk" >&2
    exit 1
  fi
  printf '%s' "$value"
}

sha256_of() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  else
    shasum -a 256 "$1" | cut -d' ' -f1
  fi
}

# fetch URL SUM FILE downloads URL to FILE and checks its sha256.
fetch() {
  curl -fsSL --retry 3 -o "$3" "$1"
  got=$(sha256_of "$3")
  if [ "$got" != "$2" ]; then
    echo "install-tools: sha256 mismatch for $1" >&2
    echo "install-tools: want $2" >&2
    echo "install-tools: got  $got" >&2
    rm -f "$3"
    exit 1
  fi
}

install_tool() {
  tool=$1
  case "$tool" in
    goreleaser) prefix=GORELEASER ;;
    cosign) prefix=COSIGN ;;
    gitsign) prefix=GITSIGN ;;
    slsa-verifier) prefix=SLSA_VERIFIER ;;
    *) echo "install-tools: unknown tool $tool" >&2; exit 2 ;;
  esac
  version=$(pin "${prefix}_VERSION")
  sum=$(pin "${prefix}_SHA256_${os}_${arch}")
  want="$bin/.$tool-$version"
  if [ -f "$want" ] && [ -x "$bin/$tool" ]; then
    return 0
  fi
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT
  mkdir -p "$bin"
  case "$tool" in
    goreleaser)
      case "$arch" in amd64) garch=x86_64 ;; *) garch=$arch ;; esac
      case "$os" in linux) gos=Linux ;; *) gos=Darwin ;; esac
      fetch "https://github.com/goreleaser/goreleaser/releases/download/$version/goreleaser_${gos}_${garch}.tar.gz" "$sum" "$tmp/goreleaser.tar.gz"
      tar -xzf "$tmp/goreleaser.tar.gz" -C "$tmp" goreleaser
      mv "$tmp/goreleaser" "$bin/goreleaser"
      ;;
    cosign)
      fetch "https://github.com/sigstore/cosign/releases/download/$version/cosign-$os-$arch" "$sum" "$tmp/cosign"
      mv "$tmp/cosign" "$bin/cosign"
      ;;
    gitsign)
      fetch "https://github.com/sigstore/gitsign/releases/download/$version/gitsign_${version#v}_${os}_$arch" "$sum" "$tmp/gitsign"
      mv "$tmp/gitsign" "$bin/gitsign"
      ;;
    slsa-verifier)
      fetch "https://github.com/slsa-framework/slsa-verifier/releases/download/$version/slsa-verifier-$os-$arch" "$sum" "$tmp/slsa-verifier"
      mv "$tmp/slsa-verifier" "$bin/slsa-verifier"
      ;;
    *) echo "install-tools: unknown tool $tool" >&2; exit 2 ;;
  esac
  chmod +x "$bin/$tool"
  rm -f "$bin/.$tool-"*
  touch "$want"
  rm -rf "$tmp"
  trap - EXIT
  echo "install-tools: installed $tool $version"
}

for tool in "$@"; do
  install_tool "$tool"
done
