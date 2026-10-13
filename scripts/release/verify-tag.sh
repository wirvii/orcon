#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Verifies a release tag before anything is published (orcon-contributing RL-4,
# ADR 0081): it is an annotated tag, its gitsign signature carries the identity
# of the release-please workflow on main and the GitHub Actions issuer, and the
# tagged commit is an ancestor of origin/main.
#
# Usage: sh scripts/release/verify-tag.sh vX.Y.Z   (needs .bin/gitsign; make release-tools)
# REPO overrides the repository (default wirvii/orcon) for rehearsals in a fork.
set -eu

if [ "$#" -ne 1 ]; then
  echo "usage: verify-tag.sh vX.Y.Z" >&2
  exit 2
fi
tag=$1
repo=${REPO:-wirvii/orcon}
root=$(cd "$(dirname "$0")/../.." && pwd)
cd "$root"

gitsign=gitsign
[ -x "$root/.bin/gitsign" ] && gitsign=$root/.bin/gitsign

kind=$(git cat-file -t "refs/tags/$tag")
if [ "$kind" != tag ]; then
  echo "verify-tag: $tag is a $kind, want an annotated tag" >&2
  exit 1
fi

identity="https://github.com/$repo/.github/workflows/release-please.yml@refs/heads/main"
issuer=https://token.actions.githubusercontent.com
if ! "$gitsign" verify-tag "$tag" --certificate-identity "$identity" --certificate-oidc-issuer "$issuer"; then
  echo "verify-tag: the signature of $tag does not match $identity" >&2
  exit 1
fi

git fetch --quiet origin main
if ! git merge-base --is-ancestor "$tag^{commit}" origin/main; then
  echo "verify-tag: the commit of $tag is not an ancestor of origin/main" >&2
  exit 1
fi
echo "verify-tag: $tag ok"
