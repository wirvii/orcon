#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Rebuilds quickjs.wasm (ADR-0018 section 3). The committed binary is the
# reviewed output of this script and CI does not rebuild it, so run the script
# only when a pinned input below changes:
#
#   sh internal/adapter/wazero/assets/build.sh
#
# Inputs, all downloaded into a temporary directory and checked against the
# SHA-256 digests below before use:
#   wasi-sdk 34.0       Apache-2.0 with LLVM exception (build toolchain only)
#   QuickJS-ng 0.17.0   MIT (linked into quickjs.wasm), plus quickjs-interrupt-poll.patch
#
# It also needs Go, to run tools/qjsloops, which writes native-loops.txt.
#
# The Luxon bundle is vendored separately by internal/expr/prelude/vendor-luxon.sh.
#
# WASI_SDK_PLATFORM and WASI_SDK_SHA256 select the toolchain archive of the
# machine that runs the build (the default is arm64-macos). The produced
# module does not depend on the build machine.
set -eu

WASI_SDK_PLATFORM="${WASI_SDK_PLATFORM:-arm64-macos}"
WASI_SDK_URL="https://github.com/WebAssembly/wasi-sdk/releases/download/wasi-sdk-34/wasi-sdk-34.0-${WASI_SDK_PLATFORM}.tar.gz"
WASI_SDK_SHA256="${WASI_SDK_SHA256:-9c59398106b417f8f14913380fdf0097a8cc0ff4af9eb3ce0065a859e88d49e9}"
QUICKJS_URL="https://github.com/quickjs-ng/quickjs/archive/refs/tags/v0.17.0.tar.gz"
QUICKJS_SHA256="559bc4c420475e55c7ab4510adbc562f55d7524d75e8e89d79ce4bb02f5687d9"

here=$(cd "$(dirname "$0")" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT INT TERM

fetch() { # url sha256 file
  curl -fsSL -o "$3" "$1"
  echo "$2  $3" | shasum -a 256 -c - >/dev/null
}

mkdir "$work/sdk" "$work/qjs"
fetch "$WASI_SDK_URL" "$WASI_SDK_SHA256" "$work/sdk/sdk.tgz"
fetch "$QUICKJS_URL" "$QUICKJS_SHA256" "$work/qjs/qjs.tgz"
tar -xzf "$work/sdk/sdk.tgz" -C "$work/sdk"
tar -xzf "$work/qjs/qjs.tgz" -C "$work/qjs"

sdk=$(ls -d "$work"/sdk/wasi-sdk-*/)
src=$(ls -d "$work"/qjs/quickjs-*/)

# Orcon patch: poll the interrupt handler from the native loops of the
# built-ins whose work grows with their input and charge native work to the
# interrupt counter (SPEC-001 amendment 10, sections A.4 and C).
patch -s -p1 -d "$src" < "$here/quickjs-interrupt-poll.patch"

# Inventory of the C functions of the patched source that have a loop bounded
# by the input or a memory move of variable length, and whether each polls or
# charges the counter. It is versioned (native-loops.txt), so a change of the
# patch or of the QuickJS-ng version shows up in review.
root=$(cd "$here/../../../.." && pwd)
(cd "$root/tools" && GOWORK=off go run ./qjsloops -src "${src}quickjs.c" -out "$here/native-loops.txt")

"${sdk}bin/clang" --target=wasm32-wasip1 --sysroot="${sdk}share/wasi-sysroot" \
  -O2 -DNDEBUG -fno-exceptions -D_GNU_SOURCE \
  -mexec-model=reactor \
  -I"$src" \
  "$here/orcon_qjs.c" "$src/quickjs.c" "$src/libregexp.c" "$src/libunicode.c" "$src/dtoa.c" \
  -Wl,--no-entry -Wl,--strip-all -Wl,-z,stack-size=1048576 \
  -o "$here/quickjs.wasm"

cp "${src}LICENSE" "$here/LICENSE.quickjs"
shasum -a 256 "$here/quickjs.wasm"
