#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# ci-changes.sh classifies the files a change touches into areas, so a workflow runs only
# the checks of those areas. It prints go, web, api, skills, docs and all as true/false and,
# when GITHUB_OUTPUT is set, appends them there.
#
# Inputs (environment): EVENT_NAME, BASE_SHA (pull request base or merge group base), HEAD
# (default HEAD). Fail safe: any event other than a pull request or a merge queue run, a
# missing base, a failed diff or a file that belongs to no known area turns every area on.
# Shared build inputs (workflows, Makefile, mk, go.mod, tools, scripts, lockfiles) also turn
# every area on.
set -u

go=false web=false api=false skills=false docs=false all=false

diff_files() {
	[ -n "${BASE_SHA:-}" ] || return 1
	git cat-file -e "$BASE_SHA^{commit}" 2>/dev/null || return 1
	git diff --name-only "$BASE_SHA" "${HEAD:-HEAD}" 2>/dev/null
}

case "${EVENT_NAME:-}" in
pull_request | merge_group) ;;
*) all=true ;;
esac

if [ "$all" = false ]; then
	if files=$(diff_files) && [ -n "$files" ]; then
		# File names are split on newlines only; the repository has no names with spaces.
		for f in $files; do
			case "$f" in
			.github/* | Makefile | mk/* | go.mod | go.sum | go.work | go.work.sum | tools/* | scripts/* | \
				package.json | pnpm-lock.yaml | pnpm-workspace.yaml | turbo.json | .golangci.yml | sqlc.yaml | \
				.spdx-exempt | .provenance-allow) all=true ;;
			cmd/* | internal/* | packages/go/* | test/* | bench/*) go=true ;;
			apps/web/* | packages/js/*) web=true ;;
			.agents/skills/orcon-ui/* | .agents/skills/orcon-web/*) web=true skills=true ;;
			packages/openapi/*) api=true ;;
			.agents/* | .*/skills/*) skills=true ;;
			docs/* | *.md | LICENSE | NOTICE) docs=true ;;
			*) all=true ;;
			esac
		done
	else
		echo "ci-changes: no usable diff, running everything" >&2
		all=true
	fi
fi

# The API contract feeds the Go server and the generated web SDK.
if [ "$api" = true ]; then
	go=true web=true
fi
if [ "$all" = true ]; then
	go=true web=true api=true skills=true docs=true
fi

out() {
	echo "$1=$2"
	[ -z "${GITHUB_OUTPUT:-}" ] || echo "$1=$2" >>"$GITHUB_OUTPUT"
}
out go "$go"
out web "$web"
out api "$api"
out skills "$skills"
out docs "$docs"
out all "$all"
