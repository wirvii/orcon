// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wirvii/orcon/internal/adapter/postgres/sqlcgen"
	"github.com/wirvii/orcon/internal/ai/agent"
	"github.com/wirvii/orcon/internal/ai/cost"
	"github.com/wirvii/orcon/internal/ai/provider"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// maxRunRows bounds the steps, model calls and transcript rows one run reads.
// A run is capped far below it (SPEC-006 R-AG-06); a run that reaches it fails.
const maxRunRows = 10000

var _ agent.Store = (*Store)(nil)

// Run implements [agent.Store].
func (s *Store) Run(ctx context.Context, scope orgscope.Org, runID id.UUID) (agent.RunState, bool, error) {
	org := identity.OrgID(scope.UUID())
	var (
		st    agent.RunState
		found bool
	)
	err := s.inOrg(ctx, "read agent run", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetAIAgentRun(ctx, pgUUID(org), pgUUID(runID))
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		found = true
		st = agent.RunState{
			Status:        agent.Status(row.Status),
			StepCount:     int(row.StepCount),
			ToolCallCount: int(row.ToolCallCount),
			SpendMicro:    row.SpendMicrodollars,
			FinalText:     row.FinalText.String,
			ErrorCode:     row.ErrorCode.String,
		}
		return nil
	})
	return st, found, err
}

// Steps implements [agent.Store]. A MODEL step carries the usage of its call.
func (s *Store) Steps(ctx context.Context, scope orgscope.Org, runID id.UUID) ([]agent.StepRecord, error) {
	org := identity.OrgID(scope.UUID())
	var out []agent.StepRecord
	err := s.inOrg(ctx, "read agent steps", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		steps, err := q.ListAIAgentSteps(ctx, pgUUID(org), pgUUID(runID), maxRunRows)
		if err != nil {
			return err
		}
		calls, err := q.ListAIModelCallsForRun(ctx, pgUUID(org), pgUUID(runID), maxRunRows)
		if err != nil {
			return err
		}
		byStep := make(map[[16]byte]sqlcgen.ListAIModelCallsForRunRow, len(calls))
		for i := range calls {
			byStep[calls[i].AiAgentStepID.Bytes] = calls[i]
		}
		out = make([]agent.StepRecord, 0, len(steps))
		for i := range steps {
			st := &steps[i]
			rec := agent.StepRecord{
				Number:           int(st.StepNumber),
				Kind:             agent.StepKind(st.Kind),
				ToolCallID:       st.ToolCallKey,
				ToolName:         st.ToolName,
				Output:           st.OutputText,
				IsError:          st.IsError,
				ArgumentsInvalid: st.IsArgumentsInvalid,
			}
			if c, ok := byStep[st.ID.Bytes]; ok {
				rec.Model = modelUseOf(&c)
			}
			out = append(out, rec)
		}
		return nil
	})
	return out, err
}

// modelUseOf reads the model call row of a MODEL step.
func modelUseOf(c *sqlcgen.ListAIModelCallsForRunRow) agent.ModelUse {
	return agent.ModelUse{
		Provider: c.Provider, Model: c.Model,
		Usage: cost.Usage{
			Input: c.InputTokenCount, Output: c.OutputTokenCount, CacheRead: c.CacheReadTokenCount,
			CacheWrite: c.CacheWriteTokenCount, Reasoning: c.ReasoningTokenCount,
		},
		CostMicro: c.CostMicrodollars, Priced: c.CostStatus != "UNKNOWN", PriceVersion: c.PriceVersion,
		CostStatus: c.CostStatus, FinishReason: provider.FinishReason(c.FinishReason),
	}
}

// Transcript implements [agent.Store].
func (s *Store) Transcript(ctx context.Context, scope orgscope.Org, runID id.UUID) ([]agent.TranscriptRow, error) {
	org := identity.OrgID(scope.UUID())
	var out []agent.TranscriptRow
	err := s.inOrg(ctx, "read agent transcript", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListAIAgentMessages(ctx, pgUUID(org), pgUUID(runID), maxRunRows)
		if err != nil {
			return err
		}
		out = make([]agent.TranscriptRow, 0, len(rows))
		for i := range rows {
			r := &rows[i]
			out = append(out, agent.TranscriptRow{
				Sequence: int(r.SequenceNumber), Role: provider.Role(r.Role), Part: r.PartKind,
				CallID: r.CallKey, ToolName: r.ToolName, Text: r.ContentText, IsError: r.IsError,
			})
		}
		return nil
	})
	return out, err
}

// Memory implements [agent.Store]: the newest messages of a conversation, oldest first.
func (s *Store) Memory(ctx context.Context, scope orgscope.Org, conversationKey string, limit int) ([]agent.MemoryRow, error) {
	org := identity.OrgID(scope.UUID())
	var out []agent.MemoryRow
	err := s.inOrg(ctx, "read conversation memory", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListAIConversationMemory(ctx, pgUUID(org), pgUUID(threadKeyOf(org, conversationKey)), int32(min(limit, maxRunRows))) //nolint:gosec // bounded by maxRunRows above
		if err != nil {
			return err
		}
		out = make([]agent.MemoryRow, 0, len(rows))
		for i := len(rows) - 1; i >= 0; i-- {
			out = append(out, agent.MemoryRow{Role: provider.Role(rows[i].Role), Text: rows[i].ContentText})
		}
		return nil
	})
	return out, err
}

// Commit implements [agent.Store]: one step, with everything it writes, in one transaction.
func (s *Store) Commit(ctx context.Context, scope orgscope.Org, c agent.Commit) error { //nolint:gocritic // the port passes the commit by value
	org := identity.OrgID(scope.UUID())
	return s.inOrg(ctx, "commit agent step", org, false, func(ctx context.Context, q *sqlcgen.Queries) error {
		return commitStep(ctx, q, org, &c)
	})
}

// commitStep writes one step inside the transaction of its caller: the run row,
// the step, its model call, its transcript rows, the run state, the memory and
// the rollups of its cost.
func commitStep(ctx context.Context, q *sqlcgen.Queries, org identity.OrgID, c *agent.Commit) error {
	at := timestamp(c.At)
	if err := q.InsertAIAgentRunIfAbsent(ctx, sqlcgen.InsertAIAgentRunIfAbsentParams{
		ID: pgUUID(c.RunID), OrgID: pgUUID(org), CreatedAt: at, UpdatedAt: at,
	}); err != nil {
		return err
	}
	stepID, err := writeStep(ctx, q, org, c)
	if err != nil {
		return err
	}
	if c.Step.Kind == agent.StepModel {
		if err := writeModelCall(ctx, q, org, c, stepID); err != nil {
			return err
		}
	}
	if err := writeRows(ctx, q, org, c); err != nil {
		return err
	}
	if err := writeRunState(ctx, q, org, c); err != nil {
		return err
	}
	if err := rememberTurns(ctx, q, org, c); err != nil {
		return err
	}
	return writeRollups(ctx, q, org, c)
}

func writeStep(ctx context.Context, q *sqlcgen.Queries, org identity.OrgID, c *agent.Commit) (id.UUID, error) {
	stepID := derivedID("ai_agent_step", org, c.RunID, c.Step.Number)
	at := timestamp(c.At)
	err := q.InsertAIAgentStep(ctx, sqlcgen.InsertAIAgentStepParams{
		ID: pgUUID(stepID), OrgID: pgUUID(org), AiAgentRunID: pgUUID(c.RunID), StepNumber: int64(c.Step.Number),
		Kind: string(c.Step.Kind), ToolCallKey: c.Step.ToolCallID, ToolName: c.Step.ToolName, OutputText: c.Step.Output,
		IsError: c.Step.IsError, IsArgumentsInvalid: c.Step.ArgumentsInvalid, CreatedAt: at, UpdatedAt: at,
	})
	return stepID, err
}

func writeModelCall(ctx context.Context, q *sqlcgen.Queries, org identity.OrgID, c *agent.Commit, stepID id.UUID) error {
	m := &c.Step.Model
	at := timestamp(c.At)
	return q.InsertAIModelCall(ctx, sqlcgen.InsertAIModelCallParams{
		ID: pgUUID(derivedID("ai_model_call", org, stepID)), OrgID: pgUUID(org), AiAgentStepID: pgUUID(stepID),
		Provider: m.Provider, Model: m.Model, InputTokenCount: m.Usage.Input, OutputTokenCount: m.Usage.Output,
		CacheReadTokenCount: m.Usage.CacheRead, CacheWriteTokenCount: m.Usage.CacheWrite,
		ReasoningTokenCount: m.Usage.Reasoning, CostMicrodollars: m.CostMicro, CostStatus: m.CostStatus,
		PriceVersion: m.PriceVersion, FinishReason: string(m.FinishReason), CreatedAt: at, UpdatedAt: at,
	})
}

func writeRows(ctx context.Context, q *sqlcgen.Queries, org identity.OrgID, c *agent.Commit) error {
	at := timestamp(c.At)
	for i := range c.Rows {
		r := &c.Rows[i]
		if err := q.InsertAIAgentMessage(ctx, sqlcgen.InsertAIAgentMessageParams{
			ID: pgUUID(derivedID("ai_agent_message", org, c.RunID, r.Sequence)), OrgID: pgUUID(org), AiAgentRunID: pgUUID(c.RunID),
			SequenceNumber: int64(r.Sequence), Role: string(r.Role), PartKind: r.Part, CallKey: r.CallID, ToolName: r.ToolName,
			IsError: r.IsError, ContentText: r.Text, CreatedAt: at, UpdatedAt: at,
		}); err != nil {
			return err
		}
	}
	return nil
}

// writeRunState moves the counters of the run and, on its last step, marks it succeeded.
func writeRunState(ctx context.Context, q *sqlcgen.Queries, org identity.OrgID, c *agent.Commit) error {
	var toolCalls, spend int64
	if c.Step.Kind == agent.StepTool {
		toolCalls = 1
	}
	if c.Cost != nil {
		spend = c.Cost.CostMicro
	}
	at := timestamp(c.At)
	if err := q.UpdateAIAgentRunCounters(ctx, sqlcgen.UpdateAIAgentRunCountersParams{
		OrgID: pgUUID(org), ID: pgUUID(c.RunID), ToolCalls: toolCalls, SpendMicrodollars: spend, UpdatedAt: at,
	}); err != nil {
		return err
	}
	if !c.Final {
		return nil
	}
	return q.MarkAIAgentRunSucceeded(ctx, sqlcgen.MarkAIAgentRunSucceededParams{
		OrgID: pgUUID(org), ID: pgUUID(c.RunID), FinalText: pgtype.Text{String: c.FinalText, Valid: true}, UpdatedAt: at,
	})
}

// writeRollups adds the cost of the step to the rollups of every scope.
func writeRollups(ctx context.Context, q *sqlcgen.Queries, org identity.OrgID, c *agent.Commit) error {
	if c.Cost == nil {
		return nil
	}
	for _, sc := range c.Scopes {
		for _, gran := range rollupGranularities {
			if err := upsertRollup(ctx, q, org, sc, gran, *c.Cost, c.At); err != nil {
				return err
			}
		}
	}
	return nil
}

// rememberTurns appends the user message and the answer to the conversation
// memory when the run ends. Sequences follow the conversation's last message.
func rememberTurns(ctx context.Context, q *sqlcgen.Queries, org identity.OrgID, c *agent.Commit) error {
	if !c.Final || c.ConversationKey == "" || len(c.Remember) == 0 {
		return nil
	}
	thread := threadKeyOf(org, c.ConversationKey)
	last, err := q.GetAIConversationLastSequence(ctx, pgUUID(org), pgUUID(thread))
	if err != nil {
		return err
	}
	at := timestamp(c.At)
	for i := range c.Remember {
		m := &c.Remember[i]
		last++
		if err := q.InsertAIConversationMessage(ctx, sqlcgen.InsertAIConversationMessageParams{
			ID: pgUUID(derivedID("ai_conversation_message", org, thread, last)), OrgID: pgUUID(org), ThreadKey: pgUUID(thread),
			ConversationKey: c.ConversationKey, SequenceNumber: last, Role: string(m.Role), ContentText: m.Text,
			CreatedAt: at, UpdatedAt: at,
		}); err != nil {
			return fmt.Errorf("remember conversation message: %w", err)
		}
	}
	return nil
}

func threadKeyOf(org identity.OrgID, conversationKey string) id.UUID {
	return id.Derive("ai_thread", org.String(), conversationKey)
}

// derivedID names a row of the agent tables by its parents, so a retried write
// of the same step names the same rows.
func derivedID(table string, parts ...any) id.UUID {
	fields := []string{table}
	for _, p := range parts {
		fields = append(fields, fmt.Sprint(p))
	}
	return id.Derive(fields...)
}
