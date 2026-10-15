// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package fairness

import (
	"slices"
	"testing"
	"time"
)

var start = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)

// batch is a run of tokens of one class that became claimable together.
type batch struct {
	enq time.Time
	n   int64
}

// sim is a fluid model of the claim rounds of one worker. The store holds
// runnable tokens per fairness key and class, in batches; a round claims what
// Plan grants, in the order the store uses (class, then enqueue time).
type sim struct {
	sched  *Scheduler
	queues map[string]map[Class][]batch
	waits  map[string][]time.Duration
	served map[string]int64
	now    time.Time
}

func newSim() *sim {
	return &sim{sched: NewScheduler(), queues: map[string]map[Class][]batch{}, waits: map[string][]time.Duration{}, served: map[string]int64{}, now: start}
}

func (s *sim) add(key string, class Class, n int, at time.Time) {
	if s.queues[key] == nil {
		s.queues[key] = map[Class][]batch{}
	}
	s.queues[key][class] = append(s.queues[key][class], batch{enq: at, n: int64(n)})
}

// snapshot is what the store would report: per key and class, the count and the
// oldest claimable instant.
func (s *sim) snapshot() []Entry {
	var out []Entry
	for key, byClass := range s.queues {
		for c := High; c <= Low; c++ {
			var e Entry
			e.Key, e.Class = key, c
			for _, b := range byClass[c] {
				e.Runnable += b.n
				if e.Oldest.IsZero() || b.enq.Before(e.Oldest) {
					e.Oldest = b.enq
				}
			}
			if e.Runnable > 0 {
				out = append(out, e)
			}
		}
	}
	return out
}

// take removes n tokens of key from the store order and records their waits.
func (s *sim) take(key string, n int64) {
	for c := High; c <= Low && n > 0; c++ {
		bs := s.queues[key][c]
		for len(bs) > 0 && n > 0 {
			b := &bs[0]
			k := min(n, b.n)
			for range k {
				s.waits[key] = append(s.waits[key], s.now.Sub(b.enq))
			}
			s.served[key] += k
			b.n -= k
			n -= k
			if b.n == 0 {
				bs = bs[1:]
			}
		}
		s.queues[key][c] = bs
	}
	empty := true
	for _, bs := range s.queues[key] {
		if len(bs) > 0 {
			empty = false
		}
	}
	if empty {
		delete(s.queues, key)
	}
}

// round claims one budget of tokens and advances the clock by tick.
func (s *sim) round(budget int64, aging time.Duration, tick time.Duration) {
	for _, g := range s.sched.Plan(s.snapshot(), s.now, aging, budget) {
		s.take(g.Key, g.Count)
	}
	s.now = s.now.Add(tick)
}

func p99(ds []time.Duration) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	c := slices.Clone(ds)
	slices.Sort(c)
	idx := min(len(c)-1, int(float64(len(c))*0.99))
	return c[idx]
}

// TestFairness: a flow that floods 100 000 tokens does not delay a flow of 100.
func TestFairness(t *testing.T) {
	s := newSim()
	s.add("flood", Normal, 100000, start)
	s.add("small", Normal, 100, start)
	for len(s.waits["small"]) < 100 {
		s.round(16, DefaultAgingInterval, 10*time.Millisecond)
		if s.now.Sub(start) > time.Minute {
			t.Fatal("the small flow was not served within a minute")
		}
	}
	if got := p99(s.waits["small"]); got > 2*time.Second {
		t.Fatalf("p99 claim wait of the small flow = %s, want <= 2s", got)
	}
}

// TestPriorityWeights: under saturation the classes get 4 : 2 : 1 within 15 %.
func TestPriorityWeights(t *testing.T) {
	s := newSim()
	s.add("high", High, 100000, start)
	s.add("normal", Normal, 100000, start)
	s.add("low", Low, 100000, start)
	const total = 7000
	for sum(s.served) < total {
		s.round(7, 1000*time.Hour, time.Millisecond)
	}
	want := map[string]float64{"high": 4.0 / 7, "normal": 2.0 / 7, "low": 1.0 / 7}
	for key, share := range want {
		got := float64(s.served[key]) / float64(sum(s.served))
		if got < share*0.85 || got > share*1.15 {
			t.Errorf("%s share = %.3f, want %.3f +-15%%", key, got, share)
		}
	}
}

func sum(m map[string]int64) int64 {
	var n int64
	for _, v := range m {
		n += v
	}
	return n
}

// TestAgingPreventsStarvation: a low-priority token behind a flood of high
// priority work is served within three aging intervals.
func TestAgingPreventsStarvation(t *testing.T) {
	const aging = 30 * time.Second
	s := newSim()
	s.add("flood", High, 1000000, start)
	s.add("low", Low, 1, start)
	for s.served["low"] == 0 {
		s.round(1, aging, 10*time.Millisecond)
		if s.now.Sub(start) > 3*aging {
			t.Fatalf("the low token waited %s, more than 3 x aging", s.now.Sub(start))
		}
	}
}

// TestEffectiveClassAges checks the aging rule itself.
func TestEffectiveClassAges(t *testing.T) {
	e := Entry{Key: "k", Class: Low, Runnable: 1, Oldest: start}
	if c := effective(e, start.Add(29*time.Second), DefaultAgingInterval); c != Low {
		t.Fatalf("before one interval: %v", c)
	}
	if c := effective(e, start.Add(31*time.Second), DefaultAgingInterval); c != Normal {
		t.Fatalf("after one interval: %v", c)
	}
	if c := effective(e, start.Add(10*time.Minute), DefaultAgingInterval); c != High {
		t.Fatalf("after many intervals: %v", c)
	}
}

// TestPlanIsDeterministicAndBounded checks the grants of a round.
func TestPlanIsDeterministicAndBounded(t *testing.T) {
	entries := []Entry{
		{Key: "b", Class: Normal, Runnable: 5, Oldest: start},
		{Key: "a", Class: High, Runnable: 3, Oldest: start},
		{Key: "c", Class: Low, Runnable: 0, Oldest: start},
	}
	first := NewScheduler().Plan(entries, start, DefaultAgingInterval, 6)
	second := NewScheduler().Plan(entries, start, DefaultAgingInterval, 6)
	if !slices.Equal(first, second) {
		t.Fatalf("plans differ: %v vs %v", first, second)
	}
	var total int64
	for _, g := range first {
		total += g.Count
		if g.Key == "c" {
			t.Fatal("an empty entry was granted")
		}
	}
	if total != 6 {
		t.Fatalf("granted %d, want 6", total)
	}
}
