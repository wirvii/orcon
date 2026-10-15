// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable

import (
	"testing"

	"github.com/wirvii/orcon/internal/execution"
)

// TestProgressedNeedsANewResult (SPEC-001 Enmienda 12 section 12-1): only a step
// that stored a node execution in a terminal state puts the abandon count back
// to zero; a checkpoint with nothing new, or with a node still running, does not.
func TestProgressedNeedsANewResult(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		nodes []execution.NodeExecutionRecord
		want  bool
	}{
		{"no node execution", nil, false},
		{"a node still running", []execution.NodeExecutionRecord{{State: execution.NodeRunning}}, false},
		{"a node that succeeded", []execution.NodeExecutionRecord{{State: execution.NodeSucceeded}}, true},
		{"a node that failed", []execution.NodeExecutionRecord{{State: execution.NodeFailed}}, true},
		{"one running and one done", []execution.NodeExecutionRecord{{State: execution.NodeRunning}, {State: execution.NodeSucceeded}}, true},
	}
	for _, c := range cases {
		if got := progressed(&execution.Delta{NodeExecutions: c.nodes}); got != c.want {
			t.Errorf("%s: progressed = %v, want %v", c.name, got, c.want)
		}
	}
}
