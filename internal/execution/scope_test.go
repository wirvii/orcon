// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/execution"
)

func TestScopePathPushAndPop(t *testing.T) {
	t.Parallel()
	var s execution.ScopePath
	if s.Depth() != 0 || s.Frames() != nil {
		t.Fatalf("top-level scope has depth %d and frames %v, want none", s.Depth(), s.Frames())
	}
	s = s.Push(execution.LoopFrame("outer", 2)).Push(execution.MultiInstanceFrame("mi", 0)).Push(execution.LoopFrame("inner", 11))
	if want := execution.ScopePath("L:outer#2/MI:mi#0/L:inner#11"); s != want {
		t.Fatalf("path = %q, want %q", s, want)
	}
	if s.Depth() != 3 {
		t.Errorf("Depth() = %d, want 3", s.Depth())
	}
	wantFrames := []execution.ScopeFrame{
		execution.LoopFrame("outer", 2), execution.MultiInstanceFrame("mi", 0), execution.LoopFrame("inner", 11),
	}
	if diff := cmp.Diff(wantFrames, s.Frames()); diff != "" {
		t.Errorf("Frames() mismatch (-want +got):\n%s", diff)
	}
	parent, frame, ok := s.Pop()
	if !ok || frame != execution.LoopFrame("inner", 11) || parent != "L:outer#2/MI:mi#0" {
		t.Errorf("Pop() = (%q, %+v, %v), want the innermost frame removed", parent, frame, ok)
	}
	top, ok := s.PopN(3)
	if !ok || top != "" {
		t.Errorf("PopN(3) = (%q, %v), want the top level", top, ok)
	}
	if _, ok := s.PopN(4); ok {
		t.Error("PopN(4) on a 3-frame path succeeded, want false")
	}
	if _, _, ok := execution.ScopePath("").Pop(); ok {
		t.Error("Pop() on the top level succeeded, want false")
	}
	one := execution.ScopePath("").Push(execution.LoopFrame("a", 0))
	if p, f, ok := one.Pop(); !ok || p != "" || f != execution.LoopFrame("a", 0) {
		t.Errorf("Pop() of a single frame = (%q, %+v, %v)", p, f, ok)
	}
}

func TestParseScopePath(t *testing.T) {
	t.Parallel()
	valid := []string{"", "L:a#0", "L:a#0/L:b#12", "MI:node-1#3/L:x_y#0"}
	for _, text := range valid {
		got, err := execution.ParseScopePath(text)
		if err != nil || string(got) != text {
			t.Errorf("ParseScopePath(%q) = (%q, %v), want it accepted", text, got, err)
		}
	}
	invalid := []string{"x", "L:a", "L:#1", "L:a#x", "L:a#-1", "Z:a#0", "L:a#0/", "/L:a#0", "L:a#0//L:b#0", "L:a#00", "L:a#+1", "L:0#0/L:0#00"}
	for _, text := range invalid {
		if got, err := execution.ParseScopePath(text); err == nil {
			t.Errorf("ParseScopePath(%q) = %q, want an error", text, got)
		}
	}
	if frames := execution.ScopePath("garbage").Frames(); frames != nil {
		t.Errorf("Frames() of a malformed path = %v, want nil", frames)
	}
	if _, _, ok := execution.ScopePath("garbage").Pop(); ok {
		t.Error("Pop() of a malformed path succeeded, want false")
	}
}

// FuzzParseScopePath checks the parser of stored scope text: it must never
// panic, and whatever it accepts must round-trip through its frames.
func FuzzParseScopePath(f *testing.F) {
	for _, seed := range []string{"", "L:a#0", "L:a#0/MI:b#3", "L:a", "///", "L:a#9999999999999999999"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, text string) {
		path, err := execution.ParseScopePath(text)
		if err != nil {
			return
		}
		var rebuilt execution.ScopePath
		for _, fr := range path.Frames() {
			rebuilt = rebuilt.Push(fr)
		}
		if rebuilt != path {
			t.Fatalf("accepted %q but rebuilt %q from its frames", path, rebuilt)
		}
		if path.Depth() != len(path.Frames()) {
			t.Fatalf("Depth() = %d for %d frames", path.Depth(), len(path.Frames()))
		}
		popped, ok := path.PopN(path.Depth())
		if !ok || popped != "" {
			t.Fatalf("PopN(Depth()) = (%q, %v), want the top level", popped, ok)
		}
	})
}
