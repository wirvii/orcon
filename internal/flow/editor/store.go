// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package editor is the storage port of the visual flow editor (SPEC-014
// section 6, ADR-0082): node test outputs, pins of a node to one output, test
// runs and their seeds, effect previews and editing sessions. Every record
// belongs to one organization; every read and write takes the organization
// and returns nothing of another one.
//
// The package is a port: it holds types and the interface only. The SQLite and
// PostgreSQL adapters implement [Store].
package editor

import (
	"context"
	"time"

	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// Source kinds of a node test output (ADR-0082 section 3).
const (
	SourceManual          = "MANUAL"
	SourceExecution       = "EXECUTION"
	SourceTestRun         = "TEST_RUN"
	SourceN8NPinData      = "N8N_PIN_DATA"
	SourceMock            = "MOCK"
	SourcePriorTestOutput = "PRIOR_TEST_OUTPUT"
)

// Target kinds and effect modes of a test run.
const (
	TargetFlow       = "FLOW"
	TargetNode       = "NODE"
	TargetUpToNode   = "UP_TO_NODE"
	EffectConfirm    = "CONFIRM"
	EffectSubstitute = "SUBSTITUTE"
)

// Seed kinds of a test run.
const (
	SeedPin             = "PIN"
	SeedPriorTestOutput = "PRIOR_TEST_OUTPUT"
	SeedSubstitute      = "SUBSTITUTE"
)

// Status, effect and safety values of an effect preview (SPEC-014 section 6).
const (
	PreviewOpen              = "OPEN"
	PreviewDispatchConfirmed = "DISPATCH_CONFIRMED"
	PreviewSubstituted       = "SUBSTITUTED"
	PreviewExpired           = "EXPIRED"
	PreviewCanceled          = "CANCELED"
	EffectWrite              = "WRITE"
	EffectDelete             = "DELETE"
	EffectUnknown            = "UNKNOWN"
	SafetyIdempotencyKey     = "IDEMPOTENCY_KEY"
	SafetyReconcile          = "RECONCILE"
	SafetyUnsafe             = "UNSAFE"
	RetentionEditorTestData  = "editor_test_data"
	RetentionEditorPresence  = "editor_presence"
	NodeTestOutputFormat     = "orcon/node-test-output@1"
	TestPlanFormat           = "orcon/test-plan@1"
	EffectPreviewFormat      = "orcon/effect-preview@1"
	EffectDecisionFormat     = "orcon/effect-decision@1"
	NodeIDMaxLength          = 64
	ClientLabelMaxLength     = 64
	PublicationNoteMaxLength = 500
	PinBytesMaxPerNode       = 2 << 20
	PinBytesMaxPerFlow       = 16 << 20
	EffectPreviewMaxBytes    = 256 << 10
	InlineDocumentMaxBytes   = 256 << 10
	ListPageLimitForEditor   = 50
)

// NodeTestData is one immutable output of a node: an input to a pin, a mock, or
// the result of a test run or an execution (ADR-0082 section 3). The document is
// inline up to 256 KiB; larger outputs live in a blob and carry only BlobID.
type NodeTestData struct {
	ID                 id.UUID
	OrgID              identity.OrgID
	FlowID             flow.FlowID
	NodeID             string
	SourceKind         string
	OutputDocument     []byte
	BlobID             id.UUID // zero when the output is inline
	ItemCount          int64
	SizeBytes          int64
	ContentSHA256      []byte
	HasMaskedValues    bool
	CreatedByPrincipal identity.PrincipalID // zero when the principal was deleted
	CreatedAt          time.Time
}

// NodePin is the current pin of one node of one flow.
type NodePin struct {
	ID                id.UUID
	OrgID             identity.OrgID
	FlowID            flow.FlowID
	NodeID            string
	NodeTestDataID    id.UUID
	Revision          int64
	PinnedByPrincipal identity.PrincipalID // zero when the principal was deleted
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// TestRun is the record of one TEST execution (one row per execution).
type TestRun struct {
	ID                 id.UUID
	OrgID              identity.OrgID
	FlowID             flow.FlowID
	ExecutionID        store.ExecutionID
	FlowSnapshotID     flow.FlowSnapshotID
	TargetKind         string
	TargetNodeID       string // empty when the target is the whole flow
	EffectMode         string
	PlanDocument       []byte
	StartedByPrincipal identity.PrincipalID // zero when the principal was deleted
	CreatedAt          time.Time
}

// TestRunSeed records which immutable output seeded a node of a test run.
type TestRunSeed struct {
	ID             id.UUID
	OrgID          identity.OrgID
	TestRunID      id.UUID
	NodeID         string
	SeedKind       string
	NodeTestDataID id.UUID
	CreatedAt      time.Time
}

// EffectPreview is the question put to a person before an external effect of a
// node in a test run is dispatched (SPEC-014 section 7).
type EffectPreview struct {
	ID                 id.UUID
	OrgID              identity.OrgID
	ExecutionID        store.ExecutionID
	NodeExecutionID    store.NodeExecutionID
	NodeID             string
	Status             string
	Effect             string
	SafetyLevel        string
	ItemCount          int64
	IsPreviewAvailable bool
	PreviewDocument    []byte
	PreviewSHA256      []byte
	DecisionDocument   []byte // nil until a decision is recorded
	ExpiresAt          time.Time
	DecidedByPrincipal identity.PrincipalID // zero until decided
	DecidedAt          time.Time            // zero until decided
	Revision           int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// EditingSession is the presence of one person in one flow editor. A session is
// removed hard when it expires (bookkeeping).
type EditingSession struct {
	ID                    id.UUID
	OrgID                 identity.OrgID
	FlowID                flow.FlowID
	OpenedByPrincipal     identity.PrincipalID // zero when the principal was deleted
	ClientLabel           string
	LastSeenDraftRevision int64
	OpenedAt              time.Time
	LastSeenAt            time.Time
	ExpiresAt             time.Time
	Revision              int64
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// Store is the storage port of the editor. Errors: [flow.ErrNotFound] when no
// row of the organization matches; [flow.ErrConflict] when a compare-and-set
// misses (a stale revision, or a preview that already has a decision).
type Store interface { //nolint:interfacebloat // one area's storage port; splitting it would scatter the transaction boundaries of one record family
	// InsertNodeTestData appends an output. The record must name the organization of the scope.
	InsertNodeTestData(ctx context.Context, org orgscope.Org, data NodeTestData) error
	// GetNodeTestData returns one output of the organization.
	GetNodeTestData(ctx context.Context, org orgscope.Org, dataID id.UUID) (NodeTestData, error)
	// ListNodeTestDataForNode returns the latest outputs of one node, newest first, at most limit.
	ListNodeTestDataForNode(ctx context.Context, org orgscope.Org, flowID flow.FlowID, nodeID string, limit int) ([]NodeTestData, error)

	// PinNode sets the pin of a node: the first call creates revision 1, later calls
	// replace the output and count one more revision.
	PinNode(ctx context.Context, org orgscope.Org, pin NodePin) error
	// GetNodePin returns the pin of one node.
	GetNodePin(ctx context.Context, org orgscope.Org, flowID flow.FlowID, nodeID string) (NodePin, error)
	// ListNodePins returns the pins of a flow after afterNodeID (keyset by node identifier), at most limit.
	ListNodePins(ctx context.Context, org orgscope.Org, flowID flow.FlowID, afterNodeID string, limit int) ([]NodePin, error)
	// UnpinNode removes the pin of one node.
	UnpinNode(ctx context.Context, org orgscope.Org, flowID flow.FlowID, nodeID string) error

	// InsertTestRun records a test run.
	InsertTestRun(ctx context.Context, org orgscope.Org, run TestRun) error
	// GetTestRun returns one test run of the organization.
	GetTestRun(ctx context.Context, org orgscope.Org, runID id.UUID) (TestRun, error)
	// ListTestRuns returns the test history of a flow, newest first, at most limit.
	ListTestRuns(ctx context.Context, org orgscope.Org, flowID flow.FlowID, limit int) ([]TestRun, error)
	// InsertTestRunSeed records which output seeded a node of a run.
	InsertTestRunSeed(ctx context.Context, org orgscope.Org, seed TestRunSeed) error
	// ListTestRunSeeds returns the seeds of a run, at most limit.
	ListTestRunSeeds(ctx context.Context, org orgscope.Org, runID id.UUID, limit int) ([]TestRunSeed, error)

	// InsertEffectPreview records the preview of one gated node activation.
	InsertEffectPreview(ctx context.Context, org orgscope.Org, preview *EffectPreview) error
	// GetEffectPreview returns one preview of the organization.
	GetEffectPreview(ctx context.Context, org orgscope.Org, previewID id.UUID) (EffectPreview, error)
	// DecideEffectPreview records the decision of an open preview whose revision is expectedRevision.
	DecideEffectPreview(ctx context.Context, org orgscope.Org, previewID id.UUID, expectedRevision int64, status string, decision []byte, decidedBy identity.PrincipalID, now time.Time) error
	// ListEffectPreviews returns the previews of an execution, at most limit.
	ListEffectPreviews(ctx context.Context, org orgscope.Org, execID store.ExecutionID, limit int) ([]EffectPreview, error)

	// UpsertEditingSession opens a session or renews it (same identifier).
	UpsertEditingSession(ctx context.Context, org orgscope.Org, session EditingSession) error
	// ListEditingSessions returns the sessions of a flow that have not expired at now, at most limit.
	ListEditingSessions(ctx context.Context, org orgscope.Org, flowID flow.FlowID, now time.Time, limit int) ([]EditingSession, error)

	// PurgeEditorTestData removes up to limit outputs that no pin and no run seed references and that were
	// created more than 24 hours before now, and returns how many it removed (ADR-0017 amendment).
	// It crosses organizations: it is the engine's retention, not a request of a person.
	//orcon:scope engine the retention of every organization (ADR-0047 section 2, ADR-0017 amendment)
	PurgeEditorTestData(ctx context.Context, now time.Time, limit int) (int64, error)
	// PurgeEditorPresence removes up to limit sessions that expired before now and returns how many it removed.
	//orcon:scope engine the retention of every organization (ADR-0047 section 2)
	PurgeEditorPresence(ctx context.Context, now time.Time, limit int) (int64, error)
}
