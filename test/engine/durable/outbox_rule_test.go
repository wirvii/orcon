// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable_test

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/durable"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/identity"
)

// topicDeliverer records the topic of every delivery.
type topicDeliverer struct {
	mu     sync.Mutex
	topics []string
}

func (d *topicDeliverer) Deliver(_ context.Context, dl durable.Delivery) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.topics = append(d.topics, dl.Topic)
	return nil
}

// TestEveryEventGoesThroughTheOutboxRule (BL-069): the events the store appends by
// itself (the creation and start of an execution, an expired lease, a poisoned
// token, a suspended execution) go through the same outbox rule as the ones a
// commit appends, in the same transaction as the state change.
func TestEveryEventGoesThroughTheOutboxRule(t *testing.T) {
	eachStore(t, storetest.EngineOptions{Weak: true}, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		ctx := t.Context()
		pub := env.Publish(docOf(sequence(1)))
		rule := func(_ identity.OrgID, _ store.ExecutionID, e execution.Event) (string, []byte, bool) {
			return "event." + string(e.Type), []byte(`{"type":"` + string(e.Type) + `"}`), true
		}
		deliverer := &topicDeliverer{}
		eng := env.Engine(durable.Config{Concurrency: 1, LeaseTTL: 50 * time.Millisecond, Outbox: rule, Deliver: deliverer}, runner())
		execID := env.Start(eng, pub, input())
		// The token is claimed and its lease expires, three times: the third abandonment poisons it.
		for round := 1; round <= 3; round++ {
			if claimed, _, err := eng.Claim(ctx, 1); err != nil || len(claimed) != 1 {
				t.Fatalf("round %d: claim = %d, %v", round, len(claimed), err)
			}
			time.Sleep(150 * time.Millisecond)
			if n, err := eng.Reap(ctx); err != nil || n != 1 {
				t.Fatalf("round %d: reap = %d, %v", round, n, err)
			}
		}
		for {
			n, err := eng.Dispatch(ctx, 100)
			if err != nil {
				t.Fatal(err)
			}
			if n == 0 {
				break
			}
		}
		stored := map[string]int{}
		events := env.Events(execID)
		for i := range events {
			stored["event."+events[i].Type]++
		}
		delivered := map[string]int{}
		for _, topic := range deliverer.topics {
			delivered[topic]++
		}
		for _, want := range []string{"event.execution.created", "event.execution.started", "event.lease.expired", "event.token.poisoned", "event.execution.suspended"} {
			if stored[want] == 0 {
				t.Errorf("the scenario stored no %s event", want)
			}
		}
		for topic, n := range stored {
			if delivered[topic] != n {
				t.Errorf("%d %s events stored and %d delivered through the outbox", n, topic, delivered[topic])
			}
		}
		if slices.Contains(deliverer.topics, "") {
			t.Error("a delivery without a topic")
		}
	})
}
