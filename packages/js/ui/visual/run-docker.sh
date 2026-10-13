#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Runs the visual suite inside the pinned Playwright container, the only place whose
# screenshots are valid baselines (fonts and rasterization differ per operating system).
#
# Usage:
#   pnpm --filter @orcon/ui test:visual:docker                          compare with the baselines
#   pnpm --filter @orcon/ui test:visual:docker -- --update-snapshots    regenerate them
#   pnpm --filter @orcon/ui test:visual:docker -- --project=pseudo      only the pseudo-locale check
#
# The package script builds Storybook on the host first. Storybook's output is the same on every
# platform, so only the browser has to be the pinned one: the container gets the built
# storybook-static (read only) and the visual/ folder (baselines and the failure report are
# written back there), installs @playwright/test at the pinned version, and runs the suite.
#
# Environment:
#   ORCON_CONTAINER_ENGINE   docker (default) or podman
#   ORCON_VISUAL_PLATFORM    linux/amd64 (default, the architecture of the CI runners)
#
# Exit status: the status of the test run; 1 when no container engine can be reached.

set -eu

# Keep both in step with the @playwright/test version in packages/js/ui/package.json.
PLAYWRIGHT_VERSION="1.64.0"
PLAYWRIGHT_IMAGE="mcr.microsoft.com/playwright:v${PLAYWRIGHT_VERSION}-noble"

engine="${ORCON_CONTAINER_ENGINE:-docker}"
platform="${ORCON_VISUAL_PLATFORM:-linux/amd64}"

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
ui_dir=$(CDPATH='' cd -- "$script_dir/.." && pwd)

if ! command -v "$engine" >/dev/null 2>&1; then
	echo "run-docker.sh: '$engine' is not installed. Install Docker, or run this job in CI, where the pinned container is available." >&2
	exit 1
fi
if ! "$engine" info >/dev/null 2>&1; then
	echo "run-docker.sh: '$engine' is installed but its daemon is not reachable. Start it, or run this job in CI." >&2
	exit 1
fi
if [ ! -f "$ui_dir/storybook-static/index.json" ]; then
	echo "run-docker.sh: storybook-static is missing. Run 'pnpm --filter @orcon/ui build-storybook' first." >&2
	exit 1
fi

# pnpm forwards the "--" that separates its own options from the script arguments; drop it.
if [ "${1:-}" = "--" ]; then
	shift
fi
args=""
for arg in "$@"; do
	args="$args '$arg'"
done

mkdir -p "$script_dir/__screenshots__"

exec "$engine" run --rm --ipc=host --platform "$platform" \
	-v "$script_dir:/work/visual" \
	-v "$ui_dir/storybook-static:/work/storybook-static:ro" \
	-e CI=1 \
	-e HOST_UID="$(id -u)" \
	-e HOST_GID="$(id -g)" \
	"$PLAYWRIGHT_IMAGE" \
	sh -c "
		set -eu
		cd /work
		npm init -y >/dev/null 2>&1
		npm pkg set type=module >/dev/null
		npm install --no-audit --no-fund --loglevel=error @playwright/test@${PLAYWRIGHT_VERSION}
		status=0
		npx playwright test -c visual/playwright.config.ts $args || status=\$?
		# Files written through the mount belong to the person who ran the script, not to root.
		chown -R \"\$HOST_UID:\$HOST_GID\" visual 2>/dev/null || true
		exit \$status
	"
