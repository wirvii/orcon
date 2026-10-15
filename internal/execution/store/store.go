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
	// PayloadID identifies a stored batch: a row of node_output or token_input.
	// It is the deterministic payload reference of the engine (ADR-0008 section 6).
	PayloadID id.UUID
	// NondeterminismID identifies a row of nondeterminism_record.
	NondeterminismID id.UUID
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
	// AvailableAt is when the token becomes claimable. When it is the zero
	// value, a token the engine creates becomes claimable at the database
	// clock plus QueueOrder microseconds, so that tokens created together are
	// claimed in the order they were created and a worker clock that is off
	// cannot hide them (orcon-db TY-5).
	AvailableAt time.Time
	QueueOrder  int64
	// LeaseOwner, LeaseEpoch and LeaseExpiresAt describe the lease while the
	// status is LEASED. When LeaseExpiresAt is the zero value and LeaseTTL is
	// set, the lease expires LeaseTTL after the database clock.
	LeaseOwner     execution.WorkerID
	LeaseEpoch     int64
	LeaseExpiresAt time.Time
	LeaseTTL       time.Duration
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
	// FinishedAt is the zero value until the node execution reaches a final state.
	FinishedAt time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// TaskAttempt is a row of task_attempt: one try of a node execution under a
// lease epoch (ADR-0009 section 4).
type TaskAttempt struct {
	ID              id.UUID
	OrgID           identity.OrgID
	ExecutionID     ExecutionID
	NodeExecutionID NodeExecutionID
	AttemptNumber   int64
	LeaseEpoch      int64
	Worker          execution.WorkerID
	// Status is STARTED, SUCCEEDED, FAILED, TIMED_OUT, ABANDONED or CANCELED.
	Status     string
	StartedAt  time.Time
	FinishedAt time.Time
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

// OutboxRule decides which events leave the database and where to: it returns
// the topic and payload of the row to insert in the transaction that persists
// the event, or false for an event that stays inside (ADR-0013 section 8).
type OutboxRule func(org identity.OrgID, execution ExecutionID, e execution.Event) (topic string, payload []byte, ok bool)

// EventRouter is implemented by stores that append events themselves, in the
// same transaction as the state change they make (claiming, reaping, firing
// timers): they apply the rule to each such event, so that every event goes
// through the same outbox rule as the ones the engine appends. A nil rule
// publishes nothing.
type EventRouter interface {
	RouteEvents(rule OutboxRule)
}

// OutboxRowID is the identity of the outbox row of an event: derived from the
// event, so that appending the same event twice cannot publish it twice.
func OutboxRowID(event EventID) OutboxID { return OutboxID(id.Derive("outbox", event.String())) }

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

// EngineState is the engine's view of an execution row: its status, the number
// of commits that wrote its tracker document, and the document.
type EngineState struct {
	Status string
	// Revision starts at zero and grows by one with every commit of the engine
	// to the execution. A step built on an older revision is recomputed.
	Revision int64
	// Document is the tracker document (orcon/engine-state@1); nil until the
	// first commit.
	Document []byte
}

// EngineSnapshot is what a worker opens an execution from: the engine state and
// every live token (not terminal), read together.
type EngineSnapshot struct {
	EngineState
	Tokens []Token
}

// EngineStateUpdate commits the engine state of an execution.
type EngineStateUpdate struct {
	OrgID     identity.OrgID
	Execution ExecutionID
	// ExpectedRevision is the revision the committing step read; the commit
	// fails with flow.ErrConflict if the row moved on.
	ExpectedRevision int64
	Status           string
	// Document replaces the stored tracker document unless it is nil.
	Document []byte
	// StartedAt and FinishedAt are set when not zero; StartedAt only the first time.
	StartedAt  time.Time
	FinishedAt time.Time
	Now        time.Time
}

// TokenUpdate moves a token to a status and a position in the flow.
type TokenUpdate struct {
	OrgID  identity.OrgID
	Token  TokenID
	Status string
	// From is the status the row must be in (UpdateToken only).
	From             string
	NodeID           string
	EdgeID           string
	ScopePath        string
	ForkLineage      []byte
	PayloadReference string
	// AvailableAt orders the claims of a token that becomes RUNNABLE; when it
	// is the zero value the token becomes claimable at the database clock plus
	// QueueOrder microseconds.
	AvailableAt time.Time
	QueueOrder  int64
	// ResetAbandoned sets the abandon count of the token back to zero in the same
	// fenced statement: the step stored at least one new result of the token, so
	// whatever abandoned it before is behind it (SPEC-001 Enmienda 12 section 12-1).
	ResetAbandoned bool
}

// NodeOutput is the batch a node execution emitted on a port.
type NodeOutput struct {
	ID              PayloadID
	OrgID           identity.OrgID
	ExecutionID     ExecutionID
	NodeExecutionID NodeExecutionID
	Port            string
	ItemCount       int64
	Encoding        string
	// Document is the orcon/items@1 document.
	Document  []byte
	CreatedAt time.Time
}

// TokenInput is a batch a token starts with: the execution input or the combined
// input of a join continuation or a merged loop iteration.
type TokenInput struct {
	ID          PayloadID
	OrgID       identity.OrgID
	ExecutionID ExecutionID
	ItemCount   int64
	Encoding    string
	Document    []byte
	CreatedAt   time.Time
}

// Nondeterminism is the environment recorded for the activation of a node
// execution (ADR-0013 section 7).
type Nondeterminism struct {
	ID              NondeterminismID
	OrgID           identity.OrgID
	ExecutionID     ExecutionID
	NodeExecutionID NodeExecutionID
	Sequence        int64
	// Document is the orcon/value@1 document.
	Document  []byte
	CreatedAt time.Time
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

// EngineReader reads what a worker needs to open an execution.
type EngineReader interface {
	// LoadSnapshot reads the engine state and the live tokens of an execution in
	// one consistent read, or returns flow.ErrNotFound.
	//
	//orcon:scope engine the engine reads the snapshot of the execution it claimed (ADR-0047 section 2)
	LoadSnapshot(ctx context.Context, org identity.OrgID, execution ExecutionID) (EngineSnapshot, error)
	// GetPayload returns the orcon/items@1 document of a stored batch, or
	// flow.ErrNotFound.
	//
	//orcon:scope engine the engine reads the payloads of the execution it claimed (ADR-0047 section 2)
	GetPayload(ctx context.Context, org identity.OrgID, payload PayloadID) ([]byte, error)
	// ListTokenNondeterminism returns the environments recorded for the node
	// executions of a token.
	//
	//orcon:scope engine the engine reads the recorded environments of the token it claimed (ADR-0047 section 2)
	ListTokenNondeterminism(ctx context.Context, org identity.OrgID, token TokenID, limit int) ([]Nondeterminism, error)
	// ListAttempts returns the attempts of a node execution, in attempt order.
	//
	//orcon:scope engine the engine reads the attempts of the node execution it claimed (ADR-0047 section 2)
	ListAttempts(ctx context.Context, org identity.OrgID, node NodeExecutionID, limit int) ([]TaskAttempt, error)
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

	// LockExecution takes the execution row for the rest of the transaction, so
	// that the steps of one execution commit one after the other, and returns
	// its engine state. On SQLite the single writer already serializes them.
	LockExecution(ctx context.Context, org identity.OrgID, execution ExecutionID) (EngineState, error)
	// LoadSnapshot is [Reader.LoadSnapshot] inside the transaction.
	LoadSnapshot(ctx context.Context, org identity.OrgID, execution ExecutionID) (EngineSnapshot, error)
	// GetPayload is [Reader.GetPayload] inside the transaction: it sees the
	// batches of the commits that share the transaction, which a separate read
	// does not (SQLite commits several steps together).
	GetPayload(ctx context.Context, org identity.OrgID, payload PayloadID) ([]byte, error)
	// SaveEngineState commits the engine state, failing with flow.ErrConflict
	// when the revision is not the expected one.
	SaveEngineState(ctx context.Context, u EngineStateUpdate) error
	// RefreshCounters recomputes the token counters of the execution from its
	// live tokens.
	RefreshCounters(ctx context.Context, org identity.OrgID, execution ExecutionID, now time.Time) error
	// AdvanceToken moves a token the holder of the fence keeps leased along the
	// flow. Another holder or a lost lease fails with [ErrFenced].
	AdvanceToken(ctx context.Context, f Fence, u TokenUpdate, now time.Time) error
	// MoveHeldToken moves a token the holder of the fence leaves: to the status
	// of the update, clearing the lease. A lost lease fails with [ErrFenced].
	MoveHeldToken(ctx context.Context, f Fence, u TokenUpdate, now time.Time) error
	// ReleaseDeferredToken makes a DEFERRED token RUNNABLE. It fails with
	// flow.ErrConflict if the token is not DEFERRED.
	ReleaseDeferredToken(ctx context.Context, org identity.OrgID, token TokenID, queueOrder int64, now time.Time) error
	// CancelToken cancels a token that is not terminal, whoever leases it, and
	// reports whether it was live.
	CancelToken(ctx context.Context, org identity.OrgID, token TokenID, now time.Time) (bool, error)
	// PutNodeOutput stores a batch under its deterministic id; storing it again is a no-op.
	PutNodeOutput(ctx context.Context, o NodeOutput) error
	// PutTokenInput stores a batch under its deterministic id; storing it again is a no-op.
	PutTokenInput(ctx context.Context, i TokenInput) error
	// RecordNondeterminism stores the environment of an activation; storing it
	// again is a no-op.
	RecordNondeterminism(ctx context.Context, n Nondeterminism) error
	// InsertAttempt records the start of an attempt of a node execution, before
	// the work it covers begins (effect nodes commit it first, ADR-0012).
	InsertAttempt(ctx context.Context, a TaskAttempt) error
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
	// PoisonThreshold is the number of abandonments that suspends a token. A normal
	// claim skips the tokens one abandonment short of it ([WorkQueue.ClaimLastChanceToken]
	// takes those); [DefaultPoisonThreshold] when zero.
	PoisonThreshold int64
}

// DefaultPoisonThreshold is the number of abandonments after which a token is
// suspended as poison (ADR-0013 section 9).
const DefaultPoisonThreshold = 3

// MinPoisonThreshold is the smallest threshold allowed: with 1 every token would
// have to run alone (SPEC-001 Enmienda 12 section 12-1).
const MinPoisonThreshold = 2

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
	// FireDueTimers marks up to limit due timers FIRED and, in the same
	// transaction, wakes the WAITING token of each (RUNNABLE, counters and status
	// of the execution refreshed, event timer.fired); concurrent callers never
	// fire the same timer.
	//
	//orcon:scope engine the timer scan covers the due timers of every organization (ADR-0047 section 2)
	FireDueTimers(ctx context.Context, limit int, now time.Time) ([]FiredTimer, error)
}

// LeaseRecovery gives leases back: the reaper's expired ones, the ones an owner
// returns or loses, and the one of a token whose step failed. Whether the abandon
// count grows depends on who gave the lease up (SPEC-001 Enmienda 12 section 12-1).
type LeaseRecovery interface {
	// ReleaseExpiredLeases returns up to limit expired leases to RUNNABLE, or to
	// SUSPENDED with reason POISON once a token was abandoned poisonThreshold
	// times, marks the started attempts of their node executions ABANDONED, and in
	// the same transaction appends the events lease.expired and token.poisoned,
	// opens the attention request of a poisoned token (cause POISON_TOKEN) and
	// refreshes the counters and status of the execution.
	//
	//orcon:scope engine the reaper sweeps expired leases of every organization (ADR-0047 section 2)
	ReleaseExpiredLeases(ctx context.Context, limit int, poisonThreshold int64, now time.Time) ([]Reaped, error)
	// ReleaseWorkerLeases returns every lease of the worker that is still held to
	// RUNNABLE (graceful shutdown, ADR-0013 section 9): the owner gives the tokens
	// back, so the abandon count does not grow, no lease.expired event is appended
	// and the started attempts of their node executions become ABANDONED.
	//
	//orcon:scope engine the leases of one worker can name tokens of several organizations (ADR-0047 section 2)
	ReleaseWorkerLeases(ctx context.Context, worker execution.WorkerID, poisonThreshold int64, now time.Time) ([]Reaped, error)
	// ReleaseLostLeases gives up the leases a previous incarnation of the worker
	// left behind. Nobody returned them, so each counts as an abandonment, like an
	// expired lease: the token goes back to RUNNABLE, or to SUSPENDED with reason
	// POISON at the threshold.
	//
	//orcon:scope engine the leases a previous incarnation of the worker left can name tokens of several organizations (ADR-0047 section 2)
	ReleaseLostLeases(ctx context.Context, worker execution.WorkerID, poisonThreshold int64, now time.Time) ([]Reaped, error)
	// ClaimLastChanceToken leases the one runnable token that is one abandonment
	// short of the poison threshold, if there is one (same order as ClaimTokens).
	// The caller must hold no other token and claim nothing else while it holds this
	// one: the abandonment that reaches the threshold then happens with the token
	// alone in its process (SPEC-001 Enmienda 12 section 12-1).
	//
	//orcon:scope engine the engine claims work of every organization (ADR-0047 section 2)
	ClaimLastChanceToken(ctx context.Context, r ClaimRequest) ([]Claimed, error)
	// LastChanceWaiting reports since when a runnable token one abandonment short
	// of the threshold has been claimable, for the worker to decide that it must
	// empty itself to take it.
	//
	//orcon:scope engine the engine looks for runnable work of every organization (ADR-0047 section 2)
	LastChanceWaiting(ctx context.Context, poisonThreshold int64) (since time.Time, found bool, err error)
	// ReleaseTokenLease gives up the lease of one token the worker could not go on
	// with (the engine failed on a step of it). It counts as an abandonment, like an
	// expired lease, but at once: renewing by worker would keep the lease alive for
	// as long as the worker holds any other. It reports false when the fence no
	// longer holds (the lease was lost or already released).
	//
	//orcon:scope engine the fence names one token; the engine returns the lease of a token it worked on (ADR-0047 section 2)
	ReleaseTokenLease(ctx context.Context, f Fence, poisonThreshold int64, now time.Time) (Reaped, bool, error)
}

// Work is everything the workers of the engine do across organizations.
type Work interface {
	WorkQueue
	LeaseRecovery
}

// Scopes issues the scope of an organization from the id of a row the engine
// read (ADR-0046 section 2). Only the storage adapters implement it: they wrap the
// engine functions, so the engine's ids reach the scoped methods through them.
type Scopes interface {
	//orcon:scope engine the engine turns the organization of the rows it read into a scope (ADR-0047 section 2)
	ScopeOf(org identity.OrgID) (orgscope.Org, error)
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
