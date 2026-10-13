#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Checks the output of a GoReleaser snapshot build in build/goreleaser/ (make
# release-check): six archives, checksums.txt with seven lines (six archives and
# the SBOM), a CycloneDX SBOM, a working `orcon --version` and, unless docker
# was skipped, the two local images.
#
# Usage: sh scripts/release/check-snapshot.sh
# RELEASE_SKIP (comma list, as in make) skips the image check when it has "docker".
set -eu

root=$(cd "$(dirname "$0")/../.." && pwd)
dist=$root/build/goreleaser
fail=0

problem() {
  echo "check-snapshot: $1" >&2
  fail=1
}

if [ ! -d "$dist" ]; then
  echo "check-snapshot: $dist does not exist; run make release-check" >&2
  exit 1
fi

# The six archives.
for target in linux_amd64 linux_arm64 darwin_amd64 darwin_arm64; do
  n=$(find "$dist" -maxdepth 1 -name "orcon_*_$target.tar.gz" | wc -l | tr -d ' ')
  [ "$n" = 1 ] || problem "want one orcon_*_$target.tar.gz, found $n"
done
for target in windows_amd64 windows_arm64; do
  n=$(find "$dist" -maxdepth 1 -name "orcon_*_$target.zip" | wc -l | tr -d ' ')
  [ "$n" = 1 ] || problem "want one orcon_*_$target.zip, found $n"
done

# Checksums: six archives plus the SBOM.
if [ -f "$dist/checksums.txt" ]; then
  n=$(wc -l <"$dist/checksums.txt" | tr -d ' ')
  [ "$n" = 7 ] || problem "checksums.txt has $n lines, want 7"
  grep -q '_sbom.cdx.json$' "$dist/checksums.txt" || problem "checksums.txt does not list the SBOM"
else
  problem "checksums.txt is missing"
fi

if ! grep -q '"bomFormat": *"CycloneDX"' "$root/dist/sbom/orcon.cdx.json" 2>/dev/null; then
  problem "dist/sbom/orcon.cdx.json is not a CycloneDX document"
fi

# The binary of the host platform reports the snapshot version.
case "$(uname -s)" in Linux) os=linux ;; Darwin) os=darwin ;; *) os= ;; esac
case "$(uname -m)" in x86_64 | amd64) arch=amd64 ;; arm64 | aarch64) arch=arm64 ;; *) arch= ;; esac
archive=$(find "$dist" -maxdepth 1 -name "orcon_*_${os}_${arch}.tar.gz" 2>/dev/null | head -n 1)
if [ -n "$os" ] && [ -n "$arch" ] && [ -n "$archive" ]; then
  version=${archive##*/orcon_}
  version=${version%_${os}_${arch}.tar.gz}
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT
  tar -xzf "$archive" -C "$tmp" orcon
  if ! out=$("$tmp/orcon" --version 2>&1); then
    problem "orcon --version failed: $out"
  else
    case "$out" in
      *"$version"*) ;;
      *) problem "orcon --version printed \"$out\", want it to contain $version" ;;
    esac
  fi
else
  echo "check-snapshot: no archive for this platform; skipping the --version check"
fi

# The local images, unless docker was skipped.
case ",${RELEASE_SKIP:-}," in
  *,docker,*) ;;
  *)
    if [ -n "${version:-}" ]; then
      for a in amd64 arm64; do
        docker image inspect "ghcr.io/wirvii/orcon:$version-$a" >/dev/null 2>&1 ||
          problem "image ghcr.io/wirvii/orcon:$version-$a does not exist locally"
      done
    fi
    ;;
esac

if [ "$fail" -ne 0 ]; then
  exit 1
fi
echo "check-snapshot: ok"
