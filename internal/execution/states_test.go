// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/wirvii/orcon/internal/execution"
)

// machine describes a state machine for the exhaustive transition test: its
// states, the allowed moves written out by hand from ADR-0009, and the
// operations under test. The allowed list is deliberately not derived from the
// production table, so the test pins the table to the ADR.
type machine struct {
	name    string
	states  []string
	allowed [][2]string
	// can and move wrap the typed methods.
	can     func(from, to string) bool
	move    func(from, to string) (string, error)
	initial []string
	isInit  func(state string) bool
}

func machines() []machine {
	return []machine{
		{
			name: "execution",
			states: []string{
				"QUEUED", "RUNNING", "WAITING", "SUSPENDED", "CANCELING", "COMPENSATING", "SUCCEEDED", "FAILED", "CANCELED",
			},
			allowed: [][2]string{
				{"QUEUED", "RUNNING"},
				{"RUNNING", "WAITING"}, {"WAITING", "RUNNING"},
				{"RUNNING", "SUSPENDED"}, {"WAITING", "SUSPENDED"}, {"SUSPENDED", "RUNNING"},
				{"RUNNING", "SUCCEEDED"},
				{"RUNNING", "FAILED"}, {"WAITING", "FAILED"},
				{"FAILED", "RUNNING"},
				{"QUEUED", "CANCELING"}, {"RUNNING", "CANCELING"}, {"WAITING", "CANCELING"}, {"SUSPENDED", "CANCELING"},
				{"CANCELING", "CANCELED"}, {"CANCELING", "COMPENSATING"},
				{"COMPENSATING", "CANCELED"}, {"COMPENSATING", "SUSPENDED"},
			},
			can: func(f, to string) bool {
				return execution.State(f).CanTransitionTo(execution.State(to))
			},
			move: func(f, to string) (string, error) {
				s, err := execution.State(f).Transition(execution.State(to))
				return string(s), err
			},
			initial: []string{"QUEUED"},
			isInit:  func(s string) bool { return execution.State(s).IsInitial() },
		},
		{
			name: "token",
			states: []string{
				"RUNNABLE", "LEASED", "WAITING", "DEFERRED", "SUSPENDED", "CONSUMED", "COMPLETED", "CANCELED", "FAILED",
			},
			allowed: [][2]string{
				{"RUNNABLE", "LEASED"},
				{"LEASED", "RUNNABLE"},
				{"LEASED", "WAITING"}, {"WAITING", "RUNNABLE"},
				{"LEASED", "CONSUMED"}, {"LEASED", "COMPLETED"}, {"LEASED", "FAILED"}, {"LEASED", "SUSPENDED"},
				{"DEFERRED", "RUNNABLE"},
				{"RUNNABLE", "CANCELED"}, {"LEASED", "CANCELED"}, {"WAITING", "CANCELED"}, {"DEFERRED", "CANCELED"}, {"SUSPENDED", "CANCELED"},
				{"SUSPENDED", "RUNNABLE"},
			},
			can: func(f, to string) bool {
				return execution.TokenState(f).CanTransitionTo(execution.TokenState(to))
			},
			move: func(f, to string) (string, error) {
				s, err := execution.TokenState(f).Transition(execution.TokenState(to))
				return string(s), err
			},
			initial: []string{"RUNNABLE", "DEFERRED", "LEASED"},
			isInit:  func(s string) bool { return execution.TokenState(s).IsInitial() },
		},
		{
			name:   "node execution",
			states: []string{"RUNNING", "WAITING", "SUCCEEDED", "SUCCEEDED_WITH_ERRORS", "FAILED", "SUSPENDED", "CANCELED"},
			allowed: [][2]string{
				{"RUNNING", "WAITING"}, {"RUNNING", "SUCCEEDED"}, {"RUNNING", "SUCCEEDED_WITH_ERRORS"},
				{"RUNNING", "FAILED"}, {"RUNNING", "SUSPENDED"}, {"RUNNING", "CANCELED"},
				{"WAITING", "RUNNING"}, {"WAITING", "CANCELED"},
				{"SUSPENDED", "RUNNING"}, {"SUSPENDED", "CANCELED"},
			},
			can: func(f, to string) bool {
				return execution.NodeExecutionState(f).CanTransitionTo(execution.NodeExecutionState(to))
			},
			move: func(f, to string) (string, error) {
				s, err := execution.NodeExecutionState(f).Transition(execution.NodeExecutionState(to))
				return string(s), err
			},
			initial: []string{"RUNNING"},
			isInit:  func(s string) bool { return execution.NodeExecutionState(s).IsInitial() },
		},
		{
			name:   "task attempt",
			states: []string{"STARTED", "SUCCEEDED", "FAILED", "TIMED_OUT", "ABANDONED", "CANCELED"},
			allowed: [][2]string{
				{"STARTED", "SUCCEEDED"}, {"STARTED", "FAILED"}, {"STARTED", "TIMED_OUT"},
				{"STARTED", "ABANDONED"}, {"STARTED", "CANCELED"},
			},
			can: func(f, to string) bool {
				return execution.AttemptState(f).CanTransitionTo(execution.AttemptState(to))
			},
			move: func(f, to string) (string, error) {
				s, err := execution.AttemptState(f).Transition(execution.AttemptState(to))
				return string(s), err
			},
			initial: []string{"STARTED"},
			isInit:  func(s string) bool { return execution.AttemptState(s).IsInitial() },
		},
		{
			name:    "timer",
			states:  []string{"SCHEDULED", "FIRED", "CANCELED"},
			allowed: [][2]string{{"SCHEDULED", "FIRED"}, {"SCHEDULED", "CANCELED"}},
			can: func(f, to string) bool {
				return execution.TimerState(f).CanTransitionTo(execution.TimerState(to))
			},
			move: func(f, to string) (string, error) {
				s, err := execution.TimerState(f).Transition(execution.TimerState(to))
				return string(s), err
			},
			initial: []string{"SCHEDULED"},
			isInit:  func(s string) bool { return execution.TimerState(s).IsInitial() },
		},
		{
			name:    "wait",
			states:  []string{"OPEN", "RESOLVED", "EXPIRED", "CANCELED"},
			allowed: [][2]string{{"OPEN", "RESOLVED"}, {"OPEN", "EXPIRED"}, {"OPEN", "CANCELED"}},
			can: func(f, to string) bool {
				return execution.WaitState(f).CanTransitionTo(execution.WaitState(to))
			},
			move: func(f, to string) (string, error) {
				s, err := execution.WaitState(f).Transition(execution.WaitState(to))
				return string(s), err
			},
			initial: []string{"OPEN"},
			isInit:  func(s string) bool { return execution.WaitState(s).IsInitial() },
		},
	}
}

func TestEveryTransitionOutsideTheTableIsAnError(t *testing.T) {
	t.Parallel()
	for _, m := range machines() {
		t.Run(m.name, func(t *testing.T) {
			t.Parallel()
			// A state that is in no table is tried on both sides too.
			universe := append(slices.Clone(m.states), "NO_SUCH_STATE")
			for _, from := range universe {
				for _, to := range universe {
					want := slices.Contains(m.allowed, [2]string{from, to})
					if got := m.can(from, to); got != want {
						t.Errorf("%s %s -> %s: CanTransitionTo = %v, want %v", m.name, from, to, got, want)
					}
					next, err := m.move(from, to)
					switch {
					case want && (err != nil || next != to):
						t.Errorf("%s %s -> %s: Transition = (%q, %v), want (%q, nil)", m.name, from, to, next, err, to)
					case !want && err == nil:
						t.Errorf("%s %s -> %s: Transition succeeded, want an error", m.name, from, to)
					case !want:
						var te *execution.TransitionError
						if !errors.Is(err, execution.ErrInvalidTransition) || !errors.As(err, &te) || te.From != from || te.To != to {
							t.Errorf("%s %s -> %s: error = %v, want a TransitionError matching ErrInvalidTransition", m.name, from, to, err)
						}
						if next != from {
							t.Errorf("%s %s -> %s: a rejected Transition returned %q, want the state unchanged", m.name, from, to, next)
						}
					}
				}
			}
		})
	}
}

func TestStateListsAreComplete(t *testing.T) {
	t.Parallel()
	lists := map[string][]string{
		"execution":      toStrings(execution.States()),
		"token":          toStrings(execution.TokenStates()),
		"node execution": toStrings(execution.NodeExecutionStates()),
		"task attempt":   toStrings(execution.AttemptStates()),
		"timer":          toStrings(execution.TimerStates()),
		"wait":           toStrings(execution.WaitStates()),
	}
	for _, m := range machines() {
		want := slices.Clone(m.states)
		slices.Sort(want)
		if got := lists[m.name]; !slices.Equal(got, want) {
			t.Errorf("%s states = %v, want %v", m.name, got, want)
		}
	}
}

func toStrings[S ~string](in []S) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = string(s)
	}
	return out
}

func TestInitialStates(t *testing.T) {
	t.Parallel()
	for _, m := range machines() {
		for _, s := range m.states {
			if got, want := m.isInit(s), slices.Contains(m.initial, s); got != want {
				t.Errorf("%s: IsInitial(%s) = %v, want %v", m.name, s, got, want)
			}
		}
	}
}

func TestTokenTerminalStates(t *testing.T) {
	t.Parallel()
	terminal := map[execution.TokenState]bool{
		execution.TokenConsumed: true, execution.TokenCompleted: true, execution.TokenCanceled: true, execution.TokenFailed: true,
	}
	for _, s := range execution.TokenStates() {
		if got := s.IsTerminal(); got != terminal[s] {
			t.Errorf("IsTerminal(%s) = %v, want %v", s, got, terminal[s])
		}
	}
}
