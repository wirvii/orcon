// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package fairness decides, for one claim round of a worker, which fairness
// keys (flows) the next tokens are claimed from (ADR-0014 section 1, SPEC-001
// R-FAIR-01). It is a pure area package: the store supplies a snapshot of the
// runnable tokens per (key, priority class), and the package returns the grants
// of the round.
//
// The policy is deficit round-robin over entries, where an entry is one fairness
// key at one priority class. Each round, an entry earns credit equal to its
// weight: 4 for HIGH, 2 for NORMAL, 1 for LOW. Aging raises an entry one class
// for every aging interval its oldest runnable token has waited, so low-priority
// work is never starved. Fairness is approximate across workers by design; each
// worker applies the same rule to its own snapshot.
package fairness

import (
	"slices"
	"strings"
	"time"
)

// Class is a priority class. Lower values are served first; the numeric order
// is the aging order.
type Class int

// Priority classes (ADR-0014 section 1).
const (
	High Class = iota
	Normal
	Low
)

// DefaultAgingInterval is one class of aging per this wait (SPEC-001 R-FAIR-01).
const DefaultAgingInterval = 30 * time.Second

// Weight is the credit an entry earns per round at a class: 4 : 2 : 1.
func Weight(c Class) int64 {
	switch c {
	case High:
		return 4
	case Normal:
		return 2
	case Low:
		return 1
	default:
		return 1
	}
}

// Entry is one row of the snapshot: the runnable tokens of a fairness key at a
// stored class, and when the oldest of them became claimable.
type Entry struct {
	Key      string
	Class    Class
	Runnable int64
	Oldest   time.Time
}

// Grant asks the worker to claim Count tokens of Key in this round. The class is
// the effective class of the entry that earned it.
type Grant struct {
	Key   string
	Class Class
	Count int64
}

// effective is the class of an entry after aging: one class up per interval of
// waiting, never above HIGH.
func effective(e Entry, now time.Time, aging time.Duration) Class {
	if aging <= 0 || e.Oldest.IsZero() {
		return e.Class
	}
	waited := now.Sub(e.Oldest)
	if waited < 0 {
		return e.Class
	}
	raised := int(waited / aging)
	c := int(e.Class) - raised
	if c < int(High) {
		c = int(High)
	}
	return Class(c)
}

// slotID names one schedulable entry: a fairness key at a stored class.
type slotID struct {
	key   string
	class Class
}

// Scheduler keeps the credit of every entry between claim rounds, so that a
// budget smaller than the number of entries still gives each entry its share
// over time. A worker owns one Scheduler; it is not safe for concurrent use.
type Scheduler struct {
	credit map[slotID]float64
}

// NewScheduler returns a scheduler with no credit yet.
func NewScheduler() *Scheduler {
	return &Scheduler{credit: map[slotID]float64{}}
}

// slot is one schedulable entry of a round: its identity, its effective class
// and the tokens it still has to give.
type slot struct {
	id   slotID
	eff  Class
	left int64
}

// Plan returns the grants of one claim round, at most budget tokens in total.
// Each entry with work earns a share of the budget proportional to its weight;
// shares an entry cannot use (it has fewer tokens) go to the others. Entries are
// visited in a fixed order (key, then class), so the same scheduler state and
// snapshot give the same grants. Grants of one key are merged.
func (s *Scheduler) Plan(entries []Entry, now time.Time, aging time.Duration, budget int64) []Grant {
	if budget <= 0 {
		return nil
	}
	if aging <= 0 {
		aging = DefaultAgingInterval
	}
	slots := s.slotsOf(entries, now, aging)
	granted := s.distribute(slots, budget)
	return mergeGrants(slots, granted)
}

// slotsOf builds the round's slots from the snapshot, in a fixed order, and
// drops the credit of entries that vanished from it.
func (s *Scheduler) slotsOf(entries []Entry, now time.Time, aging time.Duration) []slot {
	slots := make([]slot, 0, len(entries))
	seen := map[slotID]bool{}
	for _, e := range entries {
		if e.Runnable <= 0 {
			continue
		}
		id := slotID{key: e.Key, class: e.Class}
		if seen[id] {
			continue
		}
		seen[id] = true
		slots = append(slots, slot{id: id, eff: effective(e, now, aging), left: e.Runnable})
	}
	for id := range s.credit {
		if !seen[id] {
			delete(s.credit, id)
		}
	}
	slices.SortFunc(slots, func(a, b slot) int {
		if c := strings.Compare(a.id.key, b.id.key); c != 0 {
			return c
		}
		return int(a.id.class) - int(b.id.class)
	})
	return slots
}

// distribute splits the budget over the slots by weight, in up to four passes,
// so that shares unused by one slot reach the others. It returns the tokens per
// slot and drops the credit of slots that have no work left.
func (s *Scheduler) distribute(slots []slot, budget int64) []int64 {
	granted := make([]int64, len(slots))
	remaining := budget
	for pass := 0; pass < 4 && remaining > 0; pass++ {
		weights := activeWeight(slots)
		if weights == 0 {
			break
		}
		for i := range slots {
			if slots[i].left > 0 {
				s.credit[slots[i].id] += float64(remaining) * float64(Weight(slots[i].eff)) / float64(weights)
			}
		}
		for i := range slots {
			sl := &slots[i]
			if sl.left == 0 || remaining == 0 {
				continue
			}
			n := min(int64(s.credit[sl.id]), sl.left, remaining)
			if n <= 0 {
				continue
			}
			s.credit[sl.id] -= float64(n)
			sl.left -= n
			granted[i] += n
			remaining -= n
		}
	}
	for i := range slots {
		if slots[i].left == 0 {
			delete(s.credit, slots[i].id)
		}
	}
	return granted
}

// activeWeight is the sum of the weights of the slots that still have work.
func activeWeight(slots []slot) int64 {
	var w int64
	for i := range slots {
		if slots[i].left > 0 {
			w += Weight(slots[i].eff)
		}
	}
	return w
}

// mergeGrants folds the per-slot tokens into one grant per key, in key order.
func mergeGrants(slots []slot, granted []int64) []Grant {
	merged := map[string]*Grant{}
	var order []string
	for i, sl := range slots {
		if granted[i] == 0 {
			continue
		}
		g, ok := merged[sl.id.key]
		if !ok {
			g = &Grant{Key: sl.id.key, Class: sl.eff}
			merged[sl.id.key] = g
			order = append(order, sl.id.key)
		}
		g.Count += granted[i]
		if sl.eff < g.Class {
			g.Class = sl.eff
		}
	}
	out := make([]Grant, 0, len(order))
	for _, k := range order {
		out = append(out, *merged[k])
	}
	return out
}
