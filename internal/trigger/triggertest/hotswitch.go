// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package triggertest

import (
	"sync"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/trigger"
)

// Load sets the pressure of [HotSwitchUnderLoad].
type Load struct {
	// Rate is the number of acceptance reads per second.
	Rate int
	// Duration is how long the load runs. The publication happens halfway.
	Duration time.Duration
}

// observation is one acceptance read of the load.
type observation struct {
	start, end time.Time
	version    int64
	accepting  bool
	err        error
}

// HotSwitchUnderLoad proves ADR-0024 section 3 at the level of the storage
// the acceptance transaction of cut T2 will read: while acceptance reads run at
// Rate per second against an active flow, publishing version 2 in the middle
// makes no read fail or be refused, every read names exactly version 1 or
// version 2, reads that finished before the publication began name version 1
// and reads that began after it returned name version 2, no reader sees the
// version go backwards, and a trigger whose configuration did not change keeps
// its registry entry and hash (E-ACT-2).
//
// What it cannot prove yet: HTTP responses (cut T2), accepted events and their
// executions (cut T2 and the durable engine) and the persistent connection of
// a listener (cut T7). The criterion keeps those for the cuts that build them.
func HotSwitchUnderLoad(t *testing.T, f Factory, load Load) {
	t.Helper()
	e := newEnv(t, f, trigger.Settings{})
	hook := Node{ID: "hook", Block: WebhookBlock("hot")}
	cron := Node{ID: "cron", Type: TypeSchedule, Block: `{"family":"SCHEDULE","schedule":{"rules":[{"kind":"MINUTES","every":5}]}}`}
	poll := Node{ID: "poll", Type: TypePoll, Block: `{"family":"POLL","poll":{"rules":[{"kind":"MINUTES","every":2}]}}`}
	id := e.newFlow(e.scopeA, e.projA, "hot", Build(DocSpec{Name: "v1", Starts: []Node{hook, cron, poll}}))
	e.publish(e.scopeA, id)
	must(t, "activate", e.svc.Activate(e.ctx(), e.scopeA, id))
	before := entriesByNode(e.triggers(e.scopeA, id))

	e.saveDraft(e.scopeA, id, Build(DocSpec{Name: "v2", Starts: []Node{hook, cron, poll}}))
	var publishStart, publishEnd time.Time
	obs := runLoad(t, e, id, load, func() {
		publishStart = time.Now()
		e.publish(e.scopeA, id)
		publishEnd = time.Now()
	})

	checkObservations(t, obs, publishStart, publishEnd)
	after := entriesByNode(e.triggers(e.scopeA, id))
	for node, was := range before {
		now, ok := after[node]
		if !ok {
			t.Fatalf("trigger %s vanished in the publication", node)
		}
		if now.ID != was.ID || now.ConfigurationHash != was.ConfigurationHash {
			t.Fatalf("trigger %s lost its identity or hash: %+v then %+v", node, was, now)
		}
		if now.FlowVersionID == was.FlowVersionID || now.Status != trigger.StatusActive {
			t.Fatalf("trigger %s does not point at version 2 and stay active: %+v", node, now)
		}
	}
	reads := 0
	for _, list := range obs {
		reads += len(list)
	}
	t.Logf("hot switch: %d acceptance reads over %s, publication took %s", reads, load.Duration, publishEnd.Sub(publishStart))
}

func entriesByNode(trs []trigger.Trigger) map[flow.NodeID]*trigger.Trigger {
	out := make(map[flow.NodeID]*trigger.Trigger, len(trs))
	for i := range trs {
		out[trs[i].NodeID] = &trs[i]
	}
	return out
}

// runLoad runs acceptance reads from several workers at the requested rate
// for the duration, calls midway halfway through, and returns what the workers
// saw, in the order each worker saw it.
func runLoad(t *testing.T, e *env, id flow.FlowID, load Load, midway func()) [][]observation {
	t.Helper()
	const workers = 8
	interval := time.Duration(float64(time.Second) * workers / float64(load.Rate))
	deadline := time.Now().Add(load.Duration)
	results := make([][]observation, workers)
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tick := time.NewTicker(interval)
			defer tick.Stop()
			for time.Now().Before(deadline) {
				<-tick.C
				start := time.Now()
				acc, err := e.svc.Accept(e.ctx(), e.scopeA, id, "hook")
				results[w] = append(results[w], observation{start: start, end: time.Now(), version: acc.VersionNumber, accepting: acc.Accepting, err: err})
			}
		}()
	}
	<-time.After(load.Duration / 2)
	midway()
	wg.Wait()
	return results
}

func checkObservations(t *testing.T, obs [][]observation, publishStart, publishEnd time.Time) {
	t.Helper()
	sawOne, sawTwo := false, false
	for w, list := range obs {
		last := int64(0)
		for i := range list {
			o := &list[i]
			switch {
			case o.err != nil || !o.accepting:
				t.Fatalf("worker %d read %d was refused during the switch: accepting=%v err=%v", w, i, o.accepting, o.err)
			case o.version != 1 && o.version != 2:
				t.Fatalf("worker %d read %d names version %d, want 1 or 2", w, i, o.version)
			case o.version < last:
				t.Fatalf("worker %d read %d went back from version %d to %d", w, i, last, o.version)
			case o.end.Before(publishStart) && o.version != 1:
				t.Fatalf("worker %d read %d finished before the publication began but names version %d", w, i, o.version)
			case o.start.After(publishEnd) && o.version != 2:
				t.Fatalf("worker %d read %d began after the publication returned but names version %d", w, i, o.version)
			}
			last = o.version
			sawOne = sawOne || o.version == 1
			sawTwo = sawTwo || o.version == 2
		}
	}
	if !sawOne || !sawTwo {
		t.Fatalf("the load did not straddle the switch: saw version 1=%v version 2=%v", sawOne, sawTwo)
	}
}
