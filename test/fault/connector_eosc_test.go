// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package fault_test runs the fault scenarios of the connector layer through
// the durable engine, on the real stores (SPEC-003 E-EOSC-1, ADR-0012, ADR-0014
// section 2.4): a worker that falls in the middle of an external effect, an
// effect whose outcome is unknown, and a burst of work larger than the rate
// bucket of its credential and host.
package fault_test

import (
	"context"
	"flag"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"

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

func TestMain(m *testing.M) {
	flag.Parse()
	kinds, _ := storetest.KindsFor(executiontest.StoreFlag("auto"))
	if slices.Contains(kinds, storetest.PostgreSQL) {
		storetest.EnablePostgres()
		os.Exit(runWithPostgres(m)) //nolint:forbidigo // AL-8: TestMain ends the test process with the code of the run
	}
	goleak.VerifyTestMain(m)
}

func eachStore(t *testing.T, fn func(t *testing.T, env *storetest.EngineEnv)) {
	t.Helper()
	kinds, err := storetest.KindsFor(executiontest.StoreFlag("auto"))
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range kinds {
		t.Run(string(kind), func(t *testing.T) {
			t.Helper()
			fn(t, storetest.NewEngineEnv(t, kind, storetest.EngineOptions{Weak: true}))
		})
	}
}

// The journal timing of these tests: a crashed attempt is recovered once the
// dispatch deadline and the grace have passed on the database clock.
const (
	attemptTimeout = 1200 * time.Millisecond
	grace          = 100 * time.Millisecond
)

const (
	keyedPost = `{"method":"POST","url":"https://api.example.com/things","body":{"a":1},"idempotent":true}`
	plainPost = `{"method":"POST","url":"https://api.example.com/things","body":{"a":1}}`
	plainGet  = `{"method":"GET","url":"https://api.example.com/things"}`
)

// provider is the external system: it deduplicates by Idempotency-Key, a request
// without a key is an effect of its own, it can fall the caller after the effect
// happened, and it can answer with a status.
type provider struct {
	mu       sync.Mutex
	requests int
	effects  map[string]int
	crash    func()
	status   int
}

func (p *provider) Send(ctx context.Context, req declarative.HTTPRequest) (declarative.HTTPResponse, error) {
	if err := ctx.Err(); err != nil {
		return declarative.HTTPResponse{}, err
	}
	p.mu.Lock()
	p.requests++
	key := ""
	for name, values := range req.Header {
		if strings.EqualFold(name, dispatch.IdempotencyHeader) && len(values) > 0 {
			key = values[0]
		}
	}
	if key == "" {
		key = "request-" + strconv.Itoa(p.requests)
	}
	if p.effects == nil {
		p.effects = map[string]int{}
	}
	p.effects[key] = 1
	crash, status := p.crash, p.status
	p.mu.Unlock()
	if crash != nil {
		crash()
		return declarative.HTTPResponse{}, ctx.Err()
	}
	if status == 0 {
		status = 200
	}
	return declarative.HTTPResponse{Status: status, Header: map[string][]string{}, Body: []byte(`{"id":7}`)}, nil
}

func (p *provider) counts() (requests, effects int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.requests, len(p.effects)
}

func (p *provider) set(crash func(), status int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.crash, p.status = crash, status
}

type scopes struct{}

func (scopes) ScopeOf(org identity.OrgID) (orgscope.Org, error) { return storetest.ScopeOf(org), nil }

// countingBuckets counts the refusals of the bucket store.
type countingBuckets struct {
	store.LimitBuckets
	mu       sync.Mutex
	refusals int
}

func (c *countingBuckets) TakeBucketPermits(ctx context.Context, org identity.OrgID, r store.BucketRequest) (store.BucketVerdict, error) {
	v, err := c.LimitBuckets.TakeBucketPermits(ctx, org, r)
	if err == nil && !v.Granted {
		c.mu.Lock()
		c.refusals++
		c.mu.Unlock()
	}
	return v, err
}

func (c *countingBuckets) refused() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.refusals
}

// rig is a flow with one HTTP node, a provider, and engines that run it over a store.
type rig struct {
	t       *testing.T
	env     *storetest.EngineEnv
	mock    *provider
	buckets *countingBuckets
	runner  execution.NodeRunner
	pub     *storetest.Published
}

func newRig(t *testing.T, env *storetest.EngineEnv, params string, bucket limit.Spec) *rig {
	t.Helper()
	r := &rig{t: t, env: env, mock: &provider{}, buckets: &countingBuckets{LimitBuckets: env.Store}}
	node, err := dispatch.New(dispatch.Config{
		Journal:    env.Store,
		Scopes:     scopes{},
		Buckets:    r.buckets,
		SenderFor:  func(*url.URL) declarative.Sender { return r.mock },
		Bucket:     bucket,
		IDs:        id.NewV7Generator(time.Now),
		Clock:      clock.System{},
		JournalCfg: journal.Config{AttemptTimeout: attemptTimeout, Grace: grace},
	})
	if err != nil {
		t.Fatalf("dispatch.New: %v", err)
	}
	r.runner = dispatch.NewRouter(map[string]execution.NodeRunner{dispatch.HTTPNodeType: node})
	p, err := jsonvalue.Parse([]byte(params))
	if err != nil {
		t.Fatalf("parameters: %v", err)
	}
	doc := executiontest.NewFlow("connector").Start("s").Task("http").End("e").Chain("s", "http", "e").
		Modify("http", func(n *flow.Node) { n.Type, n.Parameters = dispatch.HTTPNodeType, p }).Doc()
	r.pub = env.Publish(&doc)
	return r
}

func (r *rig) engine() *durable.Engine {
	r.t.Helper()
	return r.env.Engine(durable.Config{Concurrency: 1, PollEvery: 20 * time.Millisecond}, execution.Deps{Runner: r.runner})
}

func (r *rig) status(exec store.ExecutionID) string { return r.env.Execution(exec).Status }

// claimOne claims the one token that becomes claimable within the time given.
func (r *rig) claimOne(eng *durable.Engine, within time.Duration) (store.Claimed, time.Time, bool) {
	r.t.Helper()
	stop := time.Now().Add(within)
	for {
		claimed, start, err := eng.Claim(r.t.Context(), 1)
		if err != nil {
			r.t.Fatalf("claim: %v", err)
		}
		if len(claimed) == 1 {
			return claimed[0], start, true
		}
		if time.Now().After(stop) {
			return store.Claimed{}, time.Time{}, false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (r *rig) work(eng *durable.Engine, c store.Claimed, start time.Time) {
	r.t.Helper()
	if err := eng.Work(r.t.Context(), c, start); err != nil {
		r.t.Fatalf("work: %v", err)
	}
}

// crashInTheEffect runs the execution on a first worker that falls right after
// the provider performed the effect, before the outcome is recorded, and then
// gives its lease back as the restart of that worker does. It returns the id of
// the execution; the effect is in the journal as DISPATCHING.
func (r *rig) crashInTheEffect() store.ExecutionID {
	r.t.Helper()
	first := r.engine()
	exec := r.env.Start(first, r.pub, executiontest.Batch(`{"n":1}`))
	ctx, fall := context.WithCancel(r.t.Context())
	defer fall()
	r.mock.set(fall, 0)
	claimed, start, err := first.Claim(ctx, 1)
	if err != nil || len(claimed) != 1 {
		r.t.Fatalf("claim = %d, %v", len(claimed), err)
	}
	if err := first.Work(ctx, claimed[0], start); err == nil {
		r.t.Fatal("the worker that fell reported success")
	}
	r.mock.set(nil, 0)
	if req, _ := r.mock.counts(); req != 1 {
		r.t.Fatalf("requests before the restart = %d, want 1", req)
	}
	if _, err := r.env.Store.ReleaseLostLeases(r.t.Context(), first.Worker(), 3, time.Now()); err != nil {
		r.t.Fatalf("release the lease of the worker that fell: %v", err)
	}
	return exec
}

// TestConnectorEOSC (SPEC-003 E-EOSC-1): an effect whose worker fell is never sent
// twice, is recovered once its window has passed, and an effect that cannot be
// settled waits for a person.
func TestConnectorEOSC(t *testing.T) {
	t.Run("a re-run right after the crash waits and then resends a keyed effect", func(t *testing.T) {
		eachStore(t, func(t *testing.T, env *storetest.EngineEnv) {
			t.Helper()
			r := newRig(t, env, keyedPost, limit.Spec{Capacity: 100, RefillPerSecond: 50})
			exec := r.crashInTheEffect()
			second := r.engine()

			// The lease is reclaimed long before the dispatch deadline (15 s against 30 s in production):
			// the token is worked again at once, with no waiting by the test.
			c, start, ok := r.claimOne(second, time.Second)
			if !ok {
				t.Fatal("the token was not claimable after the lease was given back")
			}
			r.work(second, c, start)
			if req, _ := r.mock.counts(); req != 1 {
				t.Fatalf("requests after the early re-run = %d; the effect must not be sent again before its window has passed", req)
			}
			if got := r.status(exec); got != "RUNNING" {
				t.Fatalf("execution = %s after the early re-run, want RUNNING: the token waits, it does not fail", got)
			}
			live := env.LiveTokens(exec)
			if len(live) != 1 || live[0].Status != "RUNNABLE" {
				t.Fatalf("live tokens = %+v, want the one token RUNNABLE until the recovery instant", live)
			}

			// Recovery runs once the deadline and the grace have passed, with the same key.
			c, start, ok = r.claimOne(second, 2*attemptTimeout+5*time.Second)
			if !ok {
				t.Fatal("the token never became claimable again")
			}
			r.work(second, c, start)
			if req, effects := r.mock.counts(); req != 2 || effects != 1 {
				t.Fatalf("requests = %d, effects = %d; the resend must carry the key and be deduplicated", req, effects)
			}
			if got := r.status(exec); got != "SUCCEEDED" {
				t.Fatalf("execution = %s, want SUCCEEDED", got)
			}
		})
	})

	t.Run("a re-run right after the crash waits and then suspends an unkeyed effect", func(t *testing.T) {
		eachStore(t, func(t *testing.T, env *storetest.EngineEnv) {
			t.Helper()
			r := newRig(t, env, plainPost, limit.Spec{Capacity: 100, RefillPerSecond: 50})
			exec := r.crashInTheEffect()
			second := r.engine()

			c, start, ok := r.claimOne(second, time.Second)
			if !ok {
				t.Fatal("the token was not claimable after the lease was given back")
			}
			r.work(second, c, start)
			if got := r.status(exec); got != "RUNNING" {
				t.Fatalf("execution = %s after the early re-run, want RUNNING", got)
			}

			c, start, ok = r.claimOne(second, 2*attemptTimeout+5*time.Second)
			if !ok {
				t.Fatal("the token never became claimable again")
			}
			r.work(second, c, start)
			if req, effects := r.mock.counts(); req != 1 || effects != 1 {
				t.Fatalf("requests = %d, effects = %d; an unkeyed effect is never repeated", req, effects)
			}
			assertSuspended(t, r, exec)
		})
	})

	t.Run("an effect with an unknown outcome suspends the execution", func(t *testing.T) {
		eachStore(t, func(t *testing.T, env *storetest.EngineEnv) {
			t.Helper()
			r := newRig(t, env, plainPost, limit.Spec{Capacity: 100, RefillPerSecond: 50})
			r.mock.set(nil, 503) // the provider cannot say whether it acted
			eng := r.engine()
			exec := env.Start(eng, r.pub, executiontest.Batch(`{"n":1}`))
			c, start, ok := r.claimOne(eng, time.Second)
			if !ok {
				t.Fatal("no token to claim")
			}
			r.work(eng, c, start)
			if req, _ := r.mock.counts(); req != 1 {
				t.Fatalf("requests = %d, want 1", req)
			}
			assertSuspended(t, r, exec)
			if _, _, ok := r.claimOne(eng, 200*time.Millisecond); ok {
				t.Fatal("a suspended token was claimed again: it waits for a person")
			}
		})
	})
}

// assertSuspended checks that the execution and its one live token wait for a person.
func assertSuspended(t *testing.T, r *rig, exec store.ExecutionID) {
	t.Helper()
	if got := r.status(exec); got != "SUSPENDED" {
		t.Fatalf("execution = %s, want SUSPENDED", got)
	}
	live := r.env.LiveTokens(exec)
	if len(live) != 1 || live[0].Status != "SUSPENDED" {
		t.Fatalf("live tokens = %+v, want the one token SUSPENDED", live)
	}
}

// TestBucketRefusalDelaysTheWork (BL-121, ADR-0014 section 2.4): a burst larger than the
// bucket is spread over time; no execution fails because the bucket refused it.
func TestBucketRefusalDelaysTheWork(t *testing.T) {
	cases := []struct{ name, params string }{
		{"read", plainGet},
		{"keyed effect", keyedPost},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eachStore(t, func(t *testing.T, env *storetest.EngineEnv) {
				t.Helper()
				const executions = 10
				r := newRig(t, env, tc.params, limit.Spec{Capacity: 2, RefillPerSecond: 8})
				eng := r.engine()
				ids := make([]store.ExecutionID, executions)
				for i := range ids {
					ids[i] = env.Start(eng, r.pub, executiontest.Batch(`{"n":1}`))
				}
				stop := time.Now().Add(30 * time.Second)
				for time.Now().Before(stop) {
					claimed, start, err := eng.Claim(t.Context(), 4)
					if err != nil {
						t.Fatalf("claim: %v", err)
					}
					for i := range claimed {
						r.work(eng, claimed[i], start)
					}
					done := 0
					for _, e := range ids {
						if r.status(e) == "SUCCEEDED" {
							done++
						}
					}
					if done == executions {
						break
					}
					if len(claimed) == 0 {
						time.Sleep(10 * time.Millisecond)
					}
				}
				for _, e := range ids {
					if got := r.status(e); got != "SUCCEEDED" {
						t.Fatalf("execution %s = %s, want SUCCEEDED: a refusal of the bucket delays the work, it does not fail it", e, got)
					}
				}
				if r.buckets.refused() == 0 {
					t.Fatal("the bucket never refused: the burst did not exceed its capacity")
				}
			})
		})
	}
}
