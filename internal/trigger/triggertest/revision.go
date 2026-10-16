// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package triggertest

import (
	"testing"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/idempotencyclaim"
	"github.com/wirvii/orcon/internal/trigger"
)

// FlowChangesAreRevisionedAndAtomic proves the rules of SPEC-005 Amendment 8 sections 8-9 for the
// lifecycle: activation, deactivation, rollback and deletion take the revision of the flow row and
// refuse a stale one without changing anything; publication takes the revision of the draft and
// records its note; every change moves the revision of the row by at least one; and a change
// commits together with the idempotency claim of its request, so a refused claim undoes it.
func FlowChangesAreRevisionedAndAtomic(t *testing.T, f Factory) {
	t.Helper()
	e := newEnv(t, f, trigger.Settings{})
	ctx := e.ctx()
	flowID := e.newFlow(e.scopeA, e.projA, "orders", Build(DocSpec{Name: "orders v1", Starts: []Node{{ID: "t", Block: WebhookBlock("orders")}}}))
	read := func() flow.Record {
		t.Helper()
		rec, err := e.b.Flows.GetFlow(ctx, e.scopeA, flowID)
		must(t, "read the flow", err)
		return rec
	}
	at := func(n int64) *int64 { return &n }
	change := func(rev int64) *trigger.ChangeCommand {
		return &trigger.ChangeCommand{Org: e.scopeA, Flow: flowID, ExpectedRevision: at(rev)}
	}
	if got := read(); got.Revision != 0 || got.ActiveVersion != 0 {
		t.Fatalf("a new flow = revision %d, active version %d; want 0 and 0", got.Revision, got.ActiveVersion)
	}

	// Publication takes the revision of the draft, records the note and moves the revision of the row.
	_, err := e.svc.Publish(ctx, trigger.PublishCommand{Org: e.scopeA, Flow: flowID, By: e.user, ExpectedDraftRevision: at(7)})
	wantErr(t, "publish at a stale draft revision", err, flow.ErrDraftConflict)
	if n := versionCount(t, e, flowID); n != 0 {
		t.Fatalf("a refused publication created %d versions", n)
	}
	first, err := e.svc.Publish(ctx, trigger.PublishCommand{Org: e.scopeA, Flow: flowID, By: e.user, ExpectedDraftRevision: at(0), Note: "first release"})
	must(t, "publish", err)
	if first.Version.Note != "first release" || first.Version.PublishedByPrincipalID != e.user {
		t.Fatalf("published version = %+v, want the note and the publisher", first.Version)
	}
	stored, err := e.b.Flows.GetVersionByNumber(ctx, e.scopeA, flowID, 1)
	must(t, "read version 1", err)
	if stored.Note != "first release" || stored.RolledBackFromVersion != 0 || stored.PublishedByPrincipalID != e.user {
		t.Fatalf("stored version 1 = %+v, want the note, no rollback origin and the publisher", stored)
	}
	if got := read(); got.Revision != 1 || got.ActiveVersion != 1 {
		t.Fatalf("after publishing = revision %d, active version %d; want 1 and 1", got.Revision, got.ActiveVersion)
	}

	// Activation takes the revision of the row; a stale one changes nothing.
	wantErr(t, "activate at a stale revision", e.svc.ActivateFlow(ctx, change(0)), flow.ErrRevisionConflict)
	if got := read(); got.ActivationState != flow.ActivationInactive || got.Revision != 1 {
		t.Fatalf("a refused activation changed the flow: %+v", got)
	}
	must(t, "activate", e.svc.ActivateFlow(ctx, change(1)))
	active := read()
	if active.ActivationState != flow.ActivationActive || active.Revision <= 1 {
		t.Fatalf("after activating = %+v, want active with a higher revision", active)
	}
	must(t, "activate again", e.svc.ActivateFlow(ctx, change(active.Revision)))
	if got := read(); got.Revision != active.Revision {
		t.Fatalf("activating an active flow moved the revision from %d to %d", active.Revision, got.Revision)
	}

	// Rollback takes the revision of the row and records its origin and note.
	second, err := e.svc.Publish(ctx, trigger.PublishCommand{Org: e.scopeA, Flow: flowID, By: e.user, ExpectedDraftRevision: at(0)})
	must(t, "publish again", err)
	if second.Version.Number != 2 {
		t.Fatalf("second publication = version %d", second.Version.Number)
	}
	current := read()
	_, err = e.svc.Rollback(ctx, trigger.RollbackCommand{Org: e.scopeA, Flow: flowID, By: e.user, ToVersion: 1, ExpectedRevision: at(current.Revision - 1)})
	wantErr(t, "roll back at a stale revision", err, flow.ErrRevisionConflict)
	if n := versionCount(t, e, flowID); n != 2 {
		t.Fatalf("a refused rollback left %d versions, want 2", n)
	}
	third, err := e.svc.Rollback(ctx, trigger.RollbackCommand{Org: e.scopeA, Flow: flowID, By: e.user, ToVersion: 1, ExpectedRevision: at(current.Revision), Note: "back to 1"})
	must(t, "roll back", err)
	rolled, err := e.b.Flows.GetVersionByNumber(ctx, e.scopeA, flowID, third.Version.Number)
	must(t, "read the rollback version", err)
	if rolled.Number != 3 || rolled.RolledBackFromVersion != 1 || rolled.Note != "back to 1" {
		t.Fatalf("rollback version = %+v, want version 3 from version 1 with its note", rolled)
	}
	_, err = e.svc.Rollback(ctx, trigger.RollbackCommand{Org: e.scopeA, Flow: flowID, By: e.user, ToVersion: 9})
	wantErr(t, "roll back to a version that does not exist", err, flow.ErrVersionNotFound)

	// A change commits with the claim of its request, and a refused claim undoes the change.
	claim := func(value string) idempotencyclaim.Claim {
		return idempotencyclaim.Claim{ID: e.ids.New(), Requester: "PRINCIPAL:" + e.user.String(), Value: value, Fingerprint: [32]byte{1}}
	}
	live := read()
	must(t, "deactivate with a claim", e.svc.DeactivateFlow(idempotencyclaim.With(ctx, claim("key-1")), change(live.Revision)))
	deactivated := read()
	if deactivated.ActivationState != flow.ActivationInactive {
		t.Fatalf("the flow is %s, want inactive", deactivated.ActivationState)
	}
	wantErr(t, "activate with a claim that is taken",
		e.svc.ActivateFlow(idempotencyclaim.With(ctx, claim("key-1")), change(deactivated.Revision)), idempotencyclaim.ErrTaken)
	if got := read(); got.ActivationState != flow.ActivationInactive || got.Revision != deactivated.Revision {
		t.Fatalf("a refused claim left the flow changed: %+v", got)
	}

	// Deletion takes the revision of the row too, and another organization cannot reach the flow.
	wantErr(t, "delete from another organization", e.svc.DeleteFlow(ctx, &trigger.ChangeCommand{Org: e.scopeB, Flow: flowID, ExpectedRevision: at(deactivated.Revision)}), flow.ErrNotFound)
	wantErr(t, "delete at a stale revision", e.svc.DeleteFlow(ctx, change(deactivated.Revision-1)), flow.ErrRevisionConflict)
	if _, err := e.b.Flows.GetFlow(ctx, e.scopeA, flowID); err != nil {
		t.Fatalf("a refused deletion removed the flow: %v", err)
	}
	must(t, "delete", e.svc.DeleteFlow(ctx, &trigger.ChangeCommand{Org: e.scopeA, Flow: flowID, ExpectedRevision: at(deactivated.Revision)}))
	if _, err := e.b.Flows.GetFlow(ctx, e.scopeA, flowID); err == nil {
		t.Fatal("the deleted flow is still readable")
	}
}
