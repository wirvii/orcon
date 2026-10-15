// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package dispatch

import (
	"context"
	"errors"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/connector/declarative"
	"github.com/wirvii/orcon/internal/connector/limit"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/journal"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/internal/platform/orgscope"
	"github.com/wirvii/orcon/internal/platform/orgscope/issue"
)

var (
	testOrg  = identity.OrgID(id.Derive("org"))
	testExec = id.Derive("exec")
	testNode = id.Derive("node-execution")
	testTok  = id.Derive("token")
	t0       = time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
)

// fakeSender answers from a script and records every request.
type fakeSender struct {
	mu       sync.Mutex
	requests []declarative.HTTPRequest
	respond  func(n int, req declarative.HTTPRequest) (declarative.HTTPResponse, error)
}

func (f *fakeSender) Send(_ context.Context, req declarative.HTTPRequest) (declarative.HTTPResponse, error) {
	f.mu.Lock()
	f.requests = append(f.requests, req)
	n := len(f.requests)
	f.mu.Unlock()
	return f.respond(n, req)
}

func (f *fakeSender) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requests)
}

func ok(body string) (declarative.HTTPResponse, error) {
	return declarative.HTTPResponse{Status: 200, Header: map[string][]string{}, Body: []byte(body)}, nil
}

// fakeBuckets grants every take unless refuse is set.
type fakeBuckets struct {
	refuse bool
}

func (b *fakeBuckets) TakeBucketPermits(_ context.Context, _ identity.OrgID, r store.BucketRequest) (store.BucketVerdict, error) {
	if b.refuse {
		return store.BucketVerdict{Granted: false, RetryAt: r.Now.Add(time.Second)}, nil
	}
	return store.BucketVerdict{Granted: true}, nil
}

func (b *fakeBuckets) BlockBucket(context.Context, identity.OrgID, string, time.Time, time.Time) error {
	return nil
}

// fakeJournal is an in-memory store of external operations that counts its
// transactions.
type fakeJournal struct {
	mu   sync.Mutex
	rows map[store.ExternalOperationID]*store.ExternalOperation
	txs  int
}

func newFakeJournal() *fakeJournal {
	return &fakeJournal{rows: map[store.ExternalOperationID]*store.ExternalOperation{}}
}

func (f *fakeJournal) InTx(ctx context.Context, _ orgscope.Org, fn func(ctx context.Context, tx store.Tx) error) error {
	f.mu.Lock()
	f.txs++
	f.mu.Unlock()
	return fn(ctx, &fakeJournalTx{j: f})
}

func (f *fakeJournal) ListExternalOperations(_ context.Context, _ orgscope.Org, node store.NodeExecutionID, from int64, _ store.ExternalOperationID, _ int) ([]store.ExternalOperation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []store.ExternalOperation
	for _, r := range f.rows {
		if r.NodeExecutionID == node && r.ItemIndex >= from {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (f *fakeJournal) transactions() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.txs
}

type fakeJournalTx struct {
	store.Tx
	j *fakeJournal
}

func (t *fakeJournalTx) AppendJournal(_ context.Context, ops []store.ExternalOperation) error {
	t.j.mu.Lock()
	defer t.j.mu.Unlock()
	for i := range ops {
		o := ops[i]
		o.State = journal.StatePrepared
		t.j.rows[o.ID] = &o
	}
	return nil
}

// InsertJournalIfAbsent keeps the rows whose correlation key the journal holds, as the adapters do.
func (t *fakeJournalTx) InsertJournalIfAbsent(_ context.Context, ops []store.ExternalOperation) error {
	t.j.mu.Lock()
	defer t.j.mu.Unlock()
	for i := range ops {
		if t.j.hasKey(ops[i].CorrelationKey) {
			continue
		}
		o := ops[i]
		o.State = journal.StatePrepared
		t.j.rows[o.ID] = &o
	}
	return nil
}

func (f *fakeJournal) hasKey(key string) bool {
	for _, r := range f.rows {
		if r.CorrelationKey == key {
			return true
		}
	}
	return false
}

func (t *fakeJournalTx) MarkDispatching(_ context.Context, ids []store.ExternalOperationID, attemptTimeout time.Duration) (int64, error) {
	t.j.mu.Lock()
	defer t.j.mu.Unlock()
	var n int64
	now := time.Now().UTC()
	deadline := now.Add(attemptTimeout)
	for _, id := range ids {
		if r := t.j.rows[id]; r != nil && r.State == journal.StatePrepared {
			r.State, r.DispatchDeadlineAt, r.FirstDispatchedAt = journal.StateDispatching, deadline, now
			n++
		}
	}
	return n, nil
}

func (t *fakeJournalTx) DatabaseNow(context.Context) (time.Time, error) {
	return time.Now().UTC(), nil
}

func (t *fakeJournalTx) ResolveOperation(_ context.Context, op store.ExternalOperationID, from, to, response string, _ time.Time) (bool, error) {
	t.j.mu.Lock()
	defer t.j.mu.Unlock()
	r := t.j.rows[op]
	if r == nil || r.State != from {
		return false, nil
	}
	r.State, r.ResponseReference = to, response
	return true, nil
}

// httpNode builds the flow node of the core HTTP node with the given parameters.
func httpNode(t *testing.T, params string) *flow.Node {
	t.Helper()
	p, err := jsonvalue.Parse([]byte(params))
	if err != nil {
		t.Fatalf("parse params: %v", err)
	}
	return &flow.Node{ID: "http", Type: HTTPNodeType, Parameters: p}
}

func newNode(t *testing.T, sender *fakeSender, buckets store.LimitBuckets, j *fakeJournal) *HTTPNode {
	t.Helper()
	n, err := New(Config{
		Journal:   j,
		Scopes:    testScopes{},
		Buckets:   buckets,
		SenderFor: func(*url.URL) declarative.Sender { return sender },
		Bucket:    limit.Spec{Capacity: 100, RefillPerSecond: 50},
		IDs:       id.NewV7Generator(func() time.Time { return t0 }),
		Clock:     clock.NewFake(t0),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return n
}

func callOf(node *flow.Node, items int) execution.NodeCall {
	batch := make([]execution.Item, items)
	for i := range batch {
		v, _ := jsonvalue.Parse([]byte(`{"n":1}`))
		batch[i] = execution.Item{JSON: v}
	}
	return execution.NodeCall{
		Org:           testOrg,
		Execution:     execution.ID(testExec.String()),
		Token:         execution.TokenID(testTok.String()),
		NodeExecution: execution.NodeExecutionID(testNode.String()),
		Node:          node,
		Scope:         "",
		Input:         execution.NewBatch(batch...),
	}
}

func TestReadRunsOncePerItemWithoutJournal(t *testing.T) {
	sender := &fakeSender{respond: func(int, declarative.HTTPRequest) (declarative.HTTPResponse, error) {
		return ok(`{"id":7}`)
	}}
	j := newFakeJournal()
	n := newNode(t, sender, &fakeBuckets{}, j)
	out, err := n.Execute(t.Context(), callOf(httpNode(t, `{"url":"https://api.example.com/items?x=1"}`), 2))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if sender.count() != 2 {
		t.Fatalf("sends = %d, want 2", sender.count())
	}
	if j.transactions() != 0 {
		t.Fatalf("a read opened %d journal transactions", j.transactions())
	}
	items := out.Ports[0].Batch.Items()
	if len(items) != 2 {
		t.Fatalf("outputs = %d", len(items))
	}
	if got := sender.requests[0].URL; got != "https://api.example.com/items?x=1" {
		t.Fatalf("url = %s", got)
	}
}

func TestUnsafeEffectIsNeverResentAfterAnUnknownOutcome(t *testing.T) {
	sender := &fakeSender{respond: func(int, declarative.HTTPRequest) (declarative.HTTPResponse, error) {
		return declarative.HTTPResponse{Status: 503, Header: map[string][]string{}, Body: []byte(`{}`)}, nil
	}}
	j := newFakeJournal()
	n := newNode(t, sender, &fakeBuckets{}, j)
	_, err := n.Execute(t.Context(), callOf(httpNode(t, `{"method":"POST","url":"https://api.example.com/things","body":{"a":1}}`), 1))
	if !errors.Is(err, ErrUnknownOutcome) {
		t.Fatalf("err = %v, want ErrUnknownOutcome", err)
	}
	if sender.count() != 1 {
		t.Fatalf("an unsafe effect was sent %d times", sender.count())
	}
	if j.transactions() == 0 {
		t.Fatal("an effect did not go through the journal")
	}
	if got := string(sender.requests[0].Body); got != `{"a":1}` {
		t.Fatalf("body = %s", got)
	}
}

func TestIdempotentEffectSendsItsCorrelationKey(t *testing.T) {
	sender := &fakeSender{respond: func(int, declarative.HTTPRequest) (declarative.HTTPResponse, error) {
		return declarative.HTTPResponse{Status: 201, Header: map[string][]string{}, Body: []byte(`{"id":"x"}`)}, nil
	}}
	j := newFakeJournal()
	n := newNode(t, sender, &fakeBuckets{}, j)
	out, err := n.Execute(t.Context(), callOf(httpNode(t, `{"method":"POST","url":"https://api.example.com/things","idempotent":true}`), 1))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	key := sender.requests[0].Header[IdempotencyHeader]
	if len(key) != 1 || len(key[0]) != 26 {
		t.Fatalf("idempotency key = %v", key)
	}
	if len(out.Ports[0].Batch.Items()) != 1 {
		t.Fatal("no output for the effect")
	}
}

func TestThrottledBeforeAnySend(t *testing.T) {
	sender := &fakeSender{respond: func(int, declarative.HTTPRequest) (declarative.HTTPResponse, error) {
		return ok(`{}`)
	}}
	j := newFakeJournal()
	n := newNode(t, sender, &fakeBuckets{refuse: true}, j)
	_, err := n.Execute(t.Context(), callOf(httpNode(t, `{"method":"POST","url":"https://api.example.com/things"}`), 1))
	if !errors.Is(err, ErrThrottled) {
		t.Fatalf("err = %v, want ErrThrottled", err)
	}
	if sender.count() != 0 || j.transactions() != 0 {
		t.Fatalf("sends=%d journal=%d after a refusal", sender.count(), j.transactions())
	}
}

func TestPermanentFailureIsNotUnknown(t *testing.T) {
	sender := &fakeSender{respond: func(int, declarative.HTTPRequest) (declarative.HTTPResponse, error) {
		return declarative.HTTPResponse{Status: 404, Header: map[string][]string{}, Body: []byte(`{}`)}, nil
	}}
	n := newNode(t, sender, &fakeBuckets{}, newFakeJournal())
	_, err := n.Execute(t.Context(), callOf(httpNode(t, `{"url":"https://api.example.com/missing"}`), 1))
	if err == nil || errors.Is(err, ErrUnknownOutcome) {
		t.Fatalf("err = %v", err)
	}
}

func TestParametersRefuseTemplateMarkers(t *testing.T) {
	sender := &fakeSender{respond: func(int, declarative.HTTPRequest) (declarative.HTTPResponse, error) { return ok(`{}`) }}
	n := newNode(t, sender, &fakeBuckets{}, newFakeJournal())
	_, err := n.Execute(t.Context(), callOf(httpNode(t, `{"url":"https://api.example.com/{{$credentials.token}}"}`), 1))
	if err == nil {
		t.Fatal("a URL with a template marker was accepted")
	}
	_, err = n.Execute(t.Context(), callOf(httpNode(t, `{"url":"https://api.example.com/","headers":{"X-A":"{{$credentials.token}}"}}`), 1))
	if err == nil || sender.count() != 0 {
		t.Fatalf("a header with a template marker was accepted: %v", err)
	}
}

func TestNodeTypeIsChecked(t *testing.T) {
	n := newNode(t, &fakeSender{respond: func(int, declarative.HTTPRequest) (declarative.HTTPResponse, error) { return ok(`{}`) }}, &fakeBuckets{}, newFakeJournal())
	node := httpNode(t, `{"url":"https://api.example.com/"}`)
	node.Type = "orcon.other@1"
	if _, err := n.Execute(t.Context(), callOf(node, 1)); err == nil {
		t.Fatal("another node type was run by the HTTP node")
	}
	_ = flow.PortMain
}

// testScopes issues the scopes of the test organizations (tests may build scopes; production code gets them from the store).
type testScopes struct{}

func (testScopes) ScopeOf(org identity.OrgID) (orgscope.Org, error) {
	return issue.New(id.UUID(org))
}
