// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

// TestRunDispatchesN8n proves that "orcon n8n ..." reaches the n8n commands
// instead of the server flags.
func TestRunDispatchesN8n(t *testing.T) {
	var stdout, stderr syncBuffer
	code := run(t.Context(), []string{"n8n", "coverage"}, &stdout, &stderr)
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr.String(), "--corpus is required") {
		t.Errorf("stderr = %q, want the coverage command's own usage error", stderr.String())
	}
}
