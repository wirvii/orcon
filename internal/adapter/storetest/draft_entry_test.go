// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/audit/audittest"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/id"
)

// TestUpdateFlowDraftWithEntryIsOneTransaction (SPEC-005 Amendment 8 section 9): a save with its
// audit entry moves the draft one revision; a stale revision and an entry the store refuses both
// leave the draft as it was.
func TestUpdateFlowDraftWithEntryIsOneTransaction(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		g := newGrantFixture(t, newStore)
		scope := scopeOf(g.orgA)
		rec := flow.Record{
			ID: next[flow.FlowID](g.fixture), OrgID: g.orgA, ProjectID: g.projA, Name: "Welcome",
			Draft: []byte(`{"n":0}`), ActivationState: flow.ActivationInactive, CreatedAt: base, UpdatedAt: base,
		}
		if err := g.s.CreateFlow(g.ctx(), scope, rec); err != nil {
			t.Fatalf("create flow: %v", err)
		}
		good := audittest.Org(t, id.UUID(g.orgA), "flow.updated")
		rev, err := g.s.UpdateFlowDraftWithEntry(g.ctx(), scope, rec.ID, []byte(`{"n":1}`), 0, nil, base.Add(time.Second), good)
		if err != nil || rev != 1 {
			t.Fatalf("save = %d, %v; want revision 1", rev, err)
		}
		stale := audittest.Org(t, id.UUID(g.orgA), "flow.updated")
		if _, err := g.s.UpdateFlowDraftWithEntry(g.ctx(), scope, rec.ID, []byte(`{"n":2}`), 0, nil, base.Add(2*time.Second), stale); !errors.Is(err, flow.ErrDraftConflict) {
			t.Fatalf("stale save = %v, want ErrDraftConflict", err)
		}
		foreign := audittest.Org(t, id.UUID(g.orgB), "flow.updated")
		if _, err := g.s.UpdateFlowDraftWithEntry(g.ctx(), scope, rec.ID, []byte(`{"n":3}`), 1, nil, base.Add(3*time.Second), foreign); err == nil {
			t.Fatal("an entry of another organization was accepted")
		}
		got, err := g.s.GetFlow(g.ctx(), scope, rec.ID)
		if err != nil || got.DraftRevision != 1 || !bytes.Equal(got.Draft, []byte(`{"n":1}`)) {
			t.Errorf("flow = %+v, %v; want the draft of revision 1 untouched by the refused saves", got, err)
		}
		missing := next[flow.FlowID](g.fixture)
		entry := audittest.Org(t, id.UUID(g.orgA), "flow.updated")
		if _, err := g.s.UpdateFlowDraftWithEntry(g.ctx(), scope, missing, []byte(`{}`), 0, nil, base, entry); !errors.Is(err, flow.ErrNotFound) {
			t.Errorf("missing flow = %v, want ErrNotFound", err)
		}
	})
}
