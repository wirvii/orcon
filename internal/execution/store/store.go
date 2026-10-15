// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package store declares the durable-state ports of the execution area
// (ADR-0015 section 1, architecture-layers.md AL-3, AL-6): the work queue with
// leases and fencing, the transaction in which an engine step commits, the
// journal of external operations, joins, timers, waits, the event log, the
// outbox and the attention inbox. The storage adapters implement them and
// internal/adapter/storetest proves both behave the same.
//
// The package holds rows and rules of storage only; the state machines of the
// engine (ADR-0009) decide which transitions to ask for. Status and kind
// values are the UPPER_SNAKE strings the database stores.
//
// It is a sub-package of the execution area so that it does not collide with
// the interpreter types of internal/execution.
package store

import (
	"context"
	"errors"
	"time"

	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// Errors of the ports. The errors of internal/flow (ErrNotFound, ErrConflict,
// ErrReference, ErrInvalidRecord, ErrClosed) apply here too.
var (
	// ErrFenced means the lease named by a [Fence] is no longer held: its epoch
	// moved, its owner changed or the token left LEASED. The transaction is
	// aborted and the worker drops the token (ADR-0013 section 4).
	ErrFenced = errors.New("lease lost: the token is no longer leased to this holder at this epoch")
)

// The row identities of the execution tables are distinct types. The adapters
// convert them to and from [id.UUID]; no other package does.
type (
	// ExecutionID identifies a row of execution.
	ExecutionID id.UUID
	// TokenID identifies a row of token.
	TokenID id.UUID
	// NodeExecutionID identifies a row of node_execution.
	NodeExecutionID id.UUID
	// ExternalOperationID identifies a row of external_operation.
	ExternalOperationID id.UUID
	// EventID identifies a row of execution_event.
	EventID id.UUID
	// JoinInstanceID identifies a row of join_instance.
	JoinInstanceID id.UUID
	// JoinArrivalID identifies a row of join_arrival.
	JoinArrivalID id.UUID
	// TimerID identifies a row of timer.
	TimerID id.UUID
	// WaitID identifies a row of wait.
	WaitID id.UUID
	// OutboxID identifies a row of outbox.
	OutboxID id.UUID
	// AttentionRequestID identifies a row of attention_request.
	AttentionRequestID id.UUID
)

// String returns the lowercase hyphenated form.
func (i ExecutionID) String() string { return id.UUID(i).String() }

// IsZero reports whether i is the nil identity.
func (i ExecutionID) IsZero() bool { return id.UUID(i).IsZero() }

// String returns the lowercase hyphenated form.
func (i TokenID) String() string { return id.UUID(i).String() }

// IsZero reports whether i is the nil identity.
func (i TokenID) IsZero() bool { return id.UUID(i).IsZero() }

// String returns the lowercase hyphenated form.
func (i NodeExecutionID) String() string { return id.UUID(i).String() }

// IsZero reports whether i is the nil identity.
func (i NodeExecutionID) IsZero() bool { return id.UUID(i).IsZero() }

// String returns the lowercase hyphenated form.
func (i ExternalOperationID) String() string { return id.UUID(i).String() }

// String returns the lowercase hyphenated form.
func (i EventID) String() string { return id.UUID(i).String() }

// String returns the lowercase hyphenated form.
func (i JoinInstanceID) String() string { return id.UUID(i).String() }

// String returns the lowercase hyphenated form.
func (i JoinArrivalID) String() string { return id.UUID(i).String() }

// String returns the lowercase hyphenated form.
func (i TimerID) String() string { return id.UUID(i).String() }

// String returns the lowercase hyphenated form.
func (i WaitID) String() string { return id.UUID(i).String() }

// IsZero reports whether i is the nil identity.
func (i WaitID) IsZero() bool { return id.UUID(i).IsZero() }

// String returns the lowercase hyphenated form.
func (i OutboxID) String() string { return id.UUID(i).String() }

// String returns the lowercase hyphenated form.
func (i AttentionRequestID) String() string { return id.UUID(i).String() }

// Execution is a row of execution. The documents of the row (error, summary,
// call chain) are read and written by the cuts that own them.
type Execution struct {
	ID    ExecutionID
	OrgID identity.OrgID
	// ProjectID and FlowID name the flow the execution runs.
	ProjectID identity.ProjectID
	FlowID    flow.FlowID
	// Exactly one of FlowVersionID and FlowSnapshotID is set.
	FlowVersionID  flow.FlowVersionID
	FlowSnapshotID flow.FlowSnapshotID
	Status         string
	Mode           string
	Priority       int64
	// ParentExecutionID is the zero value for a root execution; RootExecutionID
	// is the execution itself then.
	ParentExecutionID ExecutionID
	RootExecutionID   ExecutionID
	CallDepth         int64
	RunGeneration     int64
	// CancelRequestedAt is the zero time until cancellation is requested.
	CancelRequestedAt time.Time
	// The token counters ADR-0009 section 1 derives the status from.
	RunnableCount  int64
	LeasedCount    int64
	WaitingCount   int64
	SuspendedCount int64
	StartedAt      time.Time
	FinishedAt     time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Token is a row of token: the unit of work.
type Token struct {
	ID          TokenID
	OrgID       identity.OrgID
	ExecutionID ExecutionID
	Status      string
	NodeID      string
	// EdgeID is empty for the root token.
	EdgeID    string
	ScopePath string
	// ForkLineage is the document of orcon/fork-lineage@1.
	ForkLineage       []byte
	PayloadReference  string
	FairnessKey       string
	EffectivePriority int64
	AvailableAt       time.Time
	// LeaseOwner, LeaseEpoch and LeaseExpiresAt describe the lease while the
	// status is LEASED.
	LeaseOwner     execution.WorkerID
	LeaseEpoch     int64
	LeaseExpiresAt time.Time
	ClaimCount     int64
	AbandonedCount int64
	// SuspendReason is empty unless the token is SUSPENDED.
	SuspendReason string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Fence names a lease. Every write of a lease holder is fenced on
// (token, epoch, owner, status = LEASED) (ADR-0009 section 2).
type Fence struct {
	OrgID identity.OrgID
	Token TokenID
	Epoch int64
	Owner execution.WorkerID
}

// NodeExecution is a row of node_execution.
type NodeExecution struct {
	ID          NodeExecutionID
	OrgID       identity.OrgID
	ExecutionID ExecutionID
	TokenID     TokenID
	NodeID      string
	ScopePath   string
	Status      string
	StartedAt   time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Event is a row of execution_event. SequenceNumber is assigned by the store in
// the transaction that appends the event.
type Event struct {
	ID              EventID
	OrgID           identity.OrgID
	ExecutionID     ExecutionID
	RootExecutionID ExecutionID
	SequenceNumber  int64
	OccurredAt      time.Time
	// Type is the dotted event type, for example "token.forked".
	Type          string
	SchemaVersion int64
	// Reference is the document of orcon/event-reference@1; Data, when not nil,
	// the document of orcon/event-data@1.
	Reference []byte
	Data      []byte
	CreatedAt time.Time
}

// ExternalOperation is a row of external_operation.
type ExternalOperation struct {
	ID                 ExternalOperationID
	OrgID              identity.OrgID
	ExecutionID        ExecutionID
	NodeExecutionID    NodeExecutionID
	ItemIndex          int64
	CorrelationKey     string
	ContractReference  string
	State              string
	BatchKey           string
	DispatchDeadlineAt time.Time
	FirstDispatchedAt  time.Time
	RetryEpoch         int64
	ResponseReference  string
	CreatedAt          time.Time
}

// JoinKey identifies a join instance: a node in a scope, in one round.
type JoinKey struct {
	OrgID       identity.OrgID
	ExecutionID ExecutionID
	NodeID      string
	ScopePath   string
	RoundNumber int64
}

// JoinArrival records that a token arrived on an edge of a join.
type JoinArrival struct {
	Key JoinKey
	// InstanceID and ArrivalID are the deterministic identities of the rows
	// (ADR-0008 section 6); a repeat of the same arrival changes nothing.
	InstanceID JoinInstanceID
	ArrivalID  JoinArrivalID
	EdgeIndex  int64
	TokenID    TokenID
	// PayloadReference is empty when the arrival carries nothing.
	PayloadReference string
	IsLive           bool
	// SplitNodeExecutionID is the corresponding split instance of a LET_FINISH
	// round; the zero value otherwise.
	SplitNodeExecutionID NodeExecutionID
	ArrivedAt            time.Time
}

// JoinProgress is what a join knows after an arrival: how many distinct edges
// arrived, how many of them carried a live token, and whether this call
// recorded the arrival or found it already there.
type JoinProgress struct {
	Recorded bool
	Arrivals int64
	Live     int64
}

// JoinInstance is a row of join_instance.
type JoinInstance struct {
	ID                  JoinInstanceID
	Key                 JoinKey
	Status              string
	ContinuationTokenID TokenID
}

// Timer is a row of timer.
type Timer struct {
	ID          TimerID
	OrgID       identity.OrgID
	ExecutionID ExecutionID
	TokenID     TokenID
	Kind        string
	DueAt       time.Time
	Status      string
	CreatedAt   time.Time
}

// Wait is a row of wait.
type Wait struct {
	ID              WaitID
	OrgID           identity.OrgID
	ExecutionID     ExecutionID
	TokenID         TokenID
	NodeExecutionID NodeExecutionID
	Kind            string
	WaitGroupKey    string
	// SecretHash is the SHA-256 of the signal secret; nil for other kinds.
	SecretHash []byte
	ExpiresAt  time.Time
	CreatedAt  time.Time
}

// OutboxRow is a row of outbox.
type OutboxRow struct {
	ID        OutboxID
	OrgID     identity.OrgID
	Topic     string
	Payload   []byte
	CreatedAt time.Time
}

// AttentionRequest is a row of attention_request. NotifyClass and Status default
// to NOTIFY and OPEN when empty.
type AttentionRequest struct {
	ID             AttentionRequestID
	OrgID          identity.OrgID
	ExecutionID    ExecutionID
	ProjectID      identity.ProjectID
	FlowID         flow.FlowID
	NodeID         string
	TokenID        TokenID
	Cause          string
	CauseReference string
	NotifyClass    string
	Status         string
	Level          int64
	GroupKey       string
	ItemCount      int64
	Summary        string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Cursor positions a keyset page by creation time and identity. The zero
// Cursor starts at the first page.
type Cursor struct {
	CreatedAt time.Time
	ID        id.UUID
}

// IsZero reports whether c starts the first page.
func (c Cursor) IsZero() bool { return c.CreatedAt.IsZero() && c.ID.IsZero() }

// ExecutionFilter selects executions by exactly one of its fields.
type ExecutionFilter struct {
	ProjectID identity.ProjectID
	FlowID    flow.FlowID
	Status    string
}

// Reader reads inside one organization.
type Reader interface {
	// GetExecution returns an execution, or flow.ErrNotFound.
	GetExecution(ctx context.Context, org orgscope.Org, execution ExecutionID) (Execution, error)
	// ListExecutions returns at most limit executions matching the filter,
	// newest first, after the cursor. Exactly one filter field must be set.
	ListExecutions(ctx context.Context, org orgscope.Org, filter ExecutionFilter, after Cursor, limit int) ([]Execution, error)
	// GetToken returns a token, or flow.ErrNotFound.
	GetToken(ctx context.Context, org orgscope.Org, token TokenID) (Token, error)
	// ListExecutionEvents returns at most limit events of the execution with a
	// sequence number greater than after, in order.
	ListExecutionEvents(ctx context.Context, org orgscope.Org, execution ExecutionID, after int64, limit int) ([]Event, error)
	// GetExternalOperation returns one journal entry, or flow.ErrNotFound.
	GetExternalOperation(ctx context.Context, org orgscope.Org, operation ExternalOperationID) (ExternalOperation, error)
	// ListExternalOperations returns the journal of a node execution from an
	// item index on, in item order.
	ListExternalOperations(ctx context.Context, org orgscope.Org, node NodeExecutionID, fromItem int64, limit int) ([]ExternalOperation, error)
}

// Inbox reads the rows people and the editor look at.
type Inbox interface {
	// ListAttentionRequests returns at most limit requests in one status,
	// oldest first, after the cursor (the attention inbox, ADR-0073 section 9).
	ListAttentionRequests(ctx context.Context, org orgscope.Org, status string, after Cursor, limit int) ([]AttentionRequest, error)
	// GetTimer returns a timer, or flow.ErrNotFound.
	GetTimer(ctx context.Context, org orgscope.Org, timer TimerID) (Timer, error)
	// GetJoinInstance returns a join instance, or flow.ErrNotFound. The engine
	// reads the join of a token it has claimed; the key names the organization of
	// that join, so the lookup is not a read of one organization by a person.
	//
	//orcon:scope engine the engine resumes a join of the organization its claimed token names (ADR-0047 section 2)
	GetJoinInstance(ctx context.Context, key JoinKey) (JoinInstance, error)
}

// Tx is the writes of one transaction in one organization. Reads of the
// transaction go through [Reader]; the engine fences a step by calling
// [Tx.AssertFence] first.
//
//nolint:interfacebloat // the operations of one engine step commit together; splitting them would split the transaction
type Tx interface {
	// AssertFence fails with [ErrFenced] unless the lease is still held.
	AssertFence(ctx context.Context, f Fence, now time.Time) error
	// MoveToken moves a leased token to another status, clearing its lease, when
	// the fence holds; otherwise [ErrFenced]. The status must not be LEASED.
	MoveToken(ctx context.Context, f Fence, to string, now time.Time) error
	InsertExecution(ctx context.Context, e *Execution) error
	InsertToken(ctx context.Context, t *Token) error
	InsertNodeExecution(ctx context.Context, n NodeExecution) error
	// AppendEvent assigns the next sequence number of the execution to the event
	// and returns it. Concurrent transactions append in commit order.
	AppendEvent(ctx context.Context, e Event) (int64, error)
	// AppendJournal inserts the operations as PREPARED in one statement group
	// (intent before dispatch, ADR-0012 section 4). A repeated correlation key
	// fails with flow.ErrConflict and inserts none.
	AppendJournal(ctx context.Context, operations []ExternalOperation) error
	// MarkDispatching moves PREPARED operations to DISPATCHING with their
	// deadline and returns how many moved (ADR-0012 section 4, step 2).
	MarkDispatching(ctx context.Context, operations []ExternalOperationID, deadline, now time.Time) (int64, error)
	// ResolveOperation moves an operation from one state to another and reports
	// whether it was in the expected state.
	ResolveOperation(ctx context.Context, operation ExternalOperationID, from, to, responseReference string, now time.Time) (bool, error)
	// RecordJoinArrival records an arrival and reports the progress of the join
	// atomically (ADR-0010 section 3: mark me and tell me whether I am last).
	RecordJoinArrival(ctx context.Context, a JoinArrival, now time.Time) (JoinProgress, error)
	// MarkJoinFired moves an OPEN join to FIRED with its continuation token and
	// reports whether this call fired it: exactly one caller wins.
	MarkJoinFired(ctx context.Context, key JoinKey, continuation TokenID, now time.Time) (bool, error)
	InsertTimer(ctx context.Context, t Timer) error
	InsertWait(ctx context.Context, w Wait) error
	InsertOutbox(ctx context.Context, o OutboxRow) error
	// MarkOutboxDelivered records delivery by the holder of the lease.
	MarkOutboxDelivered(ctx context.Context, outbox OutboxID, holder execution.WorkerID, deliveredAt time.Time) (bool, error)
	InsertAttentionRequest(ctx context.Context, a *AttentionRequest) error
}

// Transactor runs a function in one transaction of one organization. The
// function may be run again after a serialization or busy failure, so it must
// be safe to repeat; it holds no external call (AL-6.3).
type Transactor interface {
	InTx(ctx context.Context, org orgscope.Org, fn func(ctx context.Context, tx Tx) error) error
}

// ClaimRequest asks for runnable tokens across organizations.
type ClaimRequest struct {
	Worker execution.WorkerID
	// FairnessKeys limits the claim to these keys; empty means any.
	FairnessKeys []string
	Limit        int
	// LeaseTTL is how long the lease lasts, counted by the database clock on
	// PostgreSQL and by Now on SQLite (one process).
	LeaseTTL time.Duration
	Now      time.Time
}

// Claimed is a leased token with the epoch the worker must present.
type Claimed struct {
	OrgID            identity.OrgID
	TokenID          TokenID
	ExecutionID      ExecutionID
	NodeID           string
	EdgeID           string
	ScopePath        string
	ForkLineage      []byte
	PayloadReference string
	FairnessKey      string
	LeaseEpoch       int64
	LeaseExpiresAt   time.Time
}

// Renewed is a lease that was extended, with the cancellation request of its
// execution (ADR-0013 section 4).
type Renewed struct {
	OrgID           identity.OrgID
	TokenID         TokenID
	LeaseEpoch      int64
	CancelRequested bool
}

// Reaped is a token whose lease expired.
type Reaped struct {
	OrgID          identity.OrgID
	TokenID        TokenID
	ExecutionID    ExecutionID
	AbandonedCount int64
	// Poisoned is true when the token was suspended with reason POISON.
	Poisoned bool
}

// TokenRef names a token across organizations.
type TokenRef struct {
	OrgID   identity.OrgID
	TokenID TokenID
}

// FiredTimer is a timer the scheduler fired.
type FiredTimer struct {
	OrgID       identity.OrgID
	TimerID     TimerID
	ExecutionID ExecutionID
	TokenID     TokenID
	Kind        string
	DueAt       time.Time
}

// ClaimedOutbox is an outbox row leased to a dispatcher.
type ClaimedOutbox struct {
	OrgID        identity.OrgID
	OutboxID     OutboxID
	Topic        string
	Payload      []byte
	AttemptCount int64
}

// WaitLookup is the answer of the lookup that runs before the organization is
// known.
type WaitLookup struct {
	OrgID      identity.OrgID
	WaitID     WaitID
	SecretHash []byte
	Status     string
	ExpiresAt  time.Time
}

// WorkQueue is the work that crosses organizations: claiming, renewing and
// reaping leases, and firing timers (ADR-0013, ADR-0047). On PostgreSQL each
// method calls an orcon_engine function; on SQLite the single writer does the
// same in one transaction.
type WorkQueue interface {
	// ClaimTokens leases up to Limit runnable tokens in fairness order. A token
	// is claimed by one worker only, however many claim at once.
	//
	//orcon:scope engine the engine claims work of every organization (ADR-0047 section 2)
	ClaimTokens(ctx context.Context, r ClaimRequest) ([]Claimed, error)
	// RenewLeases extends every lease of the worker that has not expired. A
	// lease the worker held and that is not returned was lost.
	//
	//orcon:scope engine the engine renews the leases of its worker across organizations (ADR-0047 section 2)
	RenewLeases(ctx context.Context, worker execution.WorkerID, ttl time.Duration, now time.Time) ([]Renewed, error)
	// ReleaseExpiredLeases returns up to limit expired leases to RUNNABLE, or to
	// SUSPENDED with reason POISON once a token was abandoned poisonThreshold
	// times, and marks the started attempts of their node executions ABANDONED.
	//
	//orcon:scope engine the reaper sweeps expired leases of every organization (ADR-0047 section 2)
	ReleaseExpiredLeases(ctx context.Context, limit int, poisonThreshold int64, now time.Time) ([]Reaped, error)
	// ReleaseWorkerLeases gives up every lease of the worker (graceful shutdown).
	//
	//orcon:scope engine the leases of one worker can name tokens of several organizations (ADR-0047 section 2)
	ReleaseWorkerLeases(ctx context.Context, worker execution.WorkerID, now time.Time) ([]TokenRef, error)
	// FireDueTimers marks up to limit due timers FIRED; concurrent callers never
	// fire the same timer.
	//
	//orcon:scope engine the timer scan covers the due timers of every organization (ADR-0047 section 2)
	FireDueTimers(ctx context.Context, limit int, now time.Time) ([]FiredTimer, error)
}

// Deliveries leases outbox rows to dispatchers (ADR-0013 section 8).
type Deliveries interface {
	// ClaimOutbox leases up to limit undelivered rows whose lease is free or
	// expired and counts the attempt.
	//
	//orcon:scope engine the dispatchers lease outbox rows of every organization (ADR-0047 section 2)
	ClaimOutbox(ctx context.Context, worker execution.WorkerID, limit int, ttl time.Duration, now time.Time) ([]ClaimedOutbox, error)
}

// WaitResolver finds a signal wait before its organization is known
// (ADR-0010 section 8).
type WaitResolver interface {
	// ResolveWaitSecret returns the wait and its organization, or
	// flow.ErrNotFound. The caller compares SecretHash in constant time.
	//
	//orcon:scope engine the organization of a wait is unknown until the wait is found (ADR-0010 section 8)
	ResolveWaitSecret(ctx context.Context, wait WaitID) (WaitLookup, error)
}
