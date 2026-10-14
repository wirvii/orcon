// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/postgres"
	"github.com/wirvii/orcon/internal/adapter/sqlite"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/durable"
	"github.com/wirvii/orcon/internal/execution/executiontest"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Kind names a store implementation the engine tests run against.
type Kind string

// The stores.
const (
	// SQLite is the standalone store.
	SQLite Kind = "sqlite"
	// PostgreSQL is the distributed store.
	PostgreSQL Kind = "postgres"
)

// KindsFor turns the value of the -store flag of the engine tests into the
// stores to run against: sqlite, postgres, both, or auto (SQLite, and
// PostgreSQL when a server is enabled). It reports an error for any other value.
func KindsFor(flagValue string) ([]Kind, error) {
	switch flagValue {
	case "sqlite":
		return []Kind{SQLite}, nil
	case "postgres":
		return []Kind{PostgreSQL}, nil
	case "both":
		return []Kind{SQLite, PostgreSQL}, nil
	case "auto":
		if PostgresEnabled() {
			return []Kind{SQLite, PostgreSQL}, nil
		}
		return []Kind{SQLite}, nil
	default:
		return nil, fmt.Errorf("unknown store %q: want sqlite, postgres, both or auto", flagValue)
	}
}

// EngineOptions tune the stores the engine tests open.
type EngineOptions struct {
	// Clock supplies the instants the store writes; the system clock when nil.
	Clock clock.Clock
	// Weak turns durability down (SQLite synchronous=NORMAL) for tests that make
	// thousands of commits and do not test power loss.
	Weak bool
	// PoisonThreshold is the abandonments after which a token is poison; 3 when zero.
	PoisonThreshold int64
}

// OpenStore opens a store of the kind on a fresh, migrated database.
//
//nolint:ireturn // the engine tests run the same code over either adapter through the one interface of the suite
func OpenStore(tb testing.TB, kind Kind, opts EngineOptions) Store {
	tb.Helper()
	switch kind {
	case SQLite:
		return OpenSQLiteFile(tb, filepath.Join(tb.TempDir(), "orcon.db"), opts)
	case PostgreSQL:
		return OpenPostgresDSN(tb, NewPostgresDatabase(tb), opts)
	default:
		tb.Fatalf("unknown store kind %q", kind)
		return nil
	}
}

// OpenSQLiteFile opens (and migrates) the database file at path.
func OpenSQLiteFile(tb testing.TB, path string, opts EngineOptions) *sqlite.Store {
	tb.Helper()
	o := sqlite.Options{Path: path, BinaryVersion: "1.0.0", Clock: opts.Clock, PoisonThreshold: opts.PoisonThreshold}
	if opts.Weak {
		o.Synchronous = sqlite.SynchronousNormal
	}
	s, err := sqlite.Open(tb.Context(), o)
	if err != nil {
		tb.Fatalf("open sqlite store: %v", err)
	}
	tb.Cleanup(func() {
		if err := s.Close(); err != nil {
			tb.Errorf("close sqlite store: %v", err)
		}
	})
	return s
}

// OpenPostgresDSN opens (and migrates) the PostgreSQL database at dsn.
func OpenPostgresDSN(tb testing.TB, dsn string, opts EngineOptions) *postgres.Store {
	tb.Helper()
	s, err := postgres.Open(tb.Context(), postgres.Options{DSN: dsn, BinaryVersion: "1.0.0", Clock: opts.Clock})
	if err != nil {
		tb.Fatalf("open postgres store: %v", err)
	}
	tb.Cleanup(func() {
		if err := s.Close(); err != nil {
			tb.Errorf("close postgres store: %v", err)
		}
	})
	return s
}

// FlowReader reads stored flow documents: it parses them and upgrades older
// formats (flow.Read), as the codec of the flow format does.
type FlowReader struct{}

// Read implements [durable.DocumentReader].
func (FlowReader) Read(data []byte) (*flow.Document, error) {
	v, err := jsonvalue.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("read flow document: %w", err)
	}
	doc, err := flow.Read(v)
	if err != nil {
		return nil, fmt.Errorf("read flow document: %w", err)
	}
	return &doc, nil
}

// Version implements [durable.DocumentReader].
func (FlowReader) Version() int { return flow.FormatMajor }

// EngineEnv is a store with an organization and a project, and what is needed
// to publish flows and start executions on it.
type EngineEnv struct {
	T       testing.TB
	Kind    Kind
	Store   Store
	IDs     id.Generator
	Clock   clock.Clock
	Org     identity.OrgID
	Project identity.ProjectID

	mu       sync.Mutex
	versions int
}

// NewEngineEnv opens a store of the kind and creates the organization and
// project the engine tests use.
func NewEngineEnv(tb testing.TB, kind Kind, opts EngineOptions) *EngineEnv {
	tb.Helper()
	env := &EngineEnv{T: tb, Kind: kind, IDs: id.NewV7Generator(time.Now), Clock: opts.Clock}
	if env.Clock == nil {
		env.Clock = clock.System{}
	}
	env.Store = OpenStore(tb, kind, opts)
	env.Setup()
	return env
}

// NewEngineEnvOn builds the environment on a store that is already open, for
// tests that reopen a database file.
func NewEngineEnvOn(tb testing.TB, kind Kind, s Store, opts EngineOptions) *EngineEnv {
	tb.Helper()
	env := &EngineEnv{T: tb, Kind: kind, Store: s, IDs: id.NewV7Generator(time.Now), Clock: opts.Clock}
	if env.Clock == nil {
		env.Clock = clock.System{}
	}
	return env
}

// Setup creates the organization and the project.
func (env *EngineEnv) Setup() {
	env.T.Helper()
	ctx := context.Background()
	now := env.Clock.Now()
	slug := "o" + strings.ToLower(hex.EncodeToString(env.IDs.New().Bytes()[10:]))
	o := flow.OrgRecord{ID: identity.OrgID(env.IDs.New()), Name: slug, Slug: slug, Status: flow.OrgActive, CreatedAt: now, UpdatedAt: now}
	if err := env.Store.CreateOrganization(ctx, o); err != nil {
		env.T.Fatalf("create organization: %v", err)
	}
	p := flow.ProjectRecord{ID: identity.ProjectID(env.IDs.New()), OrgID: o.ID, Name: "engine", Slug: "engine", Environment: "development", IsProtected: true, CreatedAt: now, UpdatedAt: now}
	if err := env.Store.CreateProject(ctx, p); err != nil {
		env.T.Fatalf("create project: %v", err)
	}
	env.Org, env.Project = o.ID, p.ID
}

// Published is a flow with one published version.
type Published struct {
	Flow    flow.FlowID
	Version flow.FlowVersionID
}

// Publish stores the document as a new flow with one published version.
func (env *EngineEnv) Publish(doc *flow.Document) *Published {
	env.T.Helper()
	ctx := context.Background()
	data, err := jsonvalue.Marshal(doc.Value())
	if err != nil {
		env.T.Fatalf("encode flow: %v", err)
	}
	sum := sha256.Sum256(data)
	env.mu.Lock()
	env.versions++
	name := fmt.Sprintf("flow-%d", env.versions)
	env.mu.Unlock()
	now := env.Clock.Now()
	rec := flow.Record{ID: flow.FlowID(env.IDs.New()), OrgID: env.Org, ProjectID: env.Project, Name: name, Draft: data, CreatedAt: now, UpdatedAt: now}
	if err := env.Store.CreateFlow(ctx, rec); err != nil {
		env.T.Fatalf("create flow: %v", err)
	}
	ver := flow.VersionRecord{
		ID: flow.FlowVersionID(env.IDs.New()), OrgID: env.Org, FlowID: rec.ID, Number: 1, Document: data,
		ContentHash: hex.EncodeToString(sum[:]), PublishedAt: now, CreatedAt: now,
	}
	if err := env.Store.PublishVersion(ctx, ver); err != nil {
		env.T.Fatalf("publish version: %v", err)
	}
	return &Published{Flow: rec.ID, Version: ver.ID}
}

// Engine builds a durable engine over the environment's store.
//
//nolint:gocritic // hugeParam: the tests write the configuration as a literal at the call
func (env *EngineEnv) Engine(cfg durable.Config, deps execution.Deps) *durable.Engine {
	env.T.Helper()
	return env.EngineOn(env.Store, cfg, deps)
}

// EngineOn builds a durable engine over a store that wraps or replaces the one
// of the environment (a test that makes it fail).
//
//nolint:gocritic // hugeParam: the tests write the configuration as a literal at the call
func (env *EngineEnv) EngineOn(st durable.Store, cfg durable.Config, deps execution.Deps) *durable.Engine {
	env.T.Helper()
	if cfg.Worker.IsZero() {
		cfg.Worker = execution.WorkerID(env.IDs.New())
	}
	if cfg.Clock == nil {
		cfg.Clock = env.Clock
	}
	eng, err := durable.New(&cfg, st, env.Store, FlowReader{}, deps, durable.PlanLimits{})
	if err != nil {
		env.T.Fatalf("new engine: %v", err)
	}
	return eng
}

// Start starts an execution of a published flow.
func (env *EngineEnv) Start(eng *durable.Engine, p *Published, input execution.Batch) store.ExecutionID {
	env.T.Helper()
	return env.StartAs(eng, p, input, store.ExecutionID{})
}

// StartAs starts an execution with the identity given (a fresh one if it is the
// zero value). Two executions with the same identity, in two databases, derive
// the same ids for their rows, which is what a test compares.
func (env *EngineEnv) StartAs(eng *durable.Engine, p *Published, input execution.Batch, execID store.ExecutionID) store.ExecutionID {
	env.T.Helper()
	execID, err := eng.Start(env.T.Context(), durable.StartRequest{
		OrgID: env.Org, ProjectID: env.Project, FlowID: p.Flow, FlowVersionID: p.Version, Input: input, ID: execID,
	})
	if err != nil {
		env.T.Fatalf("start execution: %v", err)
	}
	return execID
}

// Drain claims and works tokens one after the other until none is left to
// claim, and returns how many it worked.
func (env *EngineEnv) Drain(eng *durable.Engine) int {
	env.T.Helper()
	n := 0
	for {
		claimed, start, err := eng.Claim(env.T.Context(), 1)
		if err != nil {
			env.T.Fatalf("claim: %v", err)
		}
		if len(claimed) == 0 {
			return n
		}
		if err := eng.Work(env.T.Context(), claimed[0], start); err != nil {
			env.T.Fatalf("work: %v", err)
		}
		n++
	}
}

// Execution reads an execution.
func (env *EngineEnv) Execution(execID store.ExecutionID) store.Execution {
	env.T.Helper()
	ex, err := env.Store.GetExecution(env.T.Context(), env.Org, execID)
	if err != nil {
		env.T.Fatalf("get execution: %v", err)
	}
	return ex
}

// LiveTokens reads the tokens of the execution that are not terminal.
func (env *EngineEnv) LiveTokens(execID store.ExecutionID) []store.Token {
	env.T.Helper()
	snap, err := env.Store.LoadSnapshot(env.T.Context(), env.Org, execID)
	if err != nil {
		env.T.Fatalf("load snapshot: %v", err)
	}
	return snap.Tokens
}

// Events reads every stored event of the execution.
func (env *EngineEnv) Events(execID store.ExecutionID) []store.Event {
	env.T.Helper()
	var out []store.Event
	var after int64
	for {
		page, err := env.Store.ListExecutionEvents(env.T.Context(), env.Org, execID, after, 500)
		if err != nil {
			env.T.Fatalf("list events: %v", err)
		}
		out = append(out, page...)
		if len(page) < 500 {
			return out
		}
		after = page[len(page)-1].SequenceNumber
	}
}

// ExecutionEngine returns an [executiontest.Engine] that runs flows through the
// durable engine over a store of the kind: each flow becomes a published
// version, the execution is started and worked by one worker until nothing is
// left to claim, and the result is assembled from the deltas the engine
// committed, then checked against the rows it left in the store. Every run opens
// a database of its own.
func ExecutionEngine(kind Kind, opts EngineOptions) executiontest.Engine {
	return func(tb testing.TB, doc *flow.Document, _ *execution.Plan, deps execution.Deps, input execution.Batch) execution.Result {
		tb.Helper()
		return NewEngineEnv(tb, kind, opts).RunFlow(doc, deps, input)
	}
}

// RunFlow runs a flow to its end with one worker and returns the result in the
// shape of the in-memory interpreter. The rows the engine left in the store are
// checked against that result: the status of the execution, the state of every
// token and the events that are stored.
func (env *EngineEnv) RunFlow(doc *flow.Document, deps execution.Deps, input execution.Batch) execution.Result {
	env.T.Helper()
	plan, err := execution.Compile(doc)
	if err != nil {
		env.T.Fatalf("compile: %v", err)
	}
	pub := env.Publish(doc)
	var col collector
	cfg := durable.Config{Concurrency: 1, Live: deps.Events, Committed: col.committed}
	deps.Events = nil
	eng := env.Engine(cfg, deps)
	execID := env.Start(eng, pub, input)
	col.exec = executionText(execID)
	env.Drain(eng)
	col.claimed(env.Events(execID))
	res := col.result()
	env.verify(execID, plan, &res)
	return res
}

func executionText(e store.ExecutionID) execution.ID { return execution.ID(e.String()) }

// collector assembles the result of an execution from the deltas of its commits.
type collector struct {
	mu      sync.Mutex
	exec    execution.ID
	status  execution.State
	failure *execution.Failure
	events  []execution.Event
	nodes   []execution.NodeExecutionRecord
	ends    []execution.EndArrival
	order   []execution.TokenID
	tokens  map[execution.TokenID]execution.TokenRow
}

func (c *collector) committed(d *execution.Delta, _ bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tokens == nil {
		c.tokens = make(map[execution.TokenID]execution.TokenRow)
	}
	c.status = d.State
	if d.Failure != nil {
		c.failure = d.Failure
	}
	for i := range d.Events {
		e := d.Events[i]
		e.Sequence = len(c.events) + 1
		c.events = append(c.events, e)
	}
	c.nodes = append(c.nodes, d.NodeExecutions...)
	c.ends = append(c.ends, d.Ends...)
	for i := range d.Tokens {
		row := &d.Tokens[i]
		c.order = append(c.order, row.ID)
		c.tokens[row.ID] = *row
	}
	for i := range d.Changed {
		c.tokens[d.Changed[i].Row.ID] = d.Changed[i].Row
	}
}

// claimed adds the event the claim of the first token stores when it starts the
// execution: it is part of the history of the run but of no commit.
func (c *collector) claimed(stored []store.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range stored {
		if stored[i].Type != string(execution.EventExecutionStarted) {
			continue
		}
		for j := range c.events {
			if c.events[j].Type == execution.EventExecutionStarted {
				return
			}
		}
		c.events = append([]execution.Event{{Type: execution.EventExecutionStarted}}, c.events...)
		for j := range c.events {
			c.events[j].Sequence = j + 1
		}
		return
	}
}

func (c *collector) result() execution.Result {
	c.mu.Lock()
	defer c.mu.Unlock()
	res := execution.Result{
		Execution: c.exec, Status: c.status, Failure: c.failure, Events: slices.Clone(c.events),
		NodeExecutions: slices.Clone(c.nodes), Ends: slices.Clone(c.ends),
	}
	for _, id := range c.order {
		row := c.tokens[id]
		res.Tokens = append(res.Tokens, execution.TokenRecord{
			ID: row.ID, State: row.State, Node: row.Node, Scope: row.Scope, Lineage: slices.Clone(row.Lineage), Payload: row.Payload,
		})
	}
	return res
}

// verify compares the rows in the store with the result assembled from the commits.
func (env *EngineEnv) verify(execID store.ExecutionID, plan *execution.Plan, res *execution.Result) {
	env.T.Helper()
	ex := env.Execution(execID)
	if ex.Status != string(res.Status) {
		env.T.Errorf("stored status = %s, the commits ended it %s", ex.Status, res.Status)
	}
	if live := env.LiveTokens(execID); len(live) > 0 && (res.Status == execution.ExecSucceeded || res.Status == execution.ExecFailed) {
		env.T.Errorf("%d live tokens after the execution ended %s", len(live), res.Status)
	}
	for _, rec := range res.Tokens {
		tid, err := id.Parse(string(rec.ID))
		if err != nil {
			env.T.Errorf("token id %q: %v", rec.ID, err)
			continue
		}
		got, err := env.Store.GetToken(env.T.Context(), env.Org, store.TokenID(tid))
		if err != nil {
			env.T.Errorf("get token %s: %v", rec.ID, err)
			continue
		}
		if got.Status != string(rec.State) || got.NodeID != string(rec.Node) || got.ScopePath != string(rec.Scope) {
			env.T.Errorf("stored token %s = %s at %s in %q, the commits left it %s at %s in %q",
				rec.ID, got.Status, got.NodeID, got.ScopePath, rec.State, rec.Node, rec.Scope)
		}
	}
	var want []string
	for i := range res.Events {
		e := &res.Events[i]
		if e.Type == execution.EventEdgeClosed && !plan.IsJoin(e.Node) {
			continue
		}
		want = append(want, string(e.Type))
	}
	var got []string
	stored := env.Events(execID)
	for i := range stored {
		if stored[i].Type != "execution.created" {
			got = append(got, stored[i].Type)
		}
	}
	if !slices.Equal(want, got) {
		env.T.Errorf("stored events differ from the persisted ones of the commits:\nwant %v\n got %v", want, got)
	}
}
