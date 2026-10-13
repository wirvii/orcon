#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Verifies a published release the way a user does (docs/releases/verifying.md;
# orcon-contributing RL-8, ADR 0081): signature and sums of the checksum file, the
# six archives and the SBOM, SLSA provenance of every archive, and the image
# signature, provenance and SBOM attestation.
#
# Usage: sh scripts/release/verify.sh vX.Y.Z [owner/repo]
# Needs gh (authenticated), cosign, slsa-verifier and docker buildx in PATH or .bin/.
# The default repository is wirvii/orcon; pass another to rehearse in a fork.
set -eu

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
  echo "usage: verify.sh vX.Y.Z [owner/repo]" >&2
  exit 2
fi
tag=$1
repo=${2:-wirvii/orcon}
version=${tag#v}
root=$(cd "$(dirname "$0")/../.." && pwd)
PATH=$root/.bin:$PATH
export PATH

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cd "$work"

issuer=https://token.actions.githubusercontent.com
identity="https://github.com/$repo/.github/workflows/release.yml@refs/tags/$tag"
image="ghcr.io/$repo"

echo "verify: downloading $repo $tag"
gh release download "$tag" --repo "$repo"

echo "verify: signature of checksums.txt"
cosign verify-blob checksums.txt \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity "$identity" \
  --certificate-oidc-issuer "$issuer"

echo "verify: checksums"
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum -c checksums.txt
else
  shasum -a 256 -c checksums.txt
fi

echo "verify: archives and SBOM present"
count=0
for target in linux_amd64 linux_arm64 darwin_amd64 darwin_arm64; do
  [ -f "orcon_${version}_$target.tar.gz" ] || { echo "verify: missing orcon_${version}_$target.tar.gz" >&2; exit 1; }
  count=$((count + 1))
done
for target in windows_amd64 windows_arm64; do
  [ -f "orcon_${version}_$target.zip" ] || { echo "verify: missing orcon_${version}_$target.zip" >&2; exit 1; }
  count=$((count + 1))
done
[ "$count" = 6 ] || { echo "verify: expected 6 archives" >&2; exit 1; }
[ -f "orcon_${version}_sbom.cdx.json" ] || { echo "verify: missing orcon_${version}_sbom.cdx.json" >&2; exit 1; }

echo "verify: SLSA provenance of every archive"
for archive in orcon_"${version}"_*.tar.gz orcon_"${version}"_*.zip; do
  slsa-verifier verify-artifact "$archive" \
    --provenance-path "orcon_$version.intoto.jsonl" \
    --source-uri "github.com/$repo" \
    --source-tag "$tag"
done

echo "verify: image"
digest=$(docker buildx imagetools inspect "$image:$version" --format '{{json .Manifest.Digest}}' | tr -d '"')
cosign verify "$image:$version" \
  --certificate-identity "$identity" \
  --certificate-oidc-issuer "$issuer" >/dev/null
cosign verify-attestation "$image@$digest" --type cyclonedx \
  --certificate-identity "$identity" \
  --certificate-oidc-issuer "$issuer" >/dev/null
slsa-verifier verify-image "$image@$digest" \
  --source-uri "github.com/$repo" \
  --source-tag "$tag"

echo "release $tag: verified"
