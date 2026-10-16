// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// maxFlowName is the length bound of a flow name (the CHECK of the flow table).
const maxFlowName = 200

// CreateInput is what a caller supplies to create a flow.
type CreateInput struct {
	ProjectID identity.ProjectID
	Name      string
	// By is the principal that creates the flow; it is the actor of the audit entry.
	By audit.Actor
}

// Creator creates flows (SPEC-016 SH4, ADR-0080 section 3). A new flow is inactive and its
// draft is the empty document of the format, and the creation and its audit entry are one
// transaction.
type Creator struct {
	store Store
	clock clock.Clock
	ids   id.Generator
}

// NewCreator returns the flow creation service over its store.
func NewCreator(store Store, clk clock.Clock, ids id.Generator) *Creator {
	return &Creator{store: store, clock: clk, ids: ids}
}

// Create inserts an inactive flow with an empty draft in the project of the scope and the
// audit entry flow.created. A flow name of the project already in use fails with [ErrConflict];
// a project that is not in the organization with [ErrReference].
func (c *Creator) Create(ctx context.Context, org orgscope.Org, in CreateInput) (Record, error) {
	if in.Name == "" || utf8.RuneCountInString(in.Name) > maxFlowName {
		return Record{}, fmt.Errorf("%w: flow name must have 1 to %d characters", identity.ErrInvalidInput, maxFlowName)
	}
	now := clock.Micros(c.clock.Now())
	flowID := FlowID(c.ids.New())
	orgID := identity.OrgIDOf(org)
	draft, err := emptyDraft(flowID, in.Name)
	if err != nil {
		return Record{}, fmt.Errorf("create flow: draft: %w", err)
	}
	rec := Record{
		ID: flowID, OrgID: orgID, ProjectID: in.ProjectID, Name: in.Name, Draft: draft, DraftRevision: 0,
		ActivationState: ActivationInactive, CreatedAt: now, UpdatedAt: now,
	}
	details, err := jsonvalue.Object(
		jsonvalue.Member{Key: "flowId", Value: jsonvalue.String(id.UUID(flowID).String())},
		jsonvalue.Member{Key: "projectId", Value: jsonvalue.String(id.UUID(in.ProjectID).String())},
	)
	if err != nil {
		return Record{}, fmt.Errorf("create flow: audit: %w", err)
	}
	entry, err := audit.NewEntry(audit.EntryParams{
		ID: c.ids.New(), Chain: audit.OrganizationChain(id.UUID(orgID)), OccurredAt: now, Actor: in.By,
		Action: "flow.created", Outcome: audit.OutcomeSuccess, Details: details,
	})
	if err != nil {
		return Record{}, fmt.Errorf("create flow: audit: %w", err)
	}
	if err := c.store.CreateFlowWithEntry(ctx, org, rec, entry); err != nil {
		return Record{}, fmt.Errorf("create flow: %w", err)
	}
	return rec, nil
}

// emptyDraft is the document of a flow with no nodes, variables or edges: the required members
// of orcon/flow@1, canonical so that its content hash is stable.
func emptyDraft(flowID FlowID, name string) ([]byte, error) {
	settings, err := jsonvalue.Object()
	if err != nil {
		return nil, err
	}
	editor, err := jsonvalue.Object()
	if err != nil {
		return nil, err
	}
	doc, err := jsonvalue.Object(
		jsonvalue.Member{Key: "format", Value: jsonvalue.String("orcon/flow@1")},
		jsonvalue.Member{Key: "id", Value: jsonvalue.String(id.UUID(flowID).String())},
		jsonvalue.Member{Key: "name", Value: jsonvalue.String(name)},
		jsonvalue.Member{Key: "settings", Value: settings},
		jsonvalue.Member{Key: "variables", Value: jsonvalue.Array()},
		jsonvalue.Member{Key: "nodes", Value: jsonvalue.Array()},
		jsonvalue.Member{Key: "edges", Value: jsonvalue.Array()},
		jsonvalue.Member{Key: "editor", Value: editor},
	)
	if err != nil {
		return nil, err
	}
	return jsonvalue.Canonical(doc)
}
