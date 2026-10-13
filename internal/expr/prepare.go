// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package expr

import (
	"slices"
	"strings"

	"github.com/dop251/goja/parser"
)

// callAliases maps the extension method names that clash with a property of a
// library object to the name the prelude defines them under. Luxon gives
// DateTime an isWeekend getter, while n8n documents isWeekend as a method that
// is called. A call of such a name is renamed in the source and reading the
// property stays Luxon's, which is how n8n resolves the clash too (it rewrites
// calls in the syntax tree).
var callAliases = map[string]string{
	"isWeekend": "__isWeekend",
}

// extraMethods are method names of objects the expression context builds
// ($input, $('Name')), which no built-in prototype carries.
var extraMethods = []string{"all", "itemMatching"}

// trimCode removes the whitespace and statement terminators that follow the
// expression: n8n accepts {{ expr; }}.
func trimCode(code string) string {
	return strings.TrimRight(code, " \t\r\n;")
}

// prepareCode returns the code as the sandbox runs it: trimmed on both ends and
// with the aliased calls renamed.
func prepareCode(code string) string {
	code = strings.TrimLeft(trimCode(code), " \t\r\n")
	hit := false
	for name := range callAliases {
		if strings.Contains(code, name) {
			hit = true
			break
		}
	}
	if !hit {
		return code
	}
	const prefix = "(\n"
	prog, err := parser.ParseFile(nil, "", prefix+code+"\n)", 0)
	if err != nil {
		return code
	}
	w := &walker{base: len(prefix)}
	w.push(nil)
	for _, st := range prog.Body {
		w.stmt(st)
	}
	// Replace from the end so earlier offsets stay valid.
	slices.SortFunc(w.renames, func(a, b rename) int { return b.at - a.at })
	for _, r := range w.renames {
		if r.at >= 0 && strings.HasPrefix(code[r.at:], r.from) {
			code = code[:r.at] + r.to + code[r.at+len(r.from):]
		}
	}
	return code
}
