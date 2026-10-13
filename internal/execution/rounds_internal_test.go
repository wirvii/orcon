// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import "testing"

// TestRoundsStayOpenForWaitingAndSuspendedDescendants pins amendment 9, B-2: a
// LET_FINISH round is held open by a descendant that is WAITING or SUSPENDED
// and is released only by a terminal state, while the SEQUENTIAL counters
// (ADR-0010 section 5) count a WAITING descendant as done. Waits arrive with
// cut C8; the token is put in the state directly.
func TestRoundsStayOpenForWaitingAndSuspendedDescendants(t *testing.T) {
	t.Parallel()
	split := NodeExecutionID("split-instance")
	lineage := []ForkRef{{SplitNodeExecution: split, SplitNode: "p", Branch: 0}}
	join := nodeKey{node: 0}

	newToken := func() (*run, *token) {
		r := newRun(NewInterpreter(Deps{}), &Plan{}, Request{Execution: "x"})
		r.roundJoins[split] = []nodeKey{join}
		tok := r.spawn("t1", TokenRunnable, 0, -1, "", lineage, "")
		if err := r.setToken(tok, TokenLeased); err != nil {
			t.Fatalf("claim: %v", err)
		}
		return r, tok
	}

	t.Run("waiting", func(t *testing.T) {
		t.Parallel()
		r, tok := newToken()
		if err := r.setToken(tok, TokenWaiting); err != nil {
			t.Fatal(err)
		}
		key := forkKey{split, 0}
		if got := r.forkLive[key]; got != 0 {
			t.Errorf("SEQUENTIAL counter = %d for a WAITING token, want 0: waiting counts as done there", got)
		}
		if got := r.splitLive[split]; got != 1 {
			t.Errorf("round counter = %d for a WAITING token, want 1: the round stays open", got)
		}
		// The wait resolves and the token runs again.
		if err := r.setToken(tok, TokenRunnable); err != nil {
			t.Fatal(err)
		}
		if r.forkLive[key] != 1 || r.splitLive[split] != 1 {
			t.Errorf("after the wait: SEQUENTIAL %d, round %d, want 1 and 1", r.forkLive[key], r.splitLive[split])
		}
		if err := r.setToken(tok, TokenCanceled); err != nil {
			t.Fatal(err)
		}
		if r.splitLive[split] != 0 {
			t.Errorf("round counter = %d after the token was canceled, want 0", r.splitLive[split])
		}
		if len(r.dirtyJoins) == 0 {
			t.Error("the join of the round was not queued when the last descendant ended")
		}
	})

	t.Run("suspended", func(t *testing.T) {
		t.Parallel()
		r, tok := newToken()
		if err := r.setToken(tok, TokenSuspended); err != nil {
			t.Fatal(err)
		}
		if r.splitLive[split] != 1 || r.forkLive[forkKey{split, 0}] != 1 {
			t.Errorf("SUSPENDED: round %d, SEQUENTIAL %d, want 1 and 1: both keep the branch open", r.splitLive[split], r.forkLive[forkKey{split, 0}])
		}
		if err := r.setToken(tok, TokenCanceled); err != nil {
			t.Fatal(err)
		}
		if r.splitLive[split] != 0 {
			t.Errorf("round counter = %d after cancel, want 0", r.splitLive[split])
		}
	})

	t.Run("terminal states release", func(t *testing.T) {
		t.Parallel()
		for _, end := range []TokenState{TokenConsumed, TokenCompleted, TokenFailed} {
			r, tok := newToken()
			if err := r.setToken(tok, end); err != nil {
				t.Fatal(err)
			}
			if r.splitLive[split] != 0 {
				t.Errorf("round counter = %d after %s, want 0", r.splitLive[split], end)
			}
		}
	})
}
