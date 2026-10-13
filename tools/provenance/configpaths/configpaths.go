// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package configpaths lists the repository paths that hold the configuration
// of the assistants and tools used to work on Orcon. They are exempt from the
// provenance check (owner decision D-O4) and, being rewritten by tooling, from
// the SPDX header check. The package lives under tools/provenance because that
// tree is itself exempt: this is the one place that has to name the paths.
package configpaths

// Tooling are the tool-managed configuration paths, at any depth. They are
// outside both the provenance check and the SPDX check.
var Tooling = []string{
	".claude/**",
	".codex/**",
	".mneme/**",
	"**/AGENTS.md",
	"**/CLAUDE.md",
}

// Provenance are all the paths outside the provenance check: the tool-managed
// configuration, the project skills (which describe the tooling by design) and
// tools/provenance/** (the term list and its test messages).
var Provenance = append(append([]string{}, Tooling...), ".agents/skills/**", "tools/provenance/**")
