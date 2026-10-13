// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero_test

import (
	"hash/fnv"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/wazero"
	"github.com/wirvii/orcon/internal/expr"
)

// coveredCalls lists, in table order, the covered natives with the code that
// drives one of them on a large input.
func coveredCalls() []fuzzCall {
	var out []fuzzCall
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
			out = append(out, fuzzCall{name: name, setup: row.setup, once: call, again: loop})
		}
	}
	return out
}

type fuzzCall struct {
	name, setup, once, again string
}

// pick returns the covered native called name; a name that is not one is
// mapped to a native by its hash, so the fuzzer can mutate the string freely.
func pick(calls []fuzzCall, name string) fuzzCall {
	for _, c := range calls {
		if c.name == name {
			return c
		}
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return calls[int(h.Sum32())%len(calls)]
}

// fuzzSource composes a case from one or two covered natives: each keeps its
// own input in a closure of its own, and the loop, or the single run, calls
// them in turn.
func fuzzSource(a, b fuzzCall, loop bool) string {
	var src strings.Builder
	src.WriteString("(() => {\n")
	for i, c := range []fuzzCall{a, b} {
		call := c.once
		if loop {
			call = c.again
		}
		src.WriteString("const f" + string(rune('0'+i)) + " = (() => { " + c.setup + " return () => { " + call + "; }; })();\n")
	}
	src.WriteString("$input.all();\n")
	if loop {
		src.WriteString("for (;;) { f0(); f1(); }\n")
	} else {
		src.WriteString("f0(); f1(); return 0;\n")
	}
	src.WriteString("})()")
	return src.String()
}

// FuzzSandboxAbandon composes short chains of covered natives, one or two per
// step, in a loop or as single calls, over large generated inputs, and holds
// them to the two bounds of ADR-0018 section 3: the caller has its timeout
// within the deadline plus 5 ms and the abandoned call stops within 100 ms. The
// seeds run in a normal `go test`; `go test -fuzz FuzzSandboxAbandon` explores
// further, and each finding becomes a case of abandonCases and a change of the
// patch (SPEC-001 amendment 10, section C-4.3).
func FuzzSandboxAbandon(f *testing.F) {
	for _, seed := range []struct {
		a, b string
		loop bool
	}{
		{"Array.prototype.unshift", "Array.prototype.shift", true},
		{"String.prototype.localeCompare", "String.prototype.concat", true},
		{"Object.keys", "Object.assign", true},
		{"Array.prototype.includes", "Array.prototype.copyWithin", true},
		{"String.prototype.normalize", "String.prototype.padStart", true},
		{"JSON.parse", "JSON.stringify", true},
		{"%TypedArray%.prototype.sort", "%TypedArray%.prototype.reverse", true},
		{"Uint8Array.prototype.toHex", "atob", true},
		{"Array.prototype.includes", "Array.prototype.lastIndexOf", true},
		{"String.prototype.replaceAll", "String.prototype.match", false},
	} {
		f.Add(seed.a, seed.b, seed.loop)
	}
	calls := coveredCalls()
	ev := evaluatorIn(f, wazero.ModeCooperativeAbandon, expr.Options{Timeout: abandonDeadline, MemoryLimit: 64 << 20})
	sb := sharedSandbox(wazero.ModeCooperativeAbandon)
	f.Fuzz(func(t *testing.T, a, b string, loop bool) {
		src := fuzzSource(pick(calls, a), pick(calls, b), loop)
		stopBeat := startHeartbeat()
		runAbandonCaseOpt(t, sb, ev.SandboxConfig(), src, true, loop, true)
		// The collector is held off during a run (see driveCase), so the
		// scheduler bound here is the heartbeat's; a single miss fails.
		if late := stopBeat(); defaultTestTimeout < time.Second && late > schedulerBound {
			t.Errorf("the heartbeat ran %v late, more than %v", late, schedulerBound)
		}
	})
}
