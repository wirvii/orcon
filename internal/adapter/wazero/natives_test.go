// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero_test

import (
	"strings"
)

// The inputs the covered natives are driven with. Each is built by fast native
// constructors, close to the largest that fits the 64 MiB memory cap of a
// runtime, so building it does not use the deadline.
const (
	inArray  = `const a = new Array(1e6).fill(1);`
	inArray2 = `const a = new Array(15e5).fill(1);`
	inStr    = `const s = 'a'.repeat(1e7);`
	inStrM   = `const s = 'a'.repeat(1e6);`
	inWide   = `const w = 'Ā'.repeat(4e6);`
	inSpaces = `const s = ' '.repeat(2e7);`
	inObj    = `const o = {}; for (let i = 0; i < 1e5; i++) o['k' + i] = i;`
	inSet    = `const st = new Set(new Array(3e5).keys());`
	inMap    = `const m = new Map(new Array(3e5).entries());`
	inF64    = `const t = new Float64Array(3e6);`
	inU8     = `const u = new Uint8Array(2e7);`
	inBuf    = `const b = new ArrayBuffer(2e7);`
	inJSON   = `const j = '[' + '1,'.repeat(2e6) + '1]';`
	inURI    = `const e = 'é'.repeat(2e6);`
)

// A nativeRow classifies natives of the guest. Covered natives are driven past
// the deadline twice, as one large call and inside an interpreted loop
// (TestSandboxIsolationAbandon); bounded natives do work bounded by a constant
// (the reason says why); interpreted natives only call functions written in
// JavaScript, which check the deadline by bytecode.
type nativeRow struct {
	class string
	// names are the inventory names; a {a b c} group expands to one name per
	// word.
	names string
	// setup builds the input; call is the expression, $M standing for the last
	// component of the name. loop, when set, replaces the call inside the
	// interpreted loop (for calls that grow or empty their input).
	setup, call, loop string
	why               string
}

const (
	covered     = "covered"
	bounded     = "bounded"
	interpreted = "interpreted"
)

// expandNames expands the first {a b c} group of a name list.
func expandNames(spec string) []string {
	var out []string
	for _, field := range splitSpec(spec) {
		i, j := strings.Index(field, "{"), strings.Index(field, "}")
		if i < 0 || j < i {
			out = append(out, strings.ReplaceAll(field, "~", " "))
			continue
		}
		for _, w := range strings.Fields(strings.ReplaceAll(field[i+1:j], ",", " ")) {
			out = append(out, strings.ReplaceAll(field[:i]+w+field[j+1:], "~", " "))
		}
	}
	return out
}

// splitSpec splits on spaces outside braces.
func splitSpec(spec string) []string {
	var out []string
	depth, start := 0, 0
	for i, r := range spec {
		switch {
		case r == '{':
			depth++
		case r == '}':
			depth--
		case r == ' ' && depth == 0:
			if i > start {
				out = append(out, spec[start:i])
			}
			start = i + 1
		}
	}
	if start < len(spec) {
		out = append(out, spec[start:])
	}
	return out
}

func lastComponent(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	return name
}

// nativeCase is one function driven past its deadline.
type nativeCase struct {
	name, kind, src string
}

// tableCases returns, for every covered native, the two cases of SPEC-001
// amendment 10 (section C-4.3): one large call (F2) and an interpreted loop of
// calls on a large input (F1).
func tableCases() []nativeCase {
	var out []nativeCase
	for _, row := range nativeTable {
		if row.class != covered {
			continue
		}
		for _, name := range expandNames(row.names) {
			call := strings.ReplaceAll(row.call, "$M", lastComponent(name))
			call = strings.ReplaceAll(call, "$O", strings.TrimSuffix(name, "."+lastComponent(name)))
			loop := call
			if row.loop != "" {
				loop = strings.ReplaceAll(row.loop, "$M", lastComponent(name))
			}
			out = append(out,
				nativeCase{name, "single", "(() => { " + row.setup + " $input.all(); " + call + "; return 0; })()"},
				nativeCase{name, "loop", "(() => { " + row.setup + " $input.all(); for (;;) { " + loop + "; } })()"})
		}
	}
	return out
}
