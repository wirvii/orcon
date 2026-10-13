// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package wazero

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/expr"
)

// snapshotDigest builds the snapshot of the evaluator's configuration in a
// fresh sandbox and returns its digest.
func snapshotDigest(t *testing.T, mode Mode) string {
	t.Helper()
	sb := New(WithMode(mode))
	t.Cleanup(func() { _ = sb.Close(t.Context()) })
	cfg := expr.New(expr.Options{}).SandboxConfig()
	rt, err := sb.Open(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rt.Close(t.Context()) })
	limit := max(cfg.MemoryLimit, minMemoryLimit)
	e, err := sb.engineFor(t.Context(), (limit+pageSize-1)/pageSize)
	if err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.snapshots) != 1 {
		t.Fatalf("%d snapshots, want 1", len(e.snapshots))
	}
	for _, s := range e.snapshots {
		sum := sha256.Sum256(s.memory)
		return hex.EncodeToString(sum[:])
	}
	return ""
}

const snapshotHelperEnv = "ORCON_SNAPSHOT_HELPER"

// TestSnapshotHashHelper prints the snapshot digest for the parent test; it
// does nothing in a normal run.
func TestSnapshotHashHelper(t *testing.T) {
	if os.Getenv(snapshotHelperEnv) == "" {
		t.Skip("helper of TestSnapshotDeterministic")
	}
	t.Logf("SNAPSHOT %s", snapshotDigest(t, ModeHardLimit))
}

// TestSnapshotDeterministic checks SPEC-001 amendment 10, A.3: building the
// snapshot twice, in one process and in different processes, gives the same
// bytes. The warm-up of the Luxon caches uses fixed instants and a fixed
// clock, and touches no Luxon setting, so nothing about the run leaks into
// the memory every runtime starts from.
func TestSnapshotDeterministic(t *testing.T) {
	a, b := snapshotDigest(t, ModeHardLimit), snapshotDigest(t, ModeHardLimit)
	if a != b {
		t.Errorf("two builds in one process differ:\n  %s\n  %s", a, b)
	}
	if c := snapshotDigest(t, ModeCooperativeLimit); c == "" {
		t.Error("no digest for the cooperative mode")
	}
	digests := map[string]bool{}
	for range 2 {
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestSnapshotHashHelper$", "-test.v") //nolint:gosec // re-runs this test binary
		cmd.Env = append(os.Environ(), snapshotHelperEnv+"=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("helper: %v\n%s", err, out)
		}
		idx := bytes.Index(out, []byte("SNAPSHOT "))
		if idx < 0 {
			t.Fatalf("no digest in the helper output:\n%s", out)
		}
		line, _, _ := strings.Cut(string(out)[idx+len("SNAPSHOT "):], "\n")
		digests[strings.TrimSpace(line)] = true
	}
	if len(digests) != 1 {
		t.Errorf("snapshots built in different processes differ: %v", digests)
	}
	for d := range digests {
		if d != a {
			t.Errorf("the snapshot of another process (%s) differs from this one (%s)", d, a)
		}
	}
}
