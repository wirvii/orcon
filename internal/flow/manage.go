// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// Manager changes the row of a flow (SPEC-005 Amendment 8 section 8-9): today its name. The
// change and its audit entry are one transaction.
type Manager struct {
	store ManageStore
	clock clock.Clock
	ids   id.Generator
}

// NewManager returns the flow management service over its store.
func NewManager(store ManageStore, clk clock.Clock, ids id.Generator) *Manager {
	return &Manager{store: store, clock: clk, ids: ids}
}

// Get returns the non-deleted flow, or [ErrNotFound]. Unlike the list reads, it fills
// [Record.HasUnpublishedChanges] exactly: it compares the content hash of the draft with the one of
// the active version (SPEC-002 R-ACT-01).
func (m *Manager) Get(ctx context.Context, org orgscope.Org, flowID FlowID) (Record, error) {
	rec, err := m.store.GetFlow(ctx, org, flowID)
	if err != nil {
		return Record{}, fmt.Errorf("get flow: %w", err)
	}
	rec.HasUnpublishedChanges, err = UnpublishedChanges(ctx, m.store, org, &rec)
	if err != nil {
		return Record{}, fmt.Errorf("get flow: %w", err)
	}
	return rec, nil
}

// DraftReader is what [UnpublishedChanges] reads: a flow with its draft and a version of it.
type DraftReader interface {
	// GetFlow returns a non-deleted flow, or [ErrNotFound].
	GetFlow(ctx context.Context, org orgscope.Org, flow FlowID) (Record, error)
	// GetVersion returns a version by identity, or [ErrVersionNotFound].
	GetVersion(ctx context.Context, org orgscope.Org, version FlowVersionID) (VersionRecord, error)
}

// UnpublishedChanges reports whether the draft of the flow differs from its active version, by
// comparing content hashes (SPEC-002 R-ACT-01). A flow that was never published has changes. When
// rec carries no draft (the activity reads leave it out), the draft is read from the store.
func UnpublishedChanges(ctx context.Context, r DraftReader, org orgscope.Org, rec *Record) (bool, error) {
	if rec.ActiveVersionID.IsZero() {
		return true, nil
	}
	if len(rec.Draft) == 0 {
		full, err := r.GetFlow(ctx, org, rec.ID)
		if err != nil {
			return false, fmt.Errorf("read the draft: %w", err)
		}
		rec.Draft = full.Draft
	}
	v, err := r.GetVersion(ctx, org, rec.ActiveVersionID)
	if err != nil {
		return false, fmt.Errorf("read the active version: %w", err)
	}
	return DraftDiffers(rec.Draft, v.ContentHash), nil
}

// DraftDiffers reports whether the draft, normalized the way publication normalizes it, has a
// different content hash than the live version. A draft that does not read as a document differs by
// definition.
func DraftDiffers(draft []byte, liveHash string) bool {
	value, err := jsonvalue.Parse(draft)
	if err != nil {
		return true
	}
	doc, err := Read(value)
	if err != nil {
		return true
	}
	h, err := ContentHash(&doc)
	if err != nil {
		return true
	}
	return h.String() != liveHash
}

// RenameInput is what a caller supplies to rename a flow.
type RenameInput struct {
	FlowID FlowID
	// Name is the new display name.
	Name string
	// ExpectedRevision is the revision of the flow the caller read (the value of If-Match).
	ExpectedRevision int64
	// By is the principal that renames; it is the actor of the audit entry.
	By audit.Actor
}

// Rename changes the name of the flow when ExpectedRevision is still the current revision of the
// flow row and returns the flow as it is afterwards. A name that is empty or too long fails with
// [identity.ErrInvalidInput]; a stale revision with [ErrRevisionConflict]; a name the project
// already uses with [ErrConflict]; a flow that does not exist with [ErrNotFound].
func (m *Manager) Rename(ctx context.Context, org orgscope.Org, in RenameInput) (Record, error) {
	if strings.TrimSpace(in.Name) == "" || utf8.RuneCountInString(in.Name) > maxFlowName {
		return Record{}, fmt.Errorf("%w: flow name must have 1 to %d characters", identity.ErrInvalidInput, maxFlowName)
	}
	now := clock.Micros(m.clock.Now())
	details, err := jsonvalue.Object(
		jsonvalue.Member{Key: "flowId", Value: jsonvalue.String(id.UUID(in.FlowID).String())},
		jsonvalue.Member{Key: "fields", Value: jsonvalue.Array(jsonvalue.String("name"))},
	)
	if err != nil {
		return Record{}, fmt.Errorf("rename flow: audit: %w", err)
	}
	entry, err := audit.NewEntry(audit.EntryParams{
		ID: m.ids.New(), Chain: audit.OrganizationChain(id.UUID(identity.OrgIDOf(org))), OccurredAt: now, Actor: in.By,
		Action: "flow.updated", Outcome: audit.OutcomeSuccess, Details: details,
	})
	if err != nil {
		return Record{}, fmt.Errorf("rename flow: audit: %w", err)
	}
	if err := m.store.UpdateFlowNameWithEntry(ctx, org, in.FlowID, in.Name, in.ExpectedRevision, now, entry); err != nil {
		return Record{}, fmt.Errorf("rename flow: %w", err)
	}
	return m.Get(ctx, org, in.FlowID)
}
