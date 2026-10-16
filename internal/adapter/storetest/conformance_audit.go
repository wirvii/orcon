// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest

import (
	"context"
	"testing"

	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// orgAuditAppender is the port the read side of the API uses to audit a read.
type orgAuditAppender interface {
	AppendOrgEntry(ctx context.Context, org orgscope.Org, entry audit.Entry) error
}

// orgAuditCase checks that an entry of an organization chain is stored in its own transaction, shows
// as pending in the chain of its organization, and is refused for any other organization.
func orgAuditCase(t *testing.T, newStore Factory) {
	t.Helper()
	e := newExecFixture(t, newStore)
	appender, ok := e.s.(orgAuditAppender)
	if !ok {
		t.Fatal("the store does not append organization audit entries")
	}
	entry, err := audit.NewEntry(audit.EntryParams{
		ID: next[id.UUID](e.ids), Chain: audit.OrganizationChain(id.UUID(e.orgA)), OccurredAt: base,
		Action: "execution.data_viewed", Outcome: audit.OutcomeSuccess,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := appender.AppendOrgEntry(e.ctx(), e.scopeA, entry); err != nil {
		t.Fatalf("append: %v", err)
	}
	pending, err := e.s.ListPending(e.ctx(), audit.OrganizationChain(id.UUID(e.orgA)), 10)
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	found := 0
	for i := range pending {
		if pending[i].ID == entry.ID && pending[i].Action == "execution.data_viewed" {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("the appended entry is pending %d times, want once", found)
	}
	wantErr(t, "an entry of another organization", appender.AppendOrgEntry(e.ctx(), e.scopeB, entry), audit.ErrInvalidEntry)
	other, err := e.s.ListPending(e.ctx(), audit.OrganizationChain(id.UUID(e.orgB)), 10)
	if err != nil {
		t.Fatalf("pending of the other organization: %v", err)
	}
	for i := range other {
		if other[i].ID == entry.ID {
			t.Fatalf("the entry reached the chain of another organization")
		}
	}
}
