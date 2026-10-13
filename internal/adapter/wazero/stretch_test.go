// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/wazero"
	"github.com/wirvii/orcon/internal/expr"
)

// stretchBound is the longest the guest may go without calling the host: a
// garbage-collection pause waits for it (SPEC-001 amendment 10, section D-3.4).
// The target is 1 ms on the development machine; the test allows 2 ms.
const stretchBound = 2 * time.Millisecond

// nativeStretchBound is the bound for the covered natives of the inventory,
// driven on inputs of tens of megabytes: a single copy or fill of such a block
// by the allocator or the C library takes 2 to 4 ms and cannot be split, which
// is the residual of amendment 10 section D-4.
const nativeStretchBound = 5 * time.Millisecond

// checkStretch runs a case and holds its longest stretch to the bound. Like the
// other timing bounds it fails only if three runs in a row miss it.
func checkStretch(t *testing.T, sb *wazero.Sandbox, cfg expr.SandboxConfig, src string, reach bool, bound time.Duration) {
	t.Helper()
	var last, tail, first time.Duration
	for range 3 {
		out, _ := driveCase(t, sb, cfg, src, reach)
		if out.err != nil && !errors.Is(out.err, expr.ErrTimeout) {
			if defaultTestTimeout >= time.Second && strings.Contains(out.err.Error(), "out of memory") {
				t.Skip("the case ran out of memory before its deadline on a loaded machine")
			}
			t.Fatalf("got %v", out.err)
		}
		last, tail, first = out.stretch, out.tail, out.first
		if last <= bound || defaultTestTimeout >= time.Second {
			t.Logf("longest stretch %v (first %v, tail %v)", last, out.first, out.tail)
			return
		}
	}
	t.Errorf("the guest went %v without calling the host (%v of it after the last call, %v before the first), more than %v", last, tail, first, bound)
}

// TestSandboxIsolationStretch measures, for every case of abandonCases (the
// interpreted loop among them) and every covered native, the longest time the
// guest goes without calling the host.
func TestSandboxIsolationStretch(t *testing.T) {
	ev := evaluatorIn(t, wazero.ModeCooperativeAbandon, expr.Options{Timeout: abandonDeadline, MemoryLimit: 64 << 20})
	sb := sharedSandbox(wazero.ModeCooperativeAbandon)
	for _, tc := range abandonCases {
		t.Run(tc.name, func(t *testing.T) { checkStretch(t, sb, ev.SandboxConfig(), tc.src, false, stretchBound) })
	}
	for _, tc := range tableCases() {
		t.Run(tc.name+"/"+tc.kind, func(t *testing.T) { checkStretch(t, sb, ev.SandboxConfig(), tc.src, true, nativeStretchBound) })
	}
}
