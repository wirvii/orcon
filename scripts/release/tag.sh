#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Creates, verifies and pushes the signed release tag after the release pull
# request merged (orcon-contributing RL-3, RL-4; ADR 0081 section 4). Runs only in
# the `tag` job of .github/workflows/release-please.yml, with the token of the
# release GitHub App in GH_TOKEN and an OpenID Connect token for gitsign.
#
# Usage: sh scripts/release/tag.sh   (HEAD is the merge commit on main)
set -eu

root=$(cd "$(dirname "$0")/../.." && pwd)
cd "$root"
repo=${GITHUB_REPOSITORY:-wirvii/orcon}
sha=${GITHUB_SHA:-$(git rev-parse HEAD)}

# Absolute path of gitsign: git runs gpg.x509.program from its own environment, where
# .bin is not on PATH.
if [ -x "$root/.bin/gitsign" ]; then
  gitsign=$root/.bin/gitsign
else
  gitsign=$(command -v gitsign) || {
    echo "tag: gitsign is not installed (sh scripts/release/install-tools.sh gitsign)" >&2
    exit 1
  }
fi

# 1. The merge commit changed the release manifest.
if ! git diff --name-only "$sha^" "$sha" | grep -qx '.release-please-manifest.json'; then
  echo "tag: $sha does not change .release-please-manifest.json; nothing to tag" >&2
  exit 1
fi
version=$(node -p 'require("./.release-please-manifest.json")["."]')
tag=v$version

# 2. The merged pull request is the release pull request of the bot.
# The pull requests associated with the merge commit come from the API, not from a search.
pr=$(gh api "repos/$repo/commits/$sha/pulls" \
  --jq 'first(.[] | select(.merged_at != null and .head.ref == "release-please--branches--main" and any(.labels[]; .name == "autorelease: pending")) | .number) // empty')
if [ -z "$pr" ]; then
  echo "tag: no merged release pull request (branch release-please--branches--main, label autorelease: pending) for $sha" >&2
  exit 1
fi

# 3. The tag does not exist yet; a published tag is never moved or recreated.
if git ls-remote --exit-code --tags origin "refs/tags/$tag" >/dev/null 2>&1; then
  echo "tag: $tag already exists" >&2
  exit 1
fi

# 4. Sign with the keyless identity of this workflow.
git config user.name "orcon-release[bot]"
git config user.email "orcon-release[bot]@users.noreply.github.com"
git config gpg.format x509
git config gpg.x509.program "$gitsign"
git config tag.gpgSign true

# 5. Create, 6. verify and 7. push the tag.
git tag -s "$tag" -m "Orcon $tag" "$sha"
"$gitsign" verify-tag "$tag" \
  --certificate-identity "https://github.com/$repo/.github/workflows/release-please.yml@refs/heads/main" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com"
git push origin "refs/tags/$tag"

# 8. Mark the release pull request as tagged.
gh pr edit "$pr" --repo "$repo" --remove-label "autorelease: pending" --add-label "autorelease: tagged"
echo "tag: $tag created, verified and pushed"
