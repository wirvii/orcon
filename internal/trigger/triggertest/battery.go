// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package triggertest

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/trigger"
)

// Battery runs every scenario against backends from f. Each scenario is also
// exported on its own, so a test file can give it the name a criterion cites.
func Battery(t *testing.T, f Factory) {
	t.Helper()
	cases := []struct {
		name string
		run  func(*testing.T, Factory)
	}{
		{"PublishActivateDeactivateRollback", PublishActivateDeactivateRollback},
		{"UnpublishedChanges", UnpublishedChanges},
		{"TriggerValidation", TriggerValidation},
		{"ConfigurationHashKeepsState", ConfigurationHashKeepsState},
		{"DeletedFlowDeactivatesTriggers", DeletedFlowDeactivatesTriggers},
		{"RestoredFlowStaysInactive", RestoredFlowStaysInactive},
		{"PurgedFlowRemovesTriggers", PurgedFlowRemovesTriggers},
		{"ManualRun", ManualRun},
		{"Isolation", Isolation},
		{"ConcurrentPublish", ConcurrentPublish},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { c.run(t, f) })
	}
}

// PublishActivateDeactivateRollback proves the lifecycle of ADR-0024: saving
// the draft leaves production alone, publishing creates version N+1 and makes
// it live only when the flow is active, activating and deactivating create no
// version, rolling back republishes an earlier version as N+1, and an alias
// takes effect only when published (E-ACT-1).
func PublishActivateDeactivateRollback(t *testing.T, f Factory) {
	t.Helper()
	l := &lifecycle{t: t, e: newEnv(t, f, trigger.Settings{})}
	l.ctx = l.e.ctx()
	l.id = l.e.newFlow(l.e.orgA, l.e.projA, "orders", Build(DocSpec{Name: "orders v1", Starts: []Node{{ID: "t", Block: WebhookBlock("orders")}}}))
	wantErr(t, "activate before publishing", l.e.svc.Activate(l.ctx, l.e.orgA, l.id), trigger.ErrNotPublished)
	l.publishInactive()
	l.activate()
	l.savedDraftChangesNothing()
	l.publishWhileActive()
	p3 := l.aliasesTakeEffectOnPublish()
	l.rollback(&p3)
	l.deactivate()
	want := []string{
		trigger.EventFlowPublished, trigger.EventFlowActivated, trigger.EventFlowPublished, trigger.EventFlowPublished,
		trigger.EventFlowPublished, trigger.EventFlowRolledBack, trigger.EventFlowDeactivated,
	}
	if got := eventTypes(l.e.events(l.e.orgA, l.id)); !slices.Equal(got, want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
}

// lifecycle is the flow the lifecycle scenario walks through.
type lifecycle struct {
	t      *testing.T
	e      *env
	ctx    context.Context //nolint:containedctx // a test scenario holds its context for its whole duration
	id     flow.FlowID
	entry  trigger.ID
	first  flow.VersionRecord
	stable trigger.AddressID
}

func (l *lifecycle) accept() (trigger.Acceptance, error) {
	return l.e.svc.Accept(l.ctx, l.e.orgA, l.id, "t")
}

// publishInactive publishes a flow nobody activated: version 1 exists and the
// registry is built, but nothing accepts events.
func (l *lifecycle) publishInactive() {
	t, e := l.t, l.e
	p1 := e.publish(e.orgA, l.id)
	if p1.Version.Number != 1 || p1.Live {
		t.Fatalf("first publication = version %d live %v, want version 1 not live", p1.Version.Number, p1.Live)
	}
	l.first = p1.Version
	st, err := e.svc.Status(l.ctx, e.orgA, l.id)
	must(t, "status", err)
	if st.ActivationState != flow.ActivationInactive || st.ActiveVersion != 1 || st.HasUnpublishedChanges {
		t.Fatalf("status after publishing = %+v", st)
	}
	trs := st.Triggers
	if len(trs) != 1 || trs[0].Status != trigger.StatusInactive || trs[0].Family != flow.TriggerWebhook || trs[0].FlowVersionID != p1.Version.ID {
		t.Fatalf("registry after publishing = %+v", trs)
	}
	l.entry = trs[0].ID
	addrs := e.addresses(e.orgA, l.entry)
	if len(addrs) != 1 {
		t.Fatalf("addresses after publishing = %+v", addrs)
	}
	a := &addrs[0]
	if a.Pattern != "/hooks/alpha/orders" || a.Scope != trigger.ScopeProduction || a.Status != trigger.AddressActive ||
		!slices.Equal(a.MethodNames(), []string{"POST"}) || a.ResponseProfile != flow.ProfileNative || a.StaticPrefix != "/hooks/alpha/orders" {
		t.Fatalf("address after publishing = %+v", *a)
	}
	l.stable = a.ID
	acc, err := l.accept()
	wantErr(t, "accept on an inactive flow", err, trigger.ErrInactive)
	if acc.VersionNumber != 1 || acc.Accepting {
		t.Fatalf("acceptance on an inactive flow = %+v", acc)
	}
}

// activate switches the triggers on without creating a version.
func (l *lifecycle) activate() {
	t, e := l.t, l.e
	must(t, "activate", e.svc.Activate(l.ctx, e.orgA, l.id))
	must(t, "activate again", e.svc.Activate(l.ctx, e.orgA, l.id)) // idempotent
	if n := versionCount(t, e, l.id); n != 1 {
		t.Fatalf("activating created versions: %d, want 1", n)
	}
	acc, err := l.accept()
	must(t, "accept on an active flow", err)
	if acc.VersionNumber != 1 || acc.FlowVersionID != l.first.ID || !acc.Accepting {
		t.Fatalf("acceptance on an active flow = %+v", acc)
	}
}

// savedDraftChangesNothing saves a draft that moves the path and checks that
// production still answers on the old one.
func (l *lifecycle) savedDraftChangesNothing() {
	t, e := l.t, l.e
	e.saveDraft(e.orgA, l.id, Build(DocSpec{Name: "orders v2", Starts: []Node{{ID: "t", Block: WebhookBlock("orders-renamed")}}}))
	if got := e.addresses(e.orgA, l.entry); len(got) != 1 || got[0].Pattern != "/hooks/alpha/orders" {
		t.Fatalf("saving the draft changed the addresses: %+v", got)
	}
	if st, _ := e.svc.Status(l.ctx, e.orgA, l.id); !st.HasUnpublishedChanges {
		t.Fatal("a saved draft with changes is not reported as unpublished")
	}
}

// publishWhileActive publishes version 2 of an active flow: it is live at once
// and the address of an unchanged path keeps its identity (R-ACT-05).
func (l *lifecycle) publishWhileActive() {
	t, e := l.t, l.e
	e.saveDraft(e.orgA, l.id, Build(DocSpec{Name: "orders v2", Starts: []Node{{ID: "t", Block: WebhookBlock("orders")}}}))
	p2 := e.publish(e.orgA, l.id)
	if p2.Version.Number != 2 || !p2.Live {
		t.Fatalf("second publication = version %d live %v, want version 2 live", p2.Version.Number, p2.Live)
	}
	acc, err := l.accept()
	must(t, "accept after the second publication", err)
	if acc.VersionNumber != 2 {
		t.Fatalf("an event after publication is pinned to version %d, want 2", acc.VersionNumber)
	}
	after := e.addresses(e.orgA, l.entry)
	if len(after) != 1 || after[0].ID != l.stable {
		t.Fatalf("the address was not stable across versions: %+v", after)
	}
}

// aliasesTakeEffectOnPublish adds an n8n alias to the draft, checks it does not
// answer until published, then removes it and checks the address is retired.
// It returns the publication that carried the alias.
func (l *lifecycle) aliasesTakeEffectOnPublish() trigger.Published {
	t, e := l.t, l.e
	e.saveDraft(e.orgA, l.id, Build(DocSpec{Name: "orders v3", Starts: []Node{{ID: "t", Block: `{"family":"WEBHOOK","addresses":[{"family":"NATIVE","path":"orders"},{"family":"N8N","path":"orders-legacy","methods":["GET","POST"]}]}`}}}))
	if got := e.addresses(e.orgA, l.entry); len(got) != 1 {
		t.Fatalf("an alias in the draft already answers: %+v", got)
	}
	p3 := e.publish(e.orgA, l.id)
	byPattern := e.addressesByPattern(e.orgA, l.entry)
	legacy, ok := byPattern["/webhook/orders-legacy"]
	if !ok || len(byPattern) != 2 {
		t.Fatalf("addresses after publishing the alias = %+v", byPattern)
	}
	if legacy.Family != flow.AddressN8N || legacy.ResponseProfile != flow.ProfileN8N || !slices.Equal(legacy.MethodNames(), []string{"GET", "POST"}) {
		t.Fatalf("the n8n alias = %+v", *legacy)
	}
	e.saveDraft(e.orgA, l.id, Build(DocSpec{Name: "orders v4", Starts: []Node{{ID: "t", Block: WebhookBlock("orders")}}}))
	e.clock.Advance(time.Minute)
	e.publish(e.orgA, l.id)
	byPattern = e.addressesByPattern(e.orgA, l.entry)
	if r := byPattern["/webhook/orders-legacy"]; r == nil || r.Status != trigger.AddressRetired || r.RetiredAt.IsZero() {
		t.Fatalf("the removed alias = %+v, want retired with a retirement time", r)
	}
	if k := byPattern["/hooks/alpha/orders"]; k == nil || k.Status != trigger.AddressActive {
		t.Fatalf("the kept address = %+v", k)
	}
	return p3
}

// rollback republishes version 3 (the alias version) as version 5.
func (l *lifecycle) rollback(p3 *trigger.Published) {
	t, e := l.t, l.e
	rb, err := e.svc.Rollback(l.ctx, trigger.RollbackCommand{Org: e.orgA, Flow: l.id, By: e.user, ToVersion: p3.Version.Number})
	must(t, "rollback", err)
	if rb.Version.Number != 5 || rb.Version.ContentHash != p3.Version.ContentHash || !rb.Live {
		t.Fatalf("rollback = version %d hash %s live %v, want version 5 with the hash of version 3", rb.Version.Number, rb.Version.ContentHash, rb.Live)
	}
	st, _ := e.svc.Status(l.ctx, e.orgA, l.id)
	if st.HasUnpublishedChanges || st.ActiveVersion != 5 {
		t.Fatalf("status after rollback = %+v, want version 5 and no unpublished changes (the draft is the rolled back content)", st)
	}
	active := 0
	for _, a := range e.addressesByPattern(e.orgA, l.entry) {
		if a.Status == trigger.AddressActive {
			active++
		}
	}
	if active != 2 {
		t.Fatalf("active addresses after rollback = %d, want 2 (the alias is back)", active)
	}
}

// deactivate switches the triggers off without creating a version.
func (l *lifecycle) deactivate() {
	t, e := l.t, l.e
	must(t, "deactivate", e.svc.Deactivate(l.ctx, e.orgA, l.id))
	if n := versionCount(t, e, l.id); n != 5 {
		t.Fatalf("versions = %d, want 5", n)
	}
	_, err := l.accept()
	wantErr(t, "accept after deactivation", err, trigger.ErrInactive)
	if st, _ := e.svc.Status(l.ctx, e.orgA, l.id); st.ActivationState != flow.ActivationInactive {
		t.Fatalf("activation after deactivating = %s", st.ActivationState)
	}
}

func versionCount(t *testing.T, e *env, id flow.FlowID) int {
	t.Helper()
	vs, err := e.b.Flows.ListVersions(e.ctx(), e.orgA, id, 0, 100)
	must(t, "list versions", err)
	return len(vs)
}

// UnpublishedChanges proves that a flow reports unpublished changes exactly
// when the content hash of its draft differs from the live version (R-ACT-01).
func UnpublishedChanges(t *testing.T, f Factory) {
	t.Helper()
	e := newEnv(t, f, trigger.Settings{})
	ctx := e.ctx()
	doc := func(name string) []byte {
		return Build(DocSpec{Name: name, Starts: []Node{{ID: "t", Block: WebhookBlock("orders")}}})
	}
	id := e.newFlow(e.orgA, e.projA, "orders", doc("one"))
	check := func(step string, want bool) {
		t.Helper()
		st, err := e.svc.Status(ctx, e.orgA, id)
		must(t, step, err)
		if st.HasUnpublishedChanges != want {
			t.Fatalf("%s: HasUnpublishedChanges = %v, want %v", step, st.HasUnpublishedChanges, want)
		}
	}
	check("a draft nobody published", true)
	e.publish(e.orgA, id)
	check("right after publishing", false)
	e.saveDraft(e.orgA, id, doc("two"))
	check("after editing the draft", true)
	e.publish(e.orgA, id)
	check("after publishing the edit", false)
	// Editor data is not content: moving a node does not make the draft differ.
	moved := Build(DocSpec{Name: "two", Starts: []Node{{ID: "t", Block: WebhookBlock("orders")}}})
	moved = []byte(string(moved[:len(moved)-len(`"editor":{}}`)]) + `"editor":{"positions":{"t":{"x":10,"y":20}}}}`)
	e.saveDraft(e.orgA, id, moved)
	check("after moving a node", false)
	e.saveDraft(e.orgA, id, doc("three"))
	check("after another edit", true)
	_, err := e.svc.Rollback(ctx, trigger.RollbackCommand{Org: e.orgA, Flow: id, By: e.user, ToVersion: 1})
	must(t, "rollback", err)
	check("after rolling back", false)
}

// RecoveryPlusADay is a little more than the recovery window of a deleted flow.
const RecoveryPlusADay = trigger.RecoveryWindow + 24*time.Hour

// ConfigurationHashKeepsState proves ADR-0024 section 3: a publication keeps
// the registry entry and the addresses of a trigger whose configuration did
// not change, resets the health of one that did, retires the entry of a node
// that left the flow, and brings it back when the node returns.
func ConfigurationHashKeepsState(t *testing.T, f Factory) {
	t.Helper()
	e := newEnv(t, f, trigger.Settings{})
	ctx := e.ctx()
	schedule := func(every int) string {
		return `{"family":"SCHEDULE","schedule":{"rules":[{"kind":"MINUTES","every":` + strconv.Itoa(every) + `}]}}`
	}
	build := func(name string, nodes ...Node) []byte { return Build(DocSpec{Name: name, Starts: nodes}) }
	hook := Node{ID: "hook", Block: WebhookBlock("kept")}
	cron := func(every int) Node { return Node{ID: "cron", Type: TypeSchedule, Block: schedule(every)} }
	id := e.newFlow(e.orgA, e.projA, "stateful", build("v1", hook, cron(5)))
	e.publish(e.orgA, id)
	must(t, "activate", e.svc.Activate(ctx, e.orgA, id))

	byNode := func() map[flow.NodeID]trigger.Trigger {
		m := map[flow.NodeID]trigger.Trigger{}
		trs := e.triggers(e.orgA, id)
		for i := range trs {
			m[trs[i].NodeID] = trs[i]
		}
		return m
	}
	v1 := byNode()
	if len(v1) != 2 || v1["hook"].Status != trigger.StatusActive || v1["cron"].Family != flow.TriggerSchedule {
		t.Fatalf("registry after v1 = %+v", v1)
	}

	// v2 edits the flow name and changes the schedule, not the webhook.
	e.saveDraft(e.orgA, id, build("v2", hook, cron(7)))
	p2 := e.publish(e.orgA, id)
	v2 := byNode()
	if v2["hook"].ID != v1["hook"].ID || v2["hook"].ConfigurationHash != v1["hook"].ConfigurationHash {
		t.Fatalf("an unchanged trigger lost its identity or hash: %+v then %+v", v1["hook"], v2["hook"])
	}
	if v2["hook"].FlowVersionID != p2.Version.ID || v2["cron"].FlowVersionID != p2.Version.ID {
		t.Fatalf("entries do not point at version 2: %+v", v2)
	}
	if v2["cron"].ID != v1["cron"].ID || v2["cron"].ConfigurationHash == v1["cron"].ConfigurationHash {
		t.Fatalf("a changed trigger kept its hash or lost its identity: %+v then %+v", v1["cron"], v2["cron"])
	}
	if got := e.addresses(e.orgA, v2["hook"].ID); len(got) != 1 || got[0].ID != e.addresses(e.orgA, v1["hook"].ID)[0].ID {
		t.Fatalf("the address of the unchanged trigger was rebuilt: %+v", got)
	}

	// v3 drops the webhook: its entry is removed and its address retired.
	e.clock.Advance(time.Hour)
	e.saveDraft(e.orgA, id, build("v3", cron(7)))
	e.publish(e.orgA, id)
	v3 := byNode()
	if v3["hook"].Status != trigger.StatusRemoved {
		t.Fatalf("the entry of a removed node = %+v, want REMOVED", v3["hook"])
	}
	if a := e.addresses(e.orgA, v3["hook"].ID); len(a) != 1 || a[0].Status != trigger.AddressRetired {
		t.Fatalf("the address of a removed node = %+v, want retired", a)
	}
	_, err := e.svc.Accept(ctx, e.orgA, id, "hook")
	wantErr(t, "accept on a removed trigger", err, trigger.ErrInactive)

	// v4 brings the webhook back: same entry, same address, active again.
	e.saveDraft(e.orgA, id, build("v4", hook, cron(7)))
	e.publish(e.orgA, id)
	v4 := byNode()
	if v4["hook"].ID != v1["hook"].ID || v4["hook"].Status != trigger.StatusActive {
		t.Fatalf("a returning node = %+v, want the same entry active", v4["hook"])
	}
	a := e.addresses(e.orgA, v4["hook"].ID)
	if len(a) != 1 || a[0].ID != e.addresses(e.orgA, v1["hook"].ID)[0].ID || a[0].Status != trigger.AddressActive || !a[0].RetiredAt.IsZero() {
		t.Fatalf("the address of a returning node = %+v, want the original, active", a)
	}
	if _, err := e.svc.Accept(ctx, e.orgA, id, "hook"); err != nil {
		t.Fatalf("accept on the returned trigger: %v", err)
	}
}

// DeletedFlowDeactivatesTriggers proves R-ACT-09 and ADR-0024 section 8:
// deleting an active flow switches all its triggers off in the same
// transaction, refuses new events with the reason INACTIVE, and leaves the
// versions that running executions are pinned to (E-ACT-4).
func DeletedFlowDeactivatesTriggers(t *testing.T, f Factory) {
	t.Helper()
	e := newEnv(t, f, trigger.Settings{})
	ctx := e.ctx()
	id := e.newFlow(e.orgA, e.projA, "doomed", Build(DocSpec{Name: "a", Starts: []Node{
		{ID: "hook", Block: WebhookBlock("doomed")}, {ID: "cron", Type: TypeSchedule, Block: `{"family":"SCHEDULE","schedule":{"rules":[{"kind":"HOURS"}]}}`}}}))
	p := e.publish(e.orgA, id)
	must(t, "activate", e.svc.Activate(ctx, e.orgA, id))
	if _, err := e.svc.Accept(ctx, e.orgA, id, "hook"); err != nil {
		t.Fatalf("accept before the delete: %v", err)
	}

	must(t, "delete", e.svc.Delete(ctx, e.orgA, id, e.user))

	for _, node := range []flow.NodeID{"hook", "cron"} {
		acc, err := e.svc.Accept(ctx, e.orgA, id, node)
		wantErr(t, "accept on a deleted flow ("+string(node)+")", err, trigger.ErrInactive)
		if acc.Accepting || acc.VersionNumber != 1 {
			t.Fatalf("acceptance on a deleted flow = %+v", acc)
		}
	}
	trs := e.triggersOfDeleted(e.orgA, id)
	for i := range trs {
		if trs[i].Status != trigger.StatusInactive {
			t.Fatalf("trigger %s of a deleted flow is %s, want INACTIVE", trs[i].NodeID, trs[i].Status)
		}
	}
	_, err := e.svc.Status(ctx, e.orgA, id)
	wantErr(t, "status of a deleted flow", err, flow.ErrNotFound)
	wantErr(t, "delete twice", e.svc.Delete(ctx, e.orgA, id, e.user), flow.ErrNotFound)
	wantErr(t, "activate a deleted flow", e.svc.Activate(ctx, e.orgA, id), flow.ErrNotFound)
	_, err = e.svc.Publish(ctx, trigger.PublishCommand{Org: e.orgA, Flow: id, By: e.user})
	wantErr(t, "publish a deleted flow", err, flow.ErrNotFound)

	// The version a running execution is pinned to is still there.
	v, err := e.b.Flows.GetVersion(ctx, e.orgA, p.Version.ID)
	must(t, "read the pinned version", err)
	if v.Number != 1 {
		t.Fatalf("pinned version = %+v", v)
	}
	types := eventTypes(e.eventsOfDeleted(e.orgA, id))
	if want := []string{trigger.EventFlowPublished, trigger.EventFlowActivated, trigger.EventFlowDeactivated}; !slices.Equal(types, want) {
		t.Fatalf("events = %v, want %v", types, want)
	}
}

// triggersOfDeleted reads the registry of a flow that Status no longer shows.
func (e *env) triggersOfDeleted(org identity.OrgID, id flow.FlowID) []trigger.Trigger {
	e.t.Helper()
	var out []trigger.Trigger
	must(e.t, "list triggers", e.b.Triggers.Read(e.ctx(), org, func(ctx context.Context, tx trigger.Tx) error {
		var err error
		out, err = tx.ListTriggers(ctx, id)
		return err
	}))
	return out
}

func (e *env) eventsOfDeleted(org identity.OrgID, id flow.FlowID) []trigger.Event {
	return e.events(org, id)
}

// RestoredFlowStaysInactive proves that restoring a deleted flow within its
// 30 days brings it back inactive with inactive triggers, that a person must
// activate it, and that a restore after the window or over a taken name fails
// (E-ACT-4).
func RestoredFlowStaysInactive(t *testing.T, f Factory) {
	t.Helper()
	e := newEnv(t, f, trigger.Settings{})
	ctx := e.ctx()
	doc := Build(DocSpec{Name: "a", Starts: []Node{{ID: "hook", Block: WebhookBlock("phoenix")}}})
	id := e.newFlow(e.orgA, e.projA, "phoenix", doc)
	e.publish(e.orgA, id)
	must(t, "activate", e.svc.Activate(ctx, e.orgA, id))
	must(t, "delete", e.svc.Delete(ctx, e.orgA, id, e.user))

	e.clock.Advance(29 * 24 * time.Hour)
	must(t, "restore", e.svc.Restore(ctx, e.orgA, id))

	st, err := e.svc.Status(ctx, e.orgA, id)
	must(t, "status after restore", err)
	if st.ActivationState != flow.ActivationInactive {
		t.Fatalf("a restored flow is %s, want INACTIVE", st.ActivationState)
	}
	for i := range st.Triggers {
		if st.Triggers[i].Status != trigger.StatusInactive {
			t.Fatalf("restored trigger %s is %s, want INACTIVE", st.Triggers[i].NodeID, st.Triggers[i].Status)
		}
	}
	_, err = e.svc.Accept(ctx, e.orgA, id, "hook")
	wantErr(t, "accept after the restore", err, trigger.ErrInactive)
	must(t, "explicit activation", e.svc.Activate(ctx, e.orgA, id))
	if _, err := e.svc.Accept(ctx, e.orgA, id, "hook"); err != nil {
		t.Fatalf("accept after the explicit activation: %v", err)
	}
	wantErr(t, "restore a flow that is not deleted", e.svc.Restore(ctx, e.orgA, id), trigger.ErrNotDeleted)

	// A name taken meanwhile blocks the restore and changes nothing.
	must(t, "delete again", e.svc.Delete(ctx, e.orgA, id, e.user))
	e.newFlow(e.orgA, e.projA, "phoenix", doc)
	wantErr(t, "restore over a taken name", e.svc.Restore(ctx, e.orgA, id), flow.ErrConflict)
	_, err = e.svc.Status(ctx, e.orgA, id)
	wantErr(t, "the flow stays deleted", err, flow.ErrNotFound)

	// Past the window the flow cannot be restored.
	e.clock.Advance(RecoveryPlusADay)
	wantErr(t, "restore after the window", e.svc.Restore(ctx, e.orgA, id), trigger.ErrRecoveryExpired)
}

// PurgedFlowRemovesTriggers proves that purging a flow after its window leaves
// no row of it and frees its paths, and that purging earlier is refused
// (E-ACT-4).
func PurgedFlowRemovesTriggers(t *testing.T, f Factory) {
	t.Helper()
	e := newEnv(t, f, trigger.Settings{})
	ctx := e.ctx()
	id := e.newFlow(e.orgA, e.projA, "gone", Build(DocSpec{Name: "a", Starts: []Node{{ID: "hook", Block: WebhookBlock("gone")}}}))
	p := e.publish(e.orgA, id)
	must(t, "activate", e.svc.Activate(ctx, e.orgA, id))

	wantErr(t, "purge a flow that is not deleted", e.svc.Purge(ctx, e.orgA, id), trigger.ErrNotDeleted)
	must(t, "delete", e.svc.Delete(ctx, e.orgA, id, e.user))
	e.clock.Advance(29 * 24 * time.Hour)
	wantErr(t, "purge inside the window", e.svc.Purge(ctx, e.orgA, id), trigger.ErrRecoveryWindow)
	if len(e.triggersOfDeleted(e.orgA, id)) != 1 {
		t.Fatal("a refused purge removed trigger rows")
	}

	// While the flow can still be restored its path stays reserved.
	rival := e.newFlow(e.orgA, e.projA, "rival", Build(DocSpec{Name: "b", Starts: []Node{{ID: "hook", Block: WebhookBlock("gone")}}}))
	_, err := e.svc.Publish(ctx, trigger.PublishCommand{Org: e.orgA, Flow: rival, By: e.user})
	validationIssue(t, "claim the path of a deleted flow", err, trigger.CodeTV1)

	e.clock.Advance(2 * 24 * time.Hour)
	must(t, "purge", e.svc.Purge(ctx, e.orgA, id))

	if trs := e.triggersOfDeleted(e.orgA, id); len(trs) != 0 {
		t.Fatalf("trigger rows after the purge: %+v", trs)
	}
	if evs := e.eventsOfDeleted(e.orgA, id); len(evs) != 0 {
		t.Fatalf("trigger events after the purge: %d", len(evs))
	}
	_, err = e.b.Flows.GetVersion(ctx, e.orgA, p.Version.ID)
	wantErr(t, "the version after the purge", err, flow.ErrVersionNotFound)
	_, err = e.svc.Accept(ctx, e.orgA, id, "hook")
	wantErr(t, "accept after the purge", err, flow.ErrNotFound)
	wantErr(t, "purge twice", e.svc.Purge(ctx, e.orgA, id), flow.ErrNotFound)

	// The path is free again.
	if _, err := e.svc.Publish(ctx, trigger.PublishCommand{Org: e.orgA, Flow: rival, By: e.user}); err != nil {
		t.Fatalf("claim the path after the purge: %v", err)
	}
}

// ManualRun proves the manual trigger: a run of the draft executes a snapshot
// shared by content, a run of the published version pins that version, both in
// mode MANUAL and independent of activation, and an invalid draft or a
// missing publication is refused (R-INT-01).
func ManualRun(t *testing.T, f Factory) {
	t.Helper()
	e := newEnv(t, f, trigger.Settings{})
	ctx := e.ctx()
	manual := Node{ID: "go", Type: TypeManual, Block: `{"family":"INTERNAL"}`}
	id := e.newFlow(e.orgA, e.projA, "manual", Build(DocSpec{Name: "a", Starts: []Node{manual}}))

	_, err := e.svc.PrepareManualRun(ctx, trigger.ManualCommand{Org: e.orgA, Flow: id, Source: trigger.ManualPublished})
	wantErr(t, "run an unpublished flow", err, trigger.ErrNotPublished)

	draft, err := e.svc.PrepareManualRun(ctx, trigger.ManualCommand{Org: e.orgA, Flow: id, Source: trigger.ManualDraft})
	must(t, "run the draft", err)
	if draft.Mode != trigger.ModeManual || draft.Node != "go" || draft.Snapshot == (flow.FlowSnapshotID{}) {
		t.Fatalf("draft run = %+v", draft)
	}
	if _, isSnapshot := draft.Revision.Snapshot(); !isSnapshot {
		t.Fatalf("draft run revision = %+v, want a snapshot", draft.Revision)
	}
	again, err := e.svc.PrepareManualRun(ctx, trigger.ManualCommand{Org: e.orgA, Flow: id, Source: trigger.ManualDraft})
	must(t, "run the draft again", err)
	if again.Snapshot != draft.Snapshot {
		t.Fatal("the same draft content produced two snapshots")
	}

	e.publish(e.orgA, id)
	pub, err := e.svc.PrepareManualRun(ctx, trigger.ManualCommand{Org: e.orgA, Flow: id, Source: trigger.ManualPublished})
	must(t, "run the published version", err)
	if n, ok := pub.Revision.Version(); !ok || n != 1 || pub.Version == (flow.FlowVersionID{}) {
		t.Fatalf("published run = %+v", pub)
	}
	if st, _ := e.svc.Status(ctx, e.orgA, id); st.ActivationState != flow.ActivationInactive {
		t.Fatal("a manual run needs no activation, and preparing one must not activate")
	}

	execID, err := e.svc.RunManually(ctx, trigger.ManualCommand{Org: e.orgA, Flow: id, Source: trigger.ManualPublished}, []byte(`{"n":1}`))
	must(t, "run manually", err)
	if execID == "" || len(e.starter.Runs) != 1 || string(e.starter.Runs[0].Input) != `{"n":1}` || e.starter.Runs[0].Run.Mode != trigger.ModeManual {
		t.Fatalf("starter calls = %+v (execution %q)", e.starter.Runs, execID)
	}

	// An invalid draft is refused.
	e.saveDraft(e.orgA, id, []byte(`{"format":"orcon/flow@1","id":"01930000-0000-7000-8000-0000000000aa","name":"bad","settings":{},"variables":[],"nodes":[{"id":"go","name":"go","element":"startEvent","type":"orcon.manual@1","parameters":{}}],"edges":[{"id":"e","from":{"node":"go","port":"main"},"to":{"node":"nowhere"},"kind":"SEQUENCE"}],"editor":{}}`))
	_, err = e.svc.PrepareManualRun(ctx, trigger.ManualCommand{Org: e.orgA, Flow: id, Source: trigger.ManualDraft})
	var ve *flow.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("run an invalid draft: %v, want a validation error", err)
	}

	// Several start events need a choice.
	two := e.newFlow(e.orgA, e.projA, "two starts", Build(DocSpec{Name: "two", Starts: []Node{manual, {ID: "other", Type: TypeManual, Block: `{"family":"INTERNAL"}`}}}))
	_, err = e.svc.PrepareManualRun(ctx, trigger.ManualCommand{Org: e.orgA, Flow: two, Source: trigger.ManualDraft})
	wantErr(t, "run without choosing a start event", err, trigger.ErrNoStartNode)
	chosen, err := e.svc.PrepareManualRun(ctx, trigger.ManualCommand{Org: e.orgA, Flow: two, Source: trigger.ManualDraft, Node: "other"})
	must(t, "run the chosen start event", err)
	if chosen.Node != "other" {
		t.Fatalf("chosen node = %s", chosen.Node)
	}
	_, err = e.svc.PrepareManualRun(ctx, trigger.ManualCommand{Org: e.orgA, Flow: two, Source: trigger.ManualDraft, Node: "end"})
	wantErr(t, "run from a node that is not a start event", err, trigger.ErrNoStartNode)
}

// Isolation proves that one organization can neither read nor change the
// lifecycle of another's flows (ADR-0046).
func Isolation(t *testing.T, f Factory) {
	t.Helper()
	e := newEnv(t, f, trigger.Settings{})
	ctx := e.ctx()
	id := e.newFlow(e.orgA, e.projA, "private", Build(DocSpec{Name: "a", Starts: []Node{{ID: "hook", Block: WebhookBlock("private")}}}))
	e.publish(e.orgA, id)
	must(t, "activate", e.svc.Activate(ctx, e.orgA, id))

	_, err := e.svc.Status(ctx, e.orgB, id)
	wantErr(t, "status from another organization", err, flow.ErrNotFound)
	_, err = e.svc.Publish(ctx, trigger.PublishCommand{Org: e.orgB, Flow: id, By: e.user})
	wantErr(t, "publish from another organization", err, flow.ErrNotFound)
	wantErr(t, "deactivate from another organization", e.svc.Deactivate(ctx, e.orgB, id), flow.ErrNotFound)
	wantErr(t, "delete from another organization", e.svc.Delete(ctx, e.orgB, id, e.user), flow.ErrNotFound)
	_, err = e.svc.Accept(ctx, e.orgB, id, "hook")
	wantErr(t, "accept from another organization", err, flow.ErrNotFound)
	_, err = e.svc.PrepareManualRun(ctx, trigger.ManualCommand{Org: e.orgB, Flow: id, Source: trigger.ManualDraft})
	wantErr(t, "manual run from another organization", err, flow.ErrNotFound)
	if trs := e.triggersOfDeleted(e.orgB, id); len(trs) != 0 {
		t.Fatalf("another organization lists %d triggers of this flow", len(trs))
	}
	if evs := e.events(e.orgB, id); len(evs) != 0 {
		t.Fatalf("another organization lists %d events of this flow", len(evs))
	}
	if st, err := e.svc.Status(ctx, e.orgA, id); err != nil || st.ActivationState != flow.ActivationActive {
		t.Fatalf("the owner's flow changed: %+v, %v", st, err)
	}
}

// ConcurrentPublish proves that publications racing on one flow produce
// distinct, consecutive version numbers and leave the registry pointing at the
// flow's live version.
func ConcurrentPublish(t *testing.T, f Factory) {
	t.Helper()
	e := newEnv(t, f, trigger.Settings{})
	ctx := e.ctx()
	id := e.newFlow(e.orgA, e.projA, "racy", Build(DocSpec{Name: "a", Starts: []Node{{ID: "hook", Block: WebhookBlock("racy")}}}))
	const workers = 6
	var wg sync.WaitGroup
	results := make([]error, workers)
	for i := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, results[i] = e.svc.Publish(ctx, trigger.PublishCommand{Org: e.orgA, Flow: id, By: e.user})
		}()
	}
	wg.Wait()
	published := 0
	for i, err := range results {
		switch {
		case err == nil:
			published++
		case errors.Is(err, flow.ErrConflict) || errors.Is(err, flow.ErrDraftConflict):
			// Losing a race is allowed; leaving a half-published flow is not.
		default:
			t.Fatalf("publication %d: %v", i, err)
		}
	}
	if published == 0 {
		t.Fatal("no publication succeeded")
	}
	vs, err := e.b.Flows.ListVersions(ctx, e.orgA, id, 0, 100)
	must(t, "list versions", err)
	if len(vs) != published {
		t.Fatalf("%d publications succeeded but %d versions exist", published, len(vs))
	}
	for i := range vs {
		if vs[i].Number != int64(i+1) {
			t.Fatalf("version numbers = %v, want 1..%d", numbers(vs), published)
		}
	}
	rec, err := e.b.Flows.GetFlow(ctx, e.orgA, id)
	must(t, "read the flow", err)
	live := vs[len(vs)-1]
	if rec.ActiveVersionID != live.ID {
		t.Fatalf("the flow points at %s, want its last version %s", rec.ActiveVersionID, live.ID)
	}
	trs := e.triggers(e.orgA, id)
	if len(trs) != 1 || trs[0].FlowVersionID != live.ID {
		t.Fatalf("the registry does not point at the live version: %+v", trs)
	}
}

func numbers(vs []flow.VersionRecord) []int64 {
	out := make([]int64, len(vs))
	for i := range vs {
		out[i] = vs[i].Number
	}
	return out
}
