// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package agent runs orcon.ai.agent@1 (SPEC-006 cut AI3, ADR-0060): a stepped
// node that calls a model, runs the tools the model asks for, and repeats until
// the model answers without a tool call. Every model response and every tool
// result is committed in one short transaction with its transcript rows, so a
// run that is interrupted resumes at the step it stopped at: the recorded steps
// are replayed, and the model is not asked again for an answer it already gave.
//
// The package is an area package. Persistence is the [Store] port and the model
// adapters are reached through the chain.Models port; it imports no adapter.
package agent

import (
	"context"
	"strconv"
	"time"

	"github.com/wirvii/orcon/internal/ai/cost"
	"github.com/wirvii/orcon/internal/ai/provider"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

// Status is the state of a run.
type Status string

// The run statuses.
const (
	StatusRunning   Status = "RUNNING"
	StatusSucceeded Status = "SUCCEEDED"
	StatusFailed    Status = "FAILED"
)

// StepKind says what a step is.
type StepKind string

// The step kinds: a call to the model, or a call to a tool.
const (
	StepModel StepKind = "MODEL"
	StepTool  StepKind = "TOOL"
)

// ModelUse is what one model call used and cost. It is recorded with the MODEL
// step that made the call.
type ModelUse struct {
	Provider     string
	Model        string
	Usage        cost.Usage
	CostMicro    int64
	Priced       bool
	PriceVersion string
	CostStatus   string
	FinishReason provider.FinishReason
}

// StepRecord is one committed step of a run.
type StepRecord struct {
	Number     int
	Kind       StepKind
	ToolCallID string
	ToolName   string
	// Output is the JSON document of a MODEL step (text and tool calls) or the
	// result text of a TOOL step.
	Output  string
	IsError bool
	// ArgumentsInvalid marks a TOOL step whose arguments did not match the
	// tool's schema; two in a row for the same tool fail the run (SPEC-006 R-PRV-03).
	ArgumentsInvalid bool
	Model            ModelUse
}

// Part kinds of a transcript row.
const (
	PartText       = "TEXT"
	PartToolCall   = "TOOL_CALL"
	PartToolResult = "TOOL_RESULT"
)

// TranscriptRow is one message part of the run's transcript, in order. The
// transcript is append-only: a row is written once and never changed.
type TranscriptRow struct {
	Sequence int
	Role     provider.Role
	Part     string
	CallID   string
	ToolName string
	Text     string
	IsError  bool
}

// MemoryRow is one message of a conversation's memory.
type MemoryRow struct {
	Role provider.Role
	Text string
}

// RunState is the stored state of a run.
type RunState struct {
	Status        Status
	StepCount     int
	ToolCallCount int
	SpendMicro    int64
	FinalText     string
	ErrorCode     string
}

// Commit is everything one committed step writes, in one transaction.
type Commit struct {
	RunID id.UUID
	// ConversationKey names the conversation the run belongs to; empty means no memory.
	ConversationKey string
	Step            StepRecord
	Rows            []TranscriptRow
	// Final marks the step that ends the run: the run becomes SUCCEEDED with
	// FinalText, and the user message and the answer join the conversation memory.
	Final     bool
	FinalText string
	// Remember is what joins the conversation memory with the final commit: the
	// user message and the answer.
	Remember []MemoryRow
	// Memory is the snapshot of conversation memory the run started from. It is
	// written only with the first commit, so a replay sees the same context.
	Memory []MemoryRow
	// Cost is the usage of a MODEL step, rolled up into every scope in the same transaction.
	Cost   *cost.Entry
	Scopes []cost.Scope
	At     time.Time
}

// Store is the persistence port of the agent runs. Every method runs in one
// transaction or reads a consistent snapshot; no transaction is held across a
// model or tool call.
type Store interface {
	// Run returns the state of a run; the boolean is false when it never started.
	Run(ctx context.Context, org identity.OrgID, runID id.UUID) (RunState, bool, error)
	// Steps returns the committed steps of a run in order.
	Steps(ctx context.Context, org identity.OrgID, runID id.UUID) ([]StepRecord, error)
	// Transcript returns the transcript rows of a run in order.
	Transcript(ctx context.Context, org identity.OrgID, runID id.UUID) ([]TranscriptRow, error)
	// Commit writes one step: the step, its model call when it has one, its
	// transcript rows, the run counters and the rollups of its cost.
	Commit(ctx context.Context, org identity.OrgID, c Commit) error
	// Memory returns the most recent messages of a conversation, oldest first.
	Memory(ctx context.Context, org identity.OrgID, conversationKey string, limit int) ([]MemoryRow, error)
}

// RunID derives the identity of the run of one item of one node execution. The
// same node execution always names the same run, so a re-run finds its steps.
func RunID(org identity.OrgID, execution, nodeExecution string, item int) id.UUID {
	return id.Derive("ai_agent_run", org.String(), execution, nodeExecution, strconv.Itoa(item))
}
