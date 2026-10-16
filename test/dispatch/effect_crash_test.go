// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dispatch_test

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/connector/declarative"
	"github.com/wirvii/orcon/internal/connector/dispatch"
	"github.com/wirvii/orcon/internal/connector/limit"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/durable"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/journal"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// The journal timing of these tests: a crashed attempt is recovered once the
// dispatch deadline and the grace have passed on the database clock, which these
// stores keep in real time, so a test waits that window out.
const (
	crashAttemptTimeout = 1500 * time.Millisecond
	crashGrace          = 100 * time.Millisecond
)

// providerMock is the external system: it deduplicates by Idempotency-Key, so
// a key produces one effect however often it is sent, and a request without a
// key is an effect of its own. It can simulate a crash: the effect happens, then
// the caller's context is canceled before the answer is read.
type providerMock struct {
	mu       sync.Mutex
	requests int
	effects  map[string]int
	crash    func() // called after the effect of a request, to cancel the caller
}

func newProviderMock() *providerMock {
	return &providerMock{effects: map[string]int{}}
}

func (m *providerMock) Send(ctx context.Context, req declarative.HTTPRequest) (declarative.HTTPResponse, error) {
	if err := ctx.Err(); err != nil {
		return declarative.HTTPResponse{}, err
	}
	m.mu.Lock()
	m.requests++
	key := ""
	for name, values := range req.Header {
		if strings.EqualFold(name, dispatch.IdempotencyHeader) && len(values) > 0 {
			key = values[0]
		}
	}
	if key == "" {
		m.effects["request-"+strconv.Itoa(m.requests)] = 1
	} else {
		m.effects[key] = 1
	}
	crash := m.crash
	m.mu.Unlock()
	if crash != nil {
		crash()
		return declarative.HTTPResponse{}, ctx.Err()
	}
	return declarative.HTTPResponse{Status: 200, Header: map[string][]string{}, Body: []byte(`{"id":7}`)}, nil
}

func (m *providerMock) counts() (requests, effects int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.requests, len(m.effects)
}

// scopes issues the scope of the test organization.
type scopes struct{}

func (scopes) ScopeOf(org identity.OrgID) (orgscope.Org, error) { return storetest.ScopeOf(org), nil }

// bed is one execution with a token leased to this worker, and an HTTP node
// that runs through the journal of the real store.
type bed struct {
	t      *testing.T
	env    *storetest.EngineEnv
	exec   store.ExecutionID
	tok    store.TokenID
	nodeEx store.NodeExecutionID
	fence  store.Fence
	mock   *providerMock
	// checkpoints counts the checkpoint transactions of the bed; afterCheckpoint, when
	// set, runs after each of them and may cancel the caller.
	checkpoints     int
	afterCheckpoint func(n int)
}

func newBed(t *testing.T, env *storetest.EngineEnv) *bed {
	t.Helper()
	doc := executiontest.NewFlow("effects").Start("s").Task("t1").End("e").Chain("s", "t1", "e").Doc()
	pub := env.Publish(&doc)
	eng := env.Engine(durable.Config{}, execution.Deps{Runner: noRunner{}})
	execID := env.Start(eng, pub, executiontest.Batch(`{"n":1}`))
	claimed, _, err := eng.Claim(t.Context(), 1)
	if err != nil || len(claimed) != 1 {
		t.Fatalf("claim = %d, %v", len(claimed), err)
	}
	c := claimed[0]
	return &bed{
		t: t, env: env, exec: execID, tok: c.TokenID,
		nodeEx: store.NodeExecutionID(env.IDs.New()),
		fence:  store.Fence{OrgID: env.Org, Token: c.TokenID, Epoch: c.LeaseEpoch, Owner: eng.Worker()},
		mock:   newProviderMock(),
	}
}

// noRunner runs no node: the bed drives the HTTP node directly.
type noRunner struct{}

func (noRunner) Execute(context.Context, execution.NodeCall) (execution.NodeOutput, error) {
	return execution.NodeOutput{}, errors.New("the effect tests do not run through the engine")
}

// checkpoint is the transaction the durable engine opens before an effect: it
// locks the execution, checks the lease fence, records the node execution as
// RUNNING, and runs the node's writes.
func (b *bed) checkpoint(ctx context.Context, fn func(ctx context.Context, tx any) error) error {
	now := b.env.Clock.Now()
	err := b.env.Store.InTx(ctx, storetest.ScopeOf(b.env.Org), func(ctx context.Context, tx store.Tx) error {
		if _, err := tx.LockExecution(ctx, b.env.Org, b.exec); err != nil {
			return err
		}
		if err := tx.AssertFence(ctx, b.fence, now); err != nil {
			return err
		}
		if err := tx.UpsertNodeExecution(ctx, store.NodeExecution{
			ID: b.nodeEx, OrgID: b.env.Org, ExecutionID: b.exec, TokenID: b.tok, NodeID: "http",
			Status: "RUNNING", StartedAt: now, CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			return err
		}
		return fn(ctx, tx)
	})
	if err != nil {
		return err
	}
	b.checkpoints++
	if b.afterCheckpoint != nil {
		b.afterCheckpoint(b.checkpoints)
	}
	return nil
}

// run executes the HTTP node once for the given items, as one activation of the worker.
func (b *bed) run(ctx context.Context, params string, items int) (execution.NodeOutput, error) {
	b.t.Helper()
	p, err := jsonvalue.Parse([]byte(params))
	if err != nil {
		b.t.Fatalf("params: %v", err)
	}
	node, err := dispatch.New(dispatch.Config{
		Journal:   b.env.Store,
		Scopes:    scopes{},
		Buckets:   b.env.Store,
		SenderFor: func(*url.URL) declarative.Sender { return b.mock },
		Bucket:    limit.Spec{Capacity: 100, RefillPerSecond: 50},
		IDs:       id.NewV7Generator(time.Now),
		Clock:     clock.System{},
		JournalCfg: journal.Config{
			AttemptTimeout: crashAttemptTimeout,
			Grace:          crashGrace,
		},
	})
	if err != nil {
		b.t.Fatalf("dispatch.New: %v", err)
	}
	batch := make([]execution.Item, items)
	for i := range batch {
		v, _ := jsonvalue.Parse([]byte(`{"n":1}`))
		batch[i] = execution.Item{JSON: v}
	}
	call := execution.NodeCall{
		Org:           b.env.Org,
		Execution:     execution.ID(b.exec.String()),
		Token:         execution.TokenID(b.tok.String()),
		NodeExecution: execution.NodeExecutionID(b.nodeEx.String()),
		Node:          &flow.Node{ID: "http", Type: dispatch.HTTPNodeType, Parameters: p},
		Input:         execution.NewBatch(batch...),
		Checkpoint:    b.checkpoint,
	}
	return node.Execute(ctx, call)
}

// rows returns the journal of the node execution, by item.
func (b *bed) rows(t *testing.T) []store.ExternalOperation {
	t.Helper()
	var out []store.ExternalOperation
	var fromItem int64
	var fromID store.ExternalOperationID
	for {
		page, err := b.env.Store.ListExternalOperations(t.Context(), storetest.ScopeOf(b.env.Org), b.nodeEx, fromItem, fromID, 100)
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		out = append(out, page...)
		if len(page) < 100 {
			return out
		}
		last := page[len(page)-1]
		fromItem, fromID = last.ItemIndex, last.ID
	}
}

const idempotentPost = `{"method":"POST","url":"https://api.example.com/things","body":{"a":1},"idempotent":true}`
const plainPost = `{"method":"POST","url":"https://api.example.com/things","body":{"a":1}}`

// TestCrashAfterIntentBeforeDispatching: the process stops after the intent is
// committed and before the first mark. A re-run finds the same rows and sends each
// item exactly once.
func TestCrashAfterIntentBeforeDispatching(t *testing.T) {
	eachStore(t, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		b := newBed(t, env)
		ctx, stop := context.WithCancel(t.Context())
		b.afterCheckpoint = func(n int) {
			if n == 1 {
				stop() // the intent is committed; the process dies before the mark
			}
		}
		if _, err := b.run(ctx, plainPost, 2); err == nil {
			t.Fatal("the stopped run reported success")
		}
		if req, _ := b.mock.counts(); req != 0 {
			t.Fatalf("a request was sent before the mark: %d", req)
		}
		intent := b.rows(t)
		if len(intent) != 2 || intent[0].State != journal.StatePrepared {
			t.Fatalf("after the intent: %d rows, first %q", len(intent), intent[0].State)
		}

		b.afterCheckpoint = nil
		out, err := b.run(t.Context(), plainPost, 2)
		if err != nil {
			t.Fatalf("re-run: %v", err)
		}
		if got := len(out.Ports[0].Batch.Items()); got != 2 {
			t.Fatalf("outputs = %d, want 2", got)
		}
		req, effects := b.mock.counts()
		if req != 2 || effects != 2 {
			t.Fatalf("requests = %d, effects = %d; want exactly one per item", req, effects)
		}
		rows := b.rows(t)
		if len(rows) != 2 || rows[0].CorrelationKey != intent[0].CorrelationKey || rows[1].CorrelationKey != intent[1].CorrelationKey {
			t.Fatal("the re-run changed the journal rows or their correlation keys")
		}
		for i := range rows {
			if rows[i].State != journal.StateSucceeded {
				t.Fatalf("item %d is %q", i, rows[i].State)
			}
		}
	})
}

// TestCrashAfterDispatchingBeforeSend: the mark is committed and the process dies
// before any request leaves. Without an idempotency key nothing is sent again and
// the item becomes UNCERTAIN; with a key the recovery resends with the same key and
// the provider deduplicates it.
func TestCrashAfterDispatchingBeforeSend(t *testing.T) {
	eachStore(t, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		t.Run("without key", func(t *testing.T) {
			b := newBed(t, env)
			ctx, stop := context.WithCancel(t.Context())
			b.afterCheckpoint = func(n int) {
				if n == 2 { // the intent is 1, the mark is 2
					stop()
				}
			}
			if _, err := b.run(ctx, plainPost, 1); err == nil {
				t.Fatal("the stopped run reported success")
			}
			if req, _ := b.mock.counts(); req != 0 {
				t.Fatalf("a request was sent after the crash point: %d", req)
			}
			time.Sleep(crashAttemptTimeout + crashGrace + 300*time.Millisecond)
			b.afterCheckpoint = nil
			_, err := b.run(t.Context(), plainPost, 1)
			if !errors.Is(err, dispatch.ErrUnknownOutcome) {
				t.Fatalf("re-run error = %v, want ErrUnknownOutcome", err)
			}
			if req, _ := b.mock.counts(); req != 0 {
				t.Fatalf("an unsafe item was sent on recovery: %d", req)
			}
			if got := b.rows(t)[0].State; got != journal.StateUncertain {
				t.Fatalf("state = %q, want UNCERTAIN", got)
			}
		})
		t.Run("with key", func(t *testing.T) {
			b := newBed(t, env)
			ctx, stop := context.WithCancel(t.Context())
			b.afterCheckpoint = func(n int) {
				if n == 2 {
					stop()
				}
			}
			if _, err := b.run(ctx, idempotentPost, 1); err == nil {
				t.Fatal("the stopped run reported success")
			}
			time.Sleep(crashAttemptTimeout + crashGrace + 300*time.Millisecond)
			b.afterCheckpoint = nil
			if _, err := b.run(t.Context(), idempotentPost, 1); err != nil {
				t.Fatalf("re-run: %v", err)
			}
			req, effects := b.mock.counts()
			if req != 1 || effects != 1 {
				t.Fatalf("requests = %d, effects = %d; want the key sent once and deduplicated", req, effects)
			}
			if got := b.rows(t)[0].State; got != journal.StateSucceeded {
				t.Fatalf("state = %q, want SUCCEEDED", got)
			}
		})
	})
}

// TestCrashAfterSendBeforeCommit: the provider performs the effect and the process
// dies before the outcome is recorded. Without a key the item is UNCERTAIN and is
// never resent; with a key the recovery resends it with the same key, which the
// provider deduplicates.
func TestCrashAfterSendBeforeCommit(t *testing.T) {
	eachStore(t, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		t.Run("without key", func(t *testing.T) {
			b := newBed(t, env)
			ctx, stop := context.WithCancel(t.Context())
			b.mock.crash = stop // the effect happens, then the caller dies
			if _, err := b.run(ctx, plainPost, 1); err == nil {
				t.Fatal("the crashed run reported success")
			}
			time.Sleep(crashAttemptTimeout + crashGrace + 300*time.Millisecond)
			b.mock.crash = nil
			if _, err := b.run(t.Context(), plainPost, 1); !errors.Is(err, dispatch.ErrUnknownOutcome) {
				t.Fatalf("re-run error = %v, want ErrUnknownOutcome", err)
			}
			if req, effects := b.mock.counts(); req != 1 || effects != 1 {
				t.Fatalf("requests = %d, effects = %d; an unsafe effect must not be repeated", req, effects)
			}
		})
		t.Run("with key", func(t *testing.T) {
			b := newBed(t, env)
			ctx, stop := context.WithCancel(t.Context())
			b.mock.crash = stop
			if _, err := b.run(ctx, idempotentPost, 1); err == nil {
				t.Fatal("the crashed run reported success")
			}
			time.Sleep(crashAttemptTimeout + crashGrace + 300*time.Millisecond)
			b.mock.crash = nil
			if _, err := b.run(t.Context(), idempotentPost, 1); err != nil {
				t.Fatalf("re-run: %v", err)
			}
			req, effects := b.mock.counts()
			if req != 2 || effects != 1 {
				t.Fatalf("requests = %d, effects = %d; the resend must carry the key and be deduplicated", req, effects)
			}
			if got := b.rows(t)[0].State; got != journal.StateSucceeded {
				t.Fatalf("state = %q, want SUCCEEDED", got)
			}
		})
	})
}

// TestLostLeaseStopsTheEffect: a worker whose lease was taken over writes nothing and
// sends nothing; the checkpoint fails with the fence error.
func TestLostLeaseStopsTheEffect(t *testing.T) {
	eachStore(t, func(t *testing.T, env *storetest.EngineEnv) {
		t.Helper()
		b := newBed(t, env)
		b.fence.Epoch++ // another holder took the token over
		_, err := b.run(t.Context(), plainPost, 1)
		if !errors.Is(err, store.ErrFenced) {
			t.Fatalf("err = %v, want ErrFenced", err)
		}
		if req, _ := b.mock.counts(); req != 0 {
			t.Fatalf("a fenced worker sent %d request(s)", req)
		}
		if rows := b.rows(t); len(rows) != 0 {
			t.Fatalf("a fenced worker wrote %d journal rows", len(rows))
		}
	})
}
