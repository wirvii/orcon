// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"slices"

	"github.com/wirvii/orcon/internal/flow"
)

// ForkRef records that a token descends from one branch of a split: the split
// node execution and the branch (the position of the edge among the outgoing
// edges of the split node). The list of ForkRefs of a token is its fork
// lineage (ADR-0008 section 2).
type ForkRef struct {
	SplitNodeExecution NodeExecutionID
	SplitNode          flow.NodeID
	Branch             int
}

// joinOutcome is what a join decides after an arrival or a completion signal.
type joinOutcome uint8

const (
	// The join keeps waiting.
	joinWaiting joinOutcome = iota
	// The firing condition holds: create the continuation.
	joinFires
	// The join cannot be satisfied: JoinRequirementNotMet.
	joinFails
	// Every incoming edge completed with no live token: the join lies on a dead
	// path and completes its outputs with zero tokens.
	joinEmpty
)

// joinEdge is the state of one incoming edge of a join instance.
type joinEdge struct {
	// Sent counts the live tokens delivered to the join on the edge.
	Sent int
	// Arrived counts the tokens the join has absorbed from the edge.
	Arrived int
	// Signaled is set once the edge's completion signal was delivered.
	Signaled bool
}

// complete reports whether the edge is complete: the signal came and every
// token sent on the edge was absorbed. The completion signal follows the live
// tokens of an edge (ADR-0010 section 2), but a token may still be queued when
// the signal is processed, so the join compares the counts.
func (e joinEdge) complete() bool { return e.Signaled && e.Arrived == e.Sent }

// joinArrival is one token absorbed by a join.
type joinArrival struct {
	Slot    int
	Token   TokenID
	Payload PayloadRef
	Lineage []ForkRef
}

// joinState is the state of one join instance: a join node in a scope path
// (ADR-0010 section 3). It holds plain data and the decision rules as methods,
// so the durability cut can persist it as the join_instance edge-state
// document and run the same rules inside its transaction.
type joinState struct {
	mode     flow.JoinMode
	edges    []joinEdge
	arrivals []joinArrival
	// fired, failed and empty record the decision taken; a join decides once.
	fired  bool
	failed bool
	empty  bool
}

func newJoinState(mode flow.JoinMode, incoming int) *joinState {
	return &joinState{mode: mode, edges: make([]joinEdge, incoming)}
}

func (j *joinState) settled() bool { return j.fired || j.failed || j.empty }

// send records a live token delivered to the join on a slot.
func (j *joinState) send(slot int) { j.edges[slot].Sent++ }

// retract undoes a delivery whose token was canceled before it arrived.
func (j *joinState) retract(slot int) { j.edges[slot].Sent-- }

// arrive absorbs a token and returns what the join decides.
func (j *joinState) arrive(a joinArrival) joinOutcome {
	j.edges[a.Slot].Arrived++
	j.arrivals = append(j.arrivals, a)
	return j.decide()
}

// signal records the completion signal of an incoming edge and returns what
// the join decides.
func (j *joinState) signal(slot int) joinOutcome {
	j.edges[slot].Signaled = true
	return j.decide()
}

// decide applies the firing rules of ADR-0010 section 3 and records the
// outcome, so a settled join answers joinWaiting from then on.
func (j *joinState) decide() joinOutcome {
	if j.settled() {
		return joinWaiting
	}
	out := j.rule()
	switch out {
	case joinFires:
		j.fired = true
	case joinFails:
		j.failed = true
	case joinEmpty:
		j.empty = true
	case joinWaiting:
	}
	return out
}

func (j *joinState) rule() joinOutcome {
	allComplete, anyLive, anyDead := true, len(j.arrivals) > 0, false
	for _, e := range j.edges {
		if !e.complete() {
			allComplete = false
		}
		anyLive = anyLive || e.Sent > 0
		anyDead = anyDead || (e.complete() && e.Arrived == 0)
	}
	switch j.mode {
	case flow.JoinAll:
		// A live token on one edge and an edge that completed with none can
		// never be satisfied, so ALL fails as soon as both are known, not when
		// the others complete. A join with no live token on any edge lies on a
		// dead path: it completes its outputs with zero tokens, like ACTIVATED.
		// ADR-0010 section 3 states the failure but is silent on the dead
		// path; failing there would break every AND block inside a branch an
		// exclusive choice did not take.
		if anyLive && anyDead {
			return joinFails
		}
		if allComplete {
			if anyLive {
				return joinFires
			}
			return joinEmpty
		}
	case flow.JoinActivated:
		if allComplete {
			if len(j.arrivals) > 0 {
				return joinFires
			}
			return joinEmpty
		}
	case flow.JoinFirst:
		if len(j.arrivals) > 0 {
			return joinFires
		}
		if allComplete {
			return joinEmpty
		}
	}
	return joinWaiting
}

// combined returns the arrivals in the order the APPEND data mode
// concatenates them: by incoming edge, then by arrival order within an edge
// (ADR-0010 section 3).
func (j *joinState) combined() []joinArrival {
	out := slices.Clone(j.arrivals)
	slices.SortStableFunc(out, func(a, b joinArrival) int { return a.Slot - b.Slot })
	return out
}

// continuationLineage returns the fork lineage of the continuation token: the
// longest prefix the arrivals share, cut at the last entry whose split
// dominates the join. Such a split's branches converge at the join, so the
// continuation no longer descends from one of them. The dominates function
// answers whether a split node dominates the join; the plan precomputes it.
func continuationLineage(arrivals []joinArrival, dominates func(flow.NodeID) bool) []ForkRef {
	if len(arrivals) == 0 {
		return nil
	}
	prefix := slices.Clone(arrivals[0].Lineage)
	for _, a := range arrivals[1:] {
		n := 0
		for n < len(prefix) && n < len(a.Lineage) && prefix[n] == a.Lineage[n] {
			n++
		}
		prefix = prefix[:n]
	}
	return cutAtDominator(prefix, dominates)
}

// cutAtDominator drops the last lineage entry whose split dominates the join,
// and everything after it.
func cutAtDominator(lineage []ForkRef, dominates func(flow.NodeID) bool) []ForkRef {
	if i := lastDominating(lineage, dominates); i >= 0 {
		return lineage[:i]
	}
	return lineage
}

// lastDominating returns the index of the last lineage entry whose split node
// dominates the join, or -1.
func lastDominating(lineage []ForkRef, dominates func(flow.NodeID) bool) int {
	for i := len(lineage) - 1; i >= 0; i-- {
		if dominates(lineage[i].SplitNode) {
			return i
		}
	}
	return -1
}
