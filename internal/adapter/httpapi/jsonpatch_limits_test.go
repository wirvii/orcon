// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"errors"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
)

func TestApplyJSONPatchRejectsAmplification(t *testing.T) {
	// A document of about 1 MiB copied onto itself again and again: every copy of the root would
	// double it, so the budget must stop the patch within a few operations.
	doc := []byte(`{"pad":"` + strings.Repeat("x", 1<<20) + `"}`)
	var patch apigen.JsonPatch
	for i := range 40 {
		patch = append(patch, from(apigen.Copy, "", "/k"+strconv.Itoa(i)))
	}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	start := time.Now()
	_, err := applyJSONPatch(doc, patch)
	runtime.ReadMemStats(&after)
	var issues *documentIssuesError
	if !errors.As(err, &issues) || issues.fields[0].Code != "patch-failed" {
		t.Fatalf("amplifying patch error = %v, want a patch-failed field error", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("the patch took %s to be refused", time.Since(start))
	}
	// Encoding and decoding buffers of the copies that fit the budget; the unbounded patch took GiB.
	if grown := after.TotalAlloc - before.TotalAlloc; grown > 16*uint64(documentBodyLimit) {
		t.Fatalf("the refused patch allocated %d bytes, want at most %d", grown, 16*documentBodyLimit)
	}
}

func TestApplyJSONPatchRejectsAnOversizedResult(t *testing.T) {
	pad := strings.Repeat("x", 5<<20)
	doc := []byte(`{"a":"` + pad + `"}`)
	// Each value is within the copy budget, but the document they build is over the limit.
	_, err := applyJSONPatch(doc, apigen.JsonPatch{op(apigen.Add, "/b", pad)})
	var issues *documentIssuesError
	if !errors.As(err, &issues) || issues.fields[0].Code != "patch-failed" {
		t.Fatalf("oversized result error = %v, want a patch-failed field error", err)
	}
}

func TestApplyJSONPatchKeepsNormalCopies(t *testing.T) {
	got, err := applyJSONPatch([]byte(`{"a":{"x":1}}`), apigen.JsonPatch{from(apigen.Copy, "/a", "/b"), from(apigen.Copy, "", "/c")})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":{"x":1},"b":{"x":1},"c":{"a":{"x":1},"b":{"x":1}}}`
	if !jsonEqual(mustDecode(t, got), mustDecode(t, []byte(want))) {
		t.Fatalf("patched = %s, want %s", got, want)
	}
}

func TestApplyJSONPatchBoundsArrayShifting(t *testing.T) {
	// 100k elements and 1000 inserts and removals at index 0: each one shifts the whole array, so
	// the total work is far past the budget even though the operation count is within its cap.
	doc := []byte(`{"a":[` + strings.Repeat("0,", 99999) + `0]}`)
	var patch apigen.JsonPatch
	for range 500 {
		patch = append(patch, op(apigen.Add, "/a/0", 1), op(apigen.Remove, "/a/0", nil))
	}
	_, err := applyJSONPatch(doc, patch)
	var issues *documentIssuesError
	if !errors.As(err, &issues) || issues.fields[0].Code != "patch-failed" {
		t.Fatalf("costly patch error = %v, want a patch-failed field error", err)
	}
	moved := apigen.JsonPatch{from(apigen.Move, "/a/0", "/a/-")}
	if _, err := applyJSONPatch(doc, moved); err != nil {
		t.Fatalf("one move in a large array: %v", err)
	}
}
