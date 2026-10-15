// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package durable

import (
	"context"
	"log/slog"
	"time"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// Store is what the engine needs from storage: the reads of an execution, the
// transaction of a step, the work that crosses organizations, the outbox and the
// registry of workers. The adapters of SQLite and PostgreSQL implement it.
type Store interface { //nolint:interfacebloat // the engine works through every port it needs of the store (ADR-0013)
	store.Scopes
	store.Reader
	store.EngineReader
	store.Transactor
	store.Work
	store.Deliveries
	execution.WorkerStore
}

// Revisions reads the pinned flow revisions an execution runs (ADR-0007
// section 7: a running execution keeps the revision it started with).
type Revisions interface {
	GetVersion(ctx context.Context, org orgscope.Org, version flow.FlowVersionID) (flow.VersionRecord, error)
	GetSnapshot(ctx context.Context, org orgscope.Org, snapshot flow.FlowSnapshotID) (flow.SnapshotRecord, error)
}

// DocumentReader turns the stored bytes of a revision into the document of the
// format this build runs: it parses them and upgrades older formats through the
// chain of pure functions of ADR-0007 section 7. The stored bytes never change.
type DocumentReader interface {
	// Read parses and upgrades a stored document.
	Read(data []byte) (*flow.Document, error)
	// Version identifies the format the reader produces; the cache of upgraded
	// documents is keyed by it next to the content hash.
	Version() int
}

// Delivery is an outbox row handed to a [Deliverer].
type Delivery struct {
	OrgID   identity.OrgID
	ID      store.OutboxID
	Topic   string
	Payload []byte
	// Attempt counts the claims of the row, this one included. The id is the
	// deduplication key of the receiver: delivery is at least once (ADR-0013
	// section 8).
	Attempt int64
}

// DefaultConcurrency is the number of tokens a process works at the same time
// when the configuration does not say.
const DefaultConcurrency = 4

// Deliverer sends an outbox row out of the database.
type Deliverer interface {
	Deliver(ctx context.Context, d Delivery) error
}

// OutboxRule decides which events leave the database; see [store.OutboxRule].
type OutboxRule = store.OutboxRule

// Point names a moment of a step at which a fault test can act.
type Point string

// The points of a step. They exist for the fault tests of the engine: a test
// process stops or kills itself at the moment it names.
const (
	// PointClaimed is right after a token was claimed.
	PointClaimed Point = "claimed"
	// PointWorked is after the interpreter ran the token and before the commit.
	PointWorked Point = "worked"
	// PointInCommit is inside the commit transaction, after the writes and before
	// the transaction ends.
	PointInCommit Point = "in-commit"
	// PointCommitted is right after the commit.
	PointCommitted Point = "committed"
	// PointForked is after the commit of a segment that ended at a fork.
	PointForked Point = "forked"
)

// Faults is the hook of the fault tests; production code leaves it nil.
type Faults interface {
	// Hit is called at each [Point] with the token being worked. A test that
	// wants the process to die kills it from inside; one that wants the step to
	// fail returns an error, which fails the step as if the call at that point had.
	Hit(ctx context.Context, p Point, token store.TokenID) error
}

// Config sets up an [Engine]. Zero values take the defaults of ADR-0013.
type Config struct {
	// Worker identifies this process in the registry and in the leases it holds.
	Worker execution.WorkerID
	// Role is one of the execution.Worker* constants. Host and Version describe
	// the process in the registry.
	Role, Host, Version string

	// LeaseTTL is how long a lease lasts without renewal (15 s).
	LeaseTTL time.Duration
	// RenewEvery is the period of lease renewal (5 s).
	RenewEvery time.Duration
	// PollEvery is the period at which an idle worker looks for work (1 s,
	// spread by 20 % jitter). Polling is the guarantee; a wake-up after a
	// commit in the same process only makes it sooner.
	PollEvery time.Duration
	// DisableWake turns the in-process wake-up off, so that every claim is a
	// poll.
	DisableWake bool
	// ReapEvery is the period of the reaper and the timer scheduler (1 s).
	ReapEvery time.Duration
	// DispatchEvery is the period at which the outbox is dispatched (1 s).
	DispatchEvery time.Duration
	// ShutdownGrace bounds the time a graceful shutdown waits for the held
	// tokens to checkpoint and release (30 s).
	ShutdownGrace time.Duration
	// ExecutionLimit is the most tokens of one execution that may be leased at
	// once (ADR-0014 section 2.2); store.DefaultExecutionLimit when zero.
	ExecutionLimit int
	// Concurrency is the number of tokens the process works at the same time (4).
	Concurrency int
	// PoisonThreshold is the number of abandonments after which a token is
	// suspended (3, at least 2: with 1 every token would have to run alone).
	PoisonThreshold int64
	// IsolationWait is how long a token one abandonment short of the threshold may
	// wait for a worker with nothing else to do before the busy workers stop
	// claiming and empty themselves for it (10 s).
	IsolationWait time.Duration
	// CheckpointActivations bounds the node activations between two commits
	// (64), and CheckpointInterval the time (2 s) (ADR-0013 section 6).
	CheckpointActivations int
	CheckpointInterval    time.Duration
	// Roles selects what Run does: workers, the reaper and the timer scheduler,
	// the outbox dispatcher. All are on when none is named.
	Workers, Reaper, Dispatcher bool

	Clock  clock.Clock
	IDs    id.Generator
	Logger *slog.Logger

	// Live receives every event after the commit that persisted it, including
	// those that are streamed and not stored (an edge that closes where no join
	// waits). It is at most once.
	Live execution.EventSink
	// Outbox decides which events are published through the outbox; nil publishes none.
	Outbox OutboxRule
	// Deliver sends outbox rows; nil leaves them in the table.
	Deliver Deliverer
	// Faults is the hook of the fault tests.
	Faults Faults
	// Committed observes every delta after its commit, for tests and metrics.
	Committed func(d *execution.Delta, replayed bool)
}

func (c *Config) defaults() {
	if c.LeaseTTL <= 0 {
		c.LeaseTTL = 15 * time.Second
	}
	if c.RenewEvery <= 0 {
		c.RenewEvery = 5 * time.Second
	}
	if c.PollEvery <= 0 {
		c.PollEvery = time.Second
	}
	if c.ReapEvery <= 0 {
		c.ReapEvery = time.Second
	}
	if c.DispatchEvery <= 0 {
		c.DispatchEvery = time.Second
	}
	if c.ShutdownGrace <= 0 {
		c.ShutdownGrace = 30 * time.Second
	}
	if c.Concurrency <= 0 {
		c.Concurrency = DefaultConcurrency
	}
	if c.PoisonThreshold == 0 {
		c.PoisonThreshold = store.DefaultPoisonThreshold
	}
	if c.IsolationWait <= 0 {
		c.IsolationWait = 10 * time.Second
	}
	if c.CheckpointActivations <= 0 {
		c.CheckpointActivations = 64
	}
	if c.CheckpointInterval <= 0 {
		c.CheckpointInterval = 2 * time.Second
	}
	if !c.Workers && !c.Reaper && !c.Dispatcher {
		c.Workers, c.Reaper, c.Dispatcher = true, true, true
	}
	if c.Role == "" {
		c.Role = execution.WorkerStandalone
	}
	if c.Host == "" {
		c.Host = "unknown"
	}
	if c.Version == "" {
		c.Version = "unknown"
	}
	if c.Clock == nil {
		c.Clock = clock.System{}
	}
	if c.IDs == nil {
		c.IDs = id.NewV7Generator(c.Clock.Now)
	}
	if c.Logger == nil {
		c.Logger = slog.New(slog.DiscardHandler)
	}
}
