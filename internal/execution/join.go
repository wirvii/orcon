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
	// Canceled is set when the edge completed because the branch feeding it was
	// canceled, not because it was routed around. A canceled edge never makes
	// an ALL join fail.
	Canceled bool
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
	mode flow.JoinMode
	// letFinish keeps rounds instead of firing once (remaining LET_FINISH).
	letFinish bool
	rounds    []*joinRound
	edges     []joinEdge
	arrivals  []joinArrival
	// fired, failed and empty record the decision taken; a join decides once.
	fired  bool
	failed bool
	empty  bool
}

// joinRound is one round of a LET_FINISH join: the arrivals that descend from
// one instance of the join's corresponding split.
type joinRound struct {
	// number counts the rounds of the join from 1, in the order of their first
	// arrival. It serves ordering and display; ids never derive from it.
	number int
	// split is the split instance the round belongs to, or the arriving token
	// when the token carries none.
	split    NodeExecutionID
	arrivals []joinArrival
	fired    bool
	reset    bool
}

func newJoinState(spec *joinSpec, incoming int) *joinState {
	return &joinState{mode: spec.mode, letFinish: spec.letFinish, edges: make([]joinEdge, incoming)}
}

// round returns the round of a split instance, creating it when the arrival is
// the first of the instance.
func (j *joinState) round(split NodeExecutionID) (*joinRound, bool) {
	for _, rd := range j.rounds {
		if rd.split == split {
			return rd, false
		}
	}
	rd := &joinRound{number: len(j.rounds) + 1, split: split}
	j.rounds = append(j.rounds, rd)
	return rd, true
}

// complete reports whether every incoming edge is complete.
func (j *joinState) complete() bool {
	for _, e := range j.edges {
		if !e.complete() {
			return false
		}
	}
	return true
}

// done reports whether the join has settled for good, so that it may complete
// its outgoing edges once its continuations have run. A LET_FINISH join is done
// when every incoming edge is complete and every round has reset.
func (j *joinState) done() bool {
	if j.empty {
		return true
	}
	if !j.letFinish {
		return j.fired
	}
	if !j.complete() || len(j.rounds) == 0 {
		return false
	}
	for _, rd := range j.rounds {
		if !rd.reset {
			return false
		}
	}
	return true
}

func (j *joinState) settled() bool { return j.fired || j.failed || j.empty }

// send records a live token delivered to the join on a slot.
func (j *joinState) send(slot int) { j.edges[slot].Sent++ }

// arrive absorbs a token and returns what the join decides.
func (j *joinState) arrive(a joinArrival) joinOutcome {
	j.edges[a.Slot].Arrived++
	j.arrivals = append(j.arrivals, a)
	return j.decide()
}

// signal records the completion signal of an incoming edge and returns what
// the join decides. A canceled signal closes the edge of a branch that was
// canceled.
func (j *joinState) signal(slot int, canceled bool) joinOutcome {
	j.edges[slot].Signaled = true
	j.edges[slot].Canceled = j.edges[slot].Canceled || canceled
	return j.decide()
}

func (j *joinState) anyCanceled() bool {
	for _, e := range j.edges {
		if e.Canceled {
			return true
		}
	}
	return false
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
	if j.letFinish {
		// Rounds fire and reset the join; only a join that no token ever
		// reached is decided here.
		if len(j.arrivals) == 0 && j.complete() {
			return joinEmpty
		}
		return joinWaiting
	}
	allComplete, anyLive, anyDead := true, len(j.arrivals) > 0, false
	for _, e := range j.edges {
		if !e.complete() {
			allComplete = false
		}
		anyLive = anyLive || e.Sent > 0
		anyDead = anyDead || (e.complete() && e.Arrived == 0 && !e.Canceled)
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
			// A canceled branch among the inputs means the join belongs to an
			// abandoned region (or lost an input it needed): it does not fire.
			if anyLive && !j.anyCanceled() {
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
// longest prefix the arrivals share, without the entry of the join's
// corresponding split and whatever follows it. That split's branches converge
// at the join, so the continuation no longer descends from one of them. A
// lineage without the entry (the split did not fork on this run) is kept.
func continuationLineage(arrivals []joinArrival, split flow.NodeID) []ForkRef {
	if len(arrivals) == 0 {
		return nil
	}
	prefix := slices.Clone(arrivals[0].Lineage)
	keep := len(prefix)
	for _, a := range arrivals {
		n := 0
		for n < len(prefix) && n < len(a.Lineage) && prefix[n] == a.Lineage[n] {
			n++
		}
		prefix = prefix[:n]
		// The entry to drop is found on the full lineage of each arrival: the
		// branches of the split are what differs between the arrivals.
		if i := lastEntryOf(a.Lineage, split); i >= 0 {
			keep = min(keep, i)
		}
	}
	return prefix[:min(len(prefix), keep)]
}

// lastEntryOf returns the index of the last lineage entry of a split node, or -1.
func lastEntryOf(lineage []ForkRef, split flow.NodeID) int {
	for i := len(lineage) - 1; i >= 0; i-- {
		if lineage[i].SplitNode == split {
			return i
		}
	}
	return -1
}
