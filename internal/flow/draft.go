// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"context"
	"fmt"

	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// Drafts reads and saves the draft of a flow (SPEC-005 Amendment 8 sections 8 and 9). A save
// replaces the whole document when the revision the caller read is still the current one; the
// change and its audit entry are one transaction.
type Drafts struct {
	store DraftStore
	clock clock.Clock
	ids   id.Generator
}

// NewDrafts returns the draft service over its store.
func NewDrafts(store DraftStore, clk clock.Clock, ids id.Generator) *Drafts {
	return &Drafts{store: store, clock: clk, ids: ids}
}

// SaveInput is what a caller supplies to replace a draft.
type SaveInput struct {
	FlowID FlowID
	// Document is the new document as JSON text.
	Document []byte
	// ExpectedRevision is the draft revision the caller read (the value of If-Match).
	ExpectedRevision int64
	// By is the principal that saves; it is the actor of the audit entry.
	By audit.Actor
}

// Get returns the flow with its draft, or [ErrNotFound].
func (d *Drafts) Get(ctx context.Context, org orgscope.Org, flowID FlowID) (Record, error) {
	rec, err := d.store.GetFlow(ctx, org, flowID)
	if err != nil {
		return Record{}, fmt.Errorf("get flow draft: %w", err)
	}
	return rec, nil
}

// Save replaces the draft document and returns the flow at its new revision. A document that
// does not have the shape of the format fails with a [*ValidationError] or an
// [*UnknownFormatError]; the static rules of publication are not applied, so a draft can be
// saved half built. A revision that is no longer current fails with [ErrDraftConflict], a flow
// that does not exist with [ErrNotFound].
func (d *Drafts) Save(ctx context.Context, org orgscope.Org, in SaveInput) (Record, error) {
	raw, err := jsonvalue.Parse(in.Document)
	if err != nil {
		return Record{}, fmt.Errorf("%w: the document is not valid JSON", identity.ErrInvalidInput)
	}
	doc, err := Read(raw)
	if err != nil {
		return Record{}, fmt.Errorf("save flow draft: %w", err)
	}
	canonical, err := jsonvalue.Canonical(doc.Value())
	if err != nil {
		return Record{}, fmt.Errorf("save flow draft: canonical document: %w", err)
	}
	uses := ConnectorUses(&doc)
	refs := make([]ConnectorReference, len(uses))
	for i, u := range uses {
		refs[i] = ConnectorReference{ID: ConnectorReferenceID(d.ids.New()), ConnectorID: u.ConnectorID, ConnectorVersion: u.ConnectorVersion}
	}
	now := clock.Micros(d.clock.Now())
	details, err := jsonvalue.Object(
		jsonvalue.Member{Key: "flowId", Value: jsonvalue.String(id.UUID(in.FlowID).String())},
		jsonvalue.Member{Key: "revision", Value: jsonvalue.Int(in.ExpectedRevision + 1)},
	)
	if err != nil {
		return Record{}, fmt.Errorf("save flow draft: audit: %w", err)
	}
	entry, err := audit.NewEntry(audit.EntryParams{
		ID: d.ids.New(), Chain: audit.OrganizationChain(id.UUID(identity.OrgIDOf(org))), OccurredAt: now, Actor: in.By,
		Action: "flow.updated", Outcome: audit.OutcomeSuccess, Details: details,
	})
	if err != nil {
		return Record{}, fmt.Errorf("save flow draft: audit: %w", err)
	}
	if _, err := d.store.UpdateFlowDraftWithEntry(ctx, org, in.FlowID, canonical, in.ExpectedRevision, refs, now, entry); err != nil {
		return Record{}, fmt.Errorf("save flow draft: %w", err)
	}
	return d.Get(ctx, org, in.FlowID)
}
