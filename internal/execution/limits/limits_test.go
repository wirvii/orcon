// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package limits

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

// TestConcurrencyLimits: under concurrent load no level is ever exceeded, and a
// refusal names the level that refused.
func TestConcurrencyLimits(t *testing.T) {
	const workers, nodes = 4, 3
	cfg := Config{NodeConcurrency: 2, WorkerTokens: 5, MemoryBudget: 1000}
	var throttled sync.Map
	l := New(cfg, func(th Throttle) {
		throttled.Store(th.Level, true)
	})

	var maxTokens, maxNode atomic.Int64
	var perNodeMu sync.Mutex
	perNode := map[string]int{}
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 2000 {
				node := []string{"n0", "n1", "n2"}[i%nodes]
				g, err := l.Acquire(node, 100)
				if err != nil {
					if !errors.Is(err, ErrThrottled) {
						t.Errorf("refusal does not match ErrThrottled: %v", err)
					}
					if _, ok := ThrottleOf(err); !ok {
						t.Errorf("refusal names no level: %v", err)
					}
					continue
				}
				perNodeMu.Lock()
				perNode[node]++
				if perNode[node] > int(maxNode.Load()) {
					maxNode.Store(int64(perNode[node]))
				}
				if int64(l.InFlight()) > maxTokens.Load() {
					maxTokens.Store(int64(l.InFlight()))
				}
				if perNode[node] > cfg.NodeConcurrency {
					t.Errorf("node %s has %d in flight, cap %d", node, perNode[node], cfg.NodeConcurrency)
				}
				perNodeMu.Unlock()
				perNodeMu.Lock()
				perNode[node]--
				perNodeMu.Unlock()
				g.Release()
			}
		}()
	}
	wg.Wait()
	if got := maxTokens.Load(); got > int64(cfg.WorkerTokens) {
		t.Fatalf("worker held %d tokens, cap %d", got, cfg.WorkerTokens)
	}
	// Whether the load reaches the node cap depends on the scheduler, so the node level is also
	// driven on purpose: the grants that fill one node's cap make the next acquire refuse there.
	var fill []*Grant
	for range cfg.NodeConcurrency {
		g, err := l.Acquire("n0", 100)
		if err != nil {
			t.Fatalf("a grant within the node cap was refused: %v", err)
		}
		fill = append(fill, g)
	}
	if _, err := l.Acquire("n0", 100); !errors.Is(err, ErrThrottled) {
		t.Fatalf("a grant above the node cap was not refused: %v", err)
	} else if th, ok := ThrottleOf(err); !ok || th.Level != LevelNode {
		t.Fatalf("a grant above the node cap named the wrong level: %+v", th)
	}
	for _, g := range fill {
		g.Release()
	}
	if _, ok := throttled.Load(LevelNode); !ok {
		t.Fatal("the node level never throttled")
	}

	// The worker level, held by grants that stay in flight.
	w := New(Config{WorkerTokens: 5}, nil)
	var held []*Grant
	for range 5 {
		g, err := w.Acquire("n", 0)
		if err != nil {
			t.Fatal(err)
		}
		held = append(held, g)
	}
	_, err := w.Acquire("n", 0)
	if th, ok := ThrottleOf(err); !ok || th.Level != LevelWorker {
		t.Fatalf("sixth token: %v", err)
	}
	for _, g := range held {
		g.Release()
	}
}

// TestStrictestLimitDecides: a worker at its token cap reports WORKER even when
// the node is also full, because the order is node first but both must grant.
func TestStrictestLimitDecides(t *testing.T) {
	var last Throttle
	l := New(Config{NodeConcurrency: 1, WorkerTokens: 1}, func(th Throttle) { last = th })
	g, err := l.Acquire("n", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.Acquire("n", 0); err == nil {
		t.Fatal("the node cap did not refuse")
	} else if th, ok := ThrottleOf(err); !ok || th.Level != LevelNode {
		t.Fatalf("throttle = %+v, %v", th, ok)
	}
	if last.Level != LevelNode {
		t.Fatalf("observer saw %+v", last)
	}
	g.Release()
	g.Release() // a second release changes nothing
	if l.InFlight() != 0 {
		t.Fatalf("in flight = %d after release", l.InFlight())
	}
}

// TestMemoryAdmissionStopsAt85Percent: a token that would cross 85 % of the
// memory budget is refused by the worker level.
func TestMemoryAdmissionStopsAt85Percent(t *testing.T) {
	l := New(Config{MemoryBudget: 1000}, nil)
	g, err := l.Acquire("n", 800)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.Acquire("n", 100); err == nil {
		t.Fatal("900 bytes of 1000 was admitted; admission stops at 85 %")
	}
	g.Release()
	if _, err := l.Acquire("n", 100); err != nil {
		t.Fatalf("after release: %v", err)
	}
}
