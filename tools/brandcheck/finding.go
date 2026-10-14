// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"cmp"
	"fmt"
	"slices"
)

// Rule identifiers of SPEC-018 section 3.4.
const (
	ruleVector   = "BC-1" // brand SVGs are true vectors
	ruleCopies   = "BC-2" // wired copies are byte-identical to the canonical asset
	ruleSizes    = "BC-3" // PNG dimensions and ICO entries match the manifest
	ruleMascot   = "BC-4" // mascot cut-outs are board pixels over a transparent background
	ruleManifest = "BC-5" // every asset is listed, and wired assets are owner-approved
	ruleBoards   = "BC-6" // the reference boards are unchanged
)

// Finding is one violation. Line is 1 when the finding concerns a whole file
// rather than a place inside it.
type Finding struct {
	File   string
	Line   int
	Rule   string
	Detail string
}

// String renders the finding as "file:line: rule: detail".
func (f Finding) String() string {
	return fmt.Sprintf("%s:%d: %s: %s", f.File, f.Line, f.Rule, f.Detail)
}

// whole builds a finding about a whole file.
func whole(file, rule, format string, args ...any) Finding {
	return Finding{File: file, Line: 1, Rule: rule, Detail: fmt.Sprintf(format, args...)}
}

// sortFindings orders findings by file, line, rule and detail so output is
// deterministic.
func sortFindings(fs []Finding) {
	slices.SortFunc(fs, func(a, b Finding) int {
		return cmp.Or(
			cmp.Compare(a.File, b.File),
			cmp.Compare(a.Line, b.Line),
			cmp.Compare(a.Rule, b.Rule),
			cmp.Compare(a.Detail, b.Detail),
		)
	})
}
