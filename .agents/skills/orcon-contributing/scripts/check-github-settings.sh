#!/bin/sh
# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Audits the GitHub repository settings and rulesets against
# assets/github-settings.expected.json (references/github-settings.md, GH-1).
# Prints one PASS or FAIL line per setting and exits 1 when any setting fails.
#
# Usage: sh .agents/skills/orcon-contributing/scripts/check-github-settings.sh
#
# Requires: an authenticated GitHub CLI (gh) with administration read access
# to the repository, and jq.
#
# Required approvals (GH-9): 0 while .github/CODEOWNERS does not exist; 1 with
# code owner review and last-push approval once it exists, so a half-applied
# change fails. Required status checks: only the checks whose workflow job
# exists in this checkout are expected.

set -u

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
skill_dir=$(CDPATH='' cd -- "$script_dir/.." && pwd)
expected="$skill_dir/assets/github-settings.expected.json"
repo_root=$(CDPATH='' cd -- "$skill_dir/../../.." && pwd)

for tool in gh jq; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "check-github-settings.sh: $tool is required" >&2
    exit 2
  fi
done
if ! gh auth status >/dev/null 2>&1; then
  echo "check-github-settings.sh: gh is not authenticated (run gh auth login)" >&2
  exit 2
fi

errors=0
fail() {
  echo "FAIL $*"
  errors=$((errors + 1))
}
pass() {
  echo "PASS $*"
}

# check <label> <expected> <actual>
check() {
  if [ "$2" = "$3" ]; then
    pass "$1 = $3"
  else
    fail "$1: expected $2, got $3"
  fi
}

exp() {
  jq -r "$1" "$expected"
}

repo=$(exp '.repository')

# ---------------------------------------------------------------- repository settings (GH-4)
repo_json=$(gh api "repos/$repo") || {
  echo "check-github-settings.sh: cannot read repos/$repo" >&2
  exit 2
}
for key in $(exp '.settings | keys[]'); do
  check "settings.$key" "$(exp ".settings.$key")" "$(printf '%s' "$repo_json" | jq -r ".$key")"
done

# ---------------------------------------------------------------- security (GH-7)
for key in $(exp '.security_and_analysis | keys[]'); do
  check "security_and_analysis.$key" "$(exp ".security_and_analysis.$key")" \
    "$(printf '%s' "$repo_json" | jq -r ".security_and_analysis.$key.status // \"unknown\"")"
done
check "private_vulnerability_reporting" "$(exp '.private_vulnerability_reporting')" \
  "$(gh api "repos/$repo/private-vulnerability-reporting" --jq '.enabled' 2>/dev/null || echo unknown)"
if gh api "repos/$repo/vulnerability-alerts" >/dev/null 2>&1; then
  dependabot=true
else
  dependabot=false
fi
check "dependabot_alerts" "$(exp '.dependabot_alerts')" "$dependabot"

# ---------------------------------------------------------------- Actions (GH-4)
actions_json=$(gh api "repos/$repo/actions/permissions/workflow" 2>/dev/null || echo '{}')
for key in $(exp '.actions_workflow_permissions | keys[]'); do
  check "actions.$key" "$(exp ".actions_workflow_permissions.$key")" "$(printf '%s' "$actions_json" | jq -r ".$key")"
done
check "actions.fork_pr_approval_policy" "$(exp '.fork_pr_approval_policy')" \
  "$(gh api "repos/$repo/actions/permissions/fork-pr-contributor-approval" --jq '.approval_policy' 2>/dev/null || echo unknown)"

# ---------------------------------------------------------------- rulesets (GH-5, GH-6, GH-9)
rulesets=$(gh api "repos/$repo/rulesets?includes_parents=false" 2>/dev/null || echo '[]')

# ruleset_detail <name>: the full ruleset JSON, or an empty string.
ruleset_detail() {
  id=$(printf '%s' "$rulesets" | jq -r --arg n "$1" '[.[] | select(.name == $n)][0].id // empty')
  [ -n "$id" ] && gh api "repos/$repo/rulesets/$id"
}

for name in $(exp '.rulesets | keys[]'); do
  detail=$(ruleset_detail "$name" || true)
  if [ -z "$detail" ]; then
    fail "ruleset $name: does not exist"
    continue
  fi
  pass "ruleset $name: exists"
  check "ruleset $name.target" "$(exp ".rulesets[\"$name\"].target")" "$(printf '%s' "$detail" | jq -r '.target')"
  check "ruleset $name.enforcement" "$(exp ".rulesets[\"$name\"].enforcement")" "$(printf '%s' "$detail" | jq -r '.enforcement')"
  check "ruleset $name.include" "$(exp ".rulesets[\"$name\"].include | sort | join(\",\")")" \
    "$(printf '%s' "$detail" | jq -r '.conditions.ref_name.include | sort | join(",")')"
  check "ruleset $name.rules" "$(exp ".rulesets[\"$name\"].rules | sort | join(\",\")")" \
    "$(printf '%s' "$detail" | jq -r '[.rules[].type] | sort | join(",")')"

  # Bypass list: empty for main; only the release App for release-tags.
  check "ruleset $name.bypass_actors" "$(exp ".rulesets[\"$name\"].bypass_actors | map(.actor_type) | join(\",\")")" \
    "$(printf '%s' "$detail" | jq -r '.bypass_actors | map(.actor_type) | join(",")')"
  slug=$(exp ".rulesets[\"$name\"].bypass_actors[0].app_slug // empty")
  if [ -n "$slug" ]; then
    app_id=$(gh api "apps/$slug" --jq '.id' 2>/dev/null || true)
    if [ -z "$app_id" ]; then
      fail "ruleset $name.bypass_actors: GitHub App $slug not found"
    else
      check "ruleset $name.bypass_actors app" "$app_id" "$(printf '%s' "$detail" | jq -r '.bypass_actors[0].actor_id')"
    fi
  fi

  pr_exp=$(exp ".rulesets[\"$name\"].pull_request // empty")
  if [ -n "$pr_exp" ]; then
    pr=$(printf '%s' "$detail" | jq -c '[.rules[] | select(.type == "pull_request")][0].parameters // {}')
    if [ -f "$repo_root/.github/CODEOWNERS" ]; then
      want=$(exp ".rulesets[\"$name\"].pull_request.required_approving_review_count_with_codeowners")
      check "ruleset $name.require_code_owner_review (CODEOWNERS present)" true "$(printf '%s' "$pr" | jq -r '.require_code_owner_review')"
      check "ruleset $name.require_last_push_approval (CODEOWNERS present)" true "$(printf '%s' "$pr" | jq -r '.require_last_push_approval')"
    else
      want=$(exp ".rulesets[\"$name\"].pull_request.required_approving_review_count")
    fi
    check "ruleset $name.required_approving_review_count" "$want" "$(printf '%s' "$pr" | jq -r '.required_approving_review_count')"
    for key in dismiss_stale_reviews_on_push required_review_thread_resolution; do
      check "ruleset $name.$key" "$(exp ".rulesets[\"$name\"].pull_request.$key")" "$(printf '%s' "$pr" | jq -r ".$key")"
    done
    check "ruleset $name.allowed_merge_methods" "$(exp ".rulesets[\"$name\"].pull_request.allowed_merge_methods | sort | join(\",\")")" \
      "$(printf '%s' "$pr" | jq -r '.allowed_merge_methods // [] | sort | join(",")')"
  fi

  sc_exp=$(exp ".rulesets[\"$name\"].required_status_checks // empty")
  if [ -n "$sc_exp" ]; then
    sc=$(printf '%s' "$detail" | jq -c '[.rules[] | select(.type == "required_status_checks")][0].parameters // {}')
    check "ruleset $name.strict_required_status_checks_policy" "$(exp ".rulesets[\"$name\"].required_status_checks.strict")" \
      "$(printf '%s' "$sc" | jq -r '.strict_required_status_checks_policy')"

    # Expected contexts: only checks whose workflow job exists in this checkout.
    want_list=$(
      exp ".rulesets[\"$name\"].required_status_checks.checks[] | [.context, .workflow, .job] | @tsv" |
        while IFS="$(printf '\t')" read -r context workflow job; do
          wf="$repo_root/.github/workflows/$workflow"
          if [ -f "$wf" ] && grep -Eq "^  $job:[[:space:]]*$" "$wf"; then
            printf '%s\n' "$context"
          fi
        done | sort -u
    )
    have_list=$(printf '%s' "$sc" | jq -r '.required_status_checks // [] | .[].context' | sort -u)
    integration_id=$(exp ".rulesets[\"$name\"].required_status_checks.integration_id")
    printf '%s\n' "$want_list" | while IFS= read -r context; do
      [ -z "$context" ] && continue
      if printf '%s\n' "$have_list" | grep -qxF "$context"; then
        source=$(printf '%s' "$sc" | jq -r --arg c "$context" '.required_status_checks[] | select(.context == $c) | .integration_id')
        if [ "$source" = "$integration_id" ]; then
          echo "PASS ruleset $name required check '$context' (GitHub Actions)"
        else
          echo "FAIL ruleset $name required check '$context': source is $source, expected GitHub Actions ($integration_id)"
        fi
      else
        echo "FAIL ruleset $name required check '$context': missing (its job exists on this branch)"
      fi
    done >"${TMPDIR:-/tmp}/orcon-gh-settings.$$"
    printf '%s\n' "$have_list" | while IFS= read -r context; do
      [ -z "$context" ] && continue
      if ! printf '%s\n' "$want_list" | grep -qxF "$context"; then
        echo "FAIL ruleset $name required check '$context': not expected (no such job in the workflows, or missing from the expected list)"
      fi
    done >>"${TMPDIR:-/tmp}/orcon-gh-settings.$$"
    while IFS= read -r line; do
      case "$line" in
      PASS*) echo "$line" ;;
      FAIL*)
        echo "$line"
        errors=$((errors + 1))
        ;;
      esac
    done <"${TMPDIR:-/tmp}/orcon-gh-settings.$$"
    rm -f "${TMPDIR:-/tmp}/orcon-gh-settings.$$"
  fi
done

if [ "$errors" -gt 0 ]; then
  echo "github settings: $errors setting(s) differ from references/github-settings.md"
  exit 1
fi
echo "github settings: all PASS"
