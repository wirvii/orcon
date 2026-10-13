// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"

	"github.com/wirvii/orcon/tools/internal/glob"
)

// SkillOrder lists the skills in the order of the rulebook table (PR-6).
var SkillOrder = []string{"orcon-ui", "orcon-web", "orcon-go", "orcon-db", "orcon-api", "orcon-connector"}

// SkillMap maps each skill to the path globs whose change makes its pull
// request checklist apply. It must equal the table PR-6 of
// .agents/skills/orcon-contributing/references/pull-requests.md;
// TestSkillMapMatchesRulebook enforces it.
var SkillMap = map[string][]string{
	"orcon-ui":  {"apps/web/**", "packages/js/tokens/**", "packages/js/ui/**", "packages/js/lint-config/**"},
	"orcon-web": {"apps/web/**", "packages/js/**"},
	"orcon-go":  {"**/*.go", "**/go.mod", "**/go.sum", "go.work", ".golangci.yml", "sqlc.yaml"},
	"orcon-db":  {"**/*.sql", "**/migrations/**", "internal/adapter/sqlite/**", "internal/adapter/postgres/**"},
	"orcon-api": {
		"packages/openapi/**", "internal/adapter/httpapi/**", "internal/adapter/mcpserver/**",
		"internal/adapter/cli/**", "internal/event/**",
	},
	"orcon-connector": {"internal/adapter/connectors/**", "packages/go/connector/**", "internal/connector/**"},
}

// ApplicableSkills returns, in rulebook order, the skills whose paths the
// changed files touch.
func ApplicableSkills(changed []string) []string {
	var skills []string
	for _, skill := range SkillOrder {
		if touches(SkillMap[skill], changed) {
			skills = append(skills, skill)
		}
	}
	return skills
}

func touches(globs, changed []string) bool {
	for _, path := range changed {
		if glob.MatchAny(globs, path) {
			return true
		}
	}
	return false
}

// MissingChecklists returns the applicable skills whose line "- [x] <skill>" is
// absent from the pull request body.
func MissingChecklists(body string, skills []string) []string {
	var missing []string
	for _, skill := range skills {
		if !strings.Contains(body, "- [x] "+skill) && !strings.Contains(body, "- [X] "+skill) {
			missing = append(missing, skill)
		}
	}
	return missing
}
