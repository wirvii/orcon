// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package triggertest

import (
	"testing"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/trigger"
)

// TriggerValidation proves that every rule T-V1..T-V9 has a case that blocks
// publication (or only warns, T-V5) and that a blocked publication leaves no
// version and no registry change behind (E-ACT-1).
func TriggerValidation(t *testing.T, f Factory) {
	t.Helper()
	cases := blockedCases()
	for i := range cases {
		c := &cases[i]
		t.Run(c.name, func(t *testing.T) { runBlocked(t, f, c) })
	}
	scenarios := []struct {
		name string
		run  func(*testing.T, Factory)
	}{
		{"T-V9 the bounds themselves are allowed", boundsAreAllowed},
		{"T-V1 a path of another flow of the organization names that flow", collisionInOrganization},
		{"T-V1 a path of another organization is a generic problem", collisionAcrossOrganizations},
		{"T-V1 overlapping patterns only warn", overlapWarns},
		{"T-V3 a trigger that needs a missing credential", missingCredential},
		{"T-V4 an imported n8n address stays unless the author confirms", importedAddress},
		{"T-V4 deleting a trigger node that owns an imported address needs the confirmation", importedAddressNodeRemoved},
		{"T-V5 a respond node that cannot be reached only warns", unreachableRespondNode},
		{"T-V6 a local file inside the allowed paths publishes", localFileInside},
		{"a dry run reports what publication would", dryRun},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) { s.run(t, f) })
	}
}

// blockedCase is a document that publication must refuse with one rule.
type blockedCase struct {
	name     string
	settings trigger.Settings
	spec     DocSpec
	code     string
	contains []string
}

func single(nodeType, block string) DocSpec {
	return DocSpec{Name: "x", Starts: []Node{{ID: "t", Type: nodeType, Block: block}}}
}

func blockedCases() []blockedCase {
	localFiles := trigger.Settings{LocalFileEnabled: true, LocalFileAllowedPaths: []string{"/srv/inbox"}}
	listenAt := func(path string) string { return `{"family":"LISTEN","listen":{"localPath":"` + path + `"}}` }
	schedule := func(body string) string { return `{"family":"SCHEDULE","schedule":{` + body + `}}` }
	dedupe := func(window string) string {
		return WebhookBlock("a", `"dedupe":{"source":"HEADER","name":"X-Id","window":"`+window+`"}`)
	}
	return []blockedCase{
		{name: "T-V2 unknown time zone", code: trigger.CodeTV2,
			spec: single(TypeSchedule, schedule(`"timeZone":"Mars/Olympus","rules":[{"kind":"HOURS","every":1}]`))},
		{name: "T-V2 the Local zone", code: trigger.CodeTV2,
			spec: single(TypeSchedule, schedule(`"timeZone":"Local","rules":[{"kind":"HOURS"}]`))},
		{name: "T-V2 cron with an hour out of range", code: trigger.CodeTV2,
			spec: single(TypeSchedule, schedule(`"rules":[{"kind":"CRON","cron":"0 25 * * *"}]`))},
		{name: "T-V2 weekly rule without weekdays", code: trigger.CodeTV2,
			spec: single(TypeSchedule, schedule(`"rules":[{"kind":"WEEKS","at":"08:00"}]`))},
		{name: "T-V2 poll rule with a time of day on an interval", code: trigger.CodeTV2,
			spec: single(TypePoll, `{"family":"POLL","poll":{"rules":[{"kind":"MINUTES","every":5,"at":"08:00"}]}}`)},
		{name: "T-V6 local file disabled", code: trigger.CodeTV6, contains: []string{"disabled"},
			spec: single(TypeLocalFile, listenAt("/srv/inbox/a"))},
		{name: "T-V6 local file outside the allowed paths", code: trigger.CodeTV6, contains: []string{"outside"}, settings: localFiles,
			spec: single(TypeLocalFile, listenAt("/etc/passwd"))},
		{name: "T-V6 local file that climbs out", code: trigger.CodeTV6, settings: localFiles,
			spec: single(TypeLocalFile, listenAt("/srv/inbox/../secret"))},
		{name: "T-V6 sibling directory with the same prefix", code: trigger.CodeTV6, settings: localFiles,
			spec: single(TypeLocalFile, listenAt("/srv/inbox-evil/a"))},
		{name: "T-V7 callers that are not flow ids", code: trigger.CodeTV7,
			spec: single(TypeCalled, `{"family":"INTERNAL","callers":["not-a-flow"]}`)},
		{name: "T-V7 a caller listed twice", code: trigger.CodeTV7,
			spec: single(TypeCalled, `{"family":"INTERNAL","callers":["01930000-0000-7000-8000-0000000000bb","01930000-0000-7000-8000-0000000000bb"]}`)},
		{name: "T-V8 a node type that declares no trigger", code: trigger.CodeTV8,
			spec: single("orcon.mystery@1", WebhookBlock("a"))},
		{name: "T-V8 a family the declaration does not have", code: trigger.CodeTV8,
			spec: single(TypeWebhook, `{"family":"POLL","poll":{"rules":[{"kind":"MINUTES"}]}}`)},
		{name: "T-V8 a deduplication key the type fixes", code: trigger.CodeTV8,
			spec: single(TypeAppEvent, `{"family":"APP_EVENT","registration":{},"dedupe":{"source":"HEADER","name":"X-Id"}}`)},
		{name: "T-V9 window under a minute", code: trigger.CodeTV9, spec: single(TypeWebhook, dedupe("PT30S"))},
		{name: "T-V9 window over 90 days", code: trigger.CodeTV9, spec: single(TypeWebhook, dedupe("P91D"))},
		{name: "T-V1 the same path asked twice by one flow", code: trigger.CodeTV1, contains: []string{"nodes"}, spec: DocSpec{Name: "x",
			Starts: []Node{{ID: "t", Block: WebhookBlock("same")}, {ID: "u", Block: WebhookBlock("same")}}}},
	}
}

func runBlocked(t *testing.T, f Factory, c *blockedCase) {
	t.Helper()
	e := newEnv(t, f, c.settings)
	id := e.newFlow(e.orgA, e.projA, "blocked", Build(c.spec))
	_, err := e.svc.Publish(e.ctx(), trigger.PublishCommand{Org: e.orgA, Flow: id, By: e.user})
	issue := validationIssue(t, "publish", err, c.code)
	if !contains(issue.Message, c.contains...) {
		t.Fatalf("message %q lacks %v", issue.Message, c.contains)
	}
	if n := versionCount(t, e, id); n != 0 {
		t.Fatalf("a blocked publication left %d versions", n)
	}
	if trs := e.triggers(e.orgA, id); len(trs) != 0 {
		t.Fatalf("a blocked publication left registry entries: %+v", trs)
	}
}

// boundsAreAllowed publishes the shortest, the longest and the default
// deduplication window.
func boundsAreAllowed(t *testing.T, f Factory) {
	t.Helper()
	e := newEnv(t, f, trigger.Settings{})
	for _, window := range []string{"PT1M", "P90D", "PT24H"} {
		spec := DocSpec{Name: "w" + window, Starts: []Node{{ID: "t", Block: WebhookBlock("w", `"dedupe":{"source":"HEADER","name":"X-Id","window":"`+window+`"}`)}}}
		id := e.newFlow(e.orgA, e.projA, "bounds"+window, Build(spec))
		if _, err := e.svc.Publish(e.ctx(), trigger.PublishCommand{Org: e.orgA, Flow: id, By: e.user}); err != nil {
			t.Fatalf("window %s: %v", window, err)
		}
		// Free the path for the next flow of the loop.
		must(t, "delete", e.svc.Delete(e.ctx(), e.orgA, id, e.user))
		e.clock.Advance(RecoveryPlusADay)
		must(t, "purge", e.svc.Purge(e.ctx(), e.orgA, id))
	}
}

// collisionInOrganization checks that a path held by another flow of the same
// organization is refused with an error that names that flow.
func collisionInOrganization(t *testing.T, f Factory) {
	t.Helper()
	e := newEnv(t, f, trigger.Settings{})
	first := e.newFlow(e.orgA, e.projA, "first flow", Build(DocSpec{Name: "a", Starts: []Node{{ID: "t", Block: WebhookBlock("shared")}}}))
	e.publish(e.orgA, first)
	second := e.newFlow(e.orgA, e.projA, "second flow", Build(DocSpec{Name: "b", Starts: []Node{{ID: "t", Block: WebhookBlock("shared")}}}))
	_, err := e.svc.Publish(e.ctx(), trigger.PublishCommand{Org: e.orgA, Flow: second, By: e.user})
	issue := validationIssue(t, "publish the second flow", err, trigger.CodeTV1)
	if !contains(issue.Message, `"first flow"`, "/hooks/alpha/shared") {
		t.Fatalf("the same-organization collision message = %q, want it to name the owning flow", issue.Message)
	}
	if n := versionCount(t, e, second); n != 0 {
		t.Fatalf("the blocked publication left %d versions", n)
	}
}

// collisionAcrossOrganizations checks that a path held by another
// organization is refused with a generic problem that reveals nothing about
// that organization (ADR-0022 section 3).
func collisionAcrossOrganizations(t *testing.T, f Factory) {
	t.Helper()
	e := newEnv(t, f, trigger.Settings{})
	other := e.newFlow(e.orgB, e.projB, "secret flow of the other organization", Build(DocSpec{Name: "a", Starts: []Node{{ID: "t", Block: WebhookBlock("shared")}}}))
	e.publish(e.orgB, other)
	mine := e.newFlow(e.orgA, e.projA, "mine", Build(DocSpec{Name: "b", Starts: []Node{{ID: "t", Block: WebhookBlock("shared")}}}))
	_, err := e.svc.Publish(e.ctx(), trigger.PublishCommand{Org: e.orgA, Flow: mine, By: e.user})
	issue := validationIssue(t, "publish against another organization's path", err, trigger.CodeTV1)
	if !contains(issue.Message, "path unavailable") {
		t.Fatalf("the cross-organization message = %q, want the generic problem", issue.Message)
	}
	for _, leak := range []string{"secret flow", e.orgB.String(), e.projB.String(), other.String()} {
		if contains(issue.Message, leak) || contains(err.Error(), leak) {
			t.Fatalf("the error leaks %q of another organization: %v", leak, err)
		}
	}
	// The n8n family is shared by the whole instance: the same rule applies.
	n8n := `{"family":"WEBHOOK","addresses":[{"family":"N8N","path":"hook"}]}`
	theirs := e.newFlow(e.orgB, e.projB, "theirs n8n", Build(DocSpec{Name: "c", Starts: []Node{{ID: "t", Block: n8n}}}))
	e.publish(e.orgB, theirs)
	ours := e.newFlow(e.orgA, e.projA, "ours n8n", Build(DocSpec{Name: "d", Starts: []Node{{ID: "t", Block: n8n}}}))
	_, err = e.svc.Publish(e.ctx(), trigger.PublishCommand{Org: e.orgA, Flow: ours, By: e.user})
	validationIssue(t, "publish against an n8n path of another organization", err, trigger.CodeTV1)
}

// overlapWarns checks that a static path and a parameter path that can match
// the same request only warn, and that the warning is recorded.
func overlapWarns(t *testing.T, f Factory) {
	t.Helper()
	e := newEnv(t, f, trigger.Settings{})
	a := e.newFlow(e.orgA, e.projA, "static", Build(DocSpec{Name: "a", Starts: []Node{{ID: "t", Block: WebhookBlock("orders/new")}}}))
	e.publish(e.orgA, a)
	b := e.newFlow(e.orgA, e.projA, "parameter", Build(DocSpec{Name: "b", Starts: []Node{{ID: "t", Block: WebhookBlock("orders/:id")}}}))
	p := e.publish(e.orgA, b)
	if !p.Report.Has(trigger.CodeTV1, flow.SeverityWarning) {
		t.Fatalf("overlapping patterns did not warn: %v", p.Report.Issues)
	}
	if len(p.Version.Warnings) == 0 {
		t.Fatal("the warning was not recorded with the version")
	}
}

func missingCredential(t *testing.T, f Factory) {
	t.Helper()
	e := newEnv(t, f, trigger.Settings{})
	e.creds.SetMissing("feed token")
	spec := single(TypePoll, `{"family":"POLL","poll":{"rules":[{"kind":"MINUTES","every":5}]}}`)
	id := e.newFlow(e.orgA, e.projA, "poller", Build(spec))
	_, err := e.svc.Publish(e.ctx(), trigger.PublishCommand{Org: e.orgA, Flow: id, By: e.user})
	issue := validationIssue(t, "publish without the credential", err, trigger.CodeTV3)
	if !contains(issue.Message, "feed token") {
		t.Fatalf("message %q does not name the credential", issue.Message)
	}
	e.creds.SetMissing()
	e.publish(e.orgA, id)
}

func importedAddress(t *testing.T, f Factory) {
	t.Helper()
	e := newEnv(t, f, trigger.Settings{})
	spec := func(name, path string) []byte {
		block := `{"family":"WEBHOOK","addresses":[{"family":"N8N","path":"` + path + `"}]}`
		return Build(DocSpec{Name: name, Imported: true, Starts: []Node{{ID: "t", Block: block}}})
	}
	id := e.newFlow(e.orgA, e.projA, "imported", spec("v1", "legacy-hook"))
	e.publish(e.orgA, id)
	e.saveDraft(e.orgA, id, spec("v2", "renamed-hook"))
	_, err := e.svc.Publish(e.ctx(), trigger.PublishCommand{Org: e.orgA, Flow: id, By: e.user})
	validationIssue(t, "change an imported address", err, trigger.CodeTV4)
	_, err = e.svc.Publish(e.ctx(), trigger.PublishCommand{Org: e.orgA, Flow: id, By: e.user, AllowImportedAddressChange: true})
	must(t, "publish with the confirmation", err)
}

// importedAddressNodeRemoved checks T-V4 for a draft that deletes a whole
// trigger node owning an imported n8n address: publishing stops with an error
// that names the address, and succeeds with the explicit confirmation.
func importedAddressNodeRemoved(t *testing.T, f Factory) {
	t.Helper()
	e := newEnv(t, f, trigger.Settings{})
	n8n := `{"family":"WEBHOOK","addresses":[{"family":"N8N","path":"legacy-hook"}]}`
	both := Build(DocSpec{Name: "v1", Imported: true, Starts: []Node{{ID: "t", Block: n8n}, {ID: "keep", Block: WebhookBlock("kept")}}})
	id := e.newFlow(e.orgA, e.projA, "imported", both)
	e.publish(e.orgA, id)
	e.saveDraft(e.orgA, id, Build(DocSpec{Name: "v2", Imported: true, Starts: []Node{{ID: "keep", Block: WebhookBlock("kept")}}}))
	_, err := e.svc.Publish(e.ctx(), trigger.PublishCommand{Org: e.orgA, Flow: id, By: e.user})
	issue := validationIssue(t, "delete a trigger node that owns an imported address", err, trigger.CodeTV4)
	if !contains(issue.Message, "/legacy-hook") {
		t.Fatalf("message %q does not name the address", issue.Message)
	}
	if n := versionCount(t, e, id); n != 1 {
		t.Fatalf("the refused publication left %d versions, want 1", n)
	}
	_, err = e.svc.Publish(e.ctx(), trigger.PublishCommand{Org: e.orgA, Flow: id, By: e.user, AllowImportedAddressChange: true})
	must(t, "publish with the confirmation", err)
}

func unreachableRespondNode(t *testing.T, f Factory) {
	t.Helper()
	e := newEnv(t, f, trigger.Settings{})
	block := WebhookBlock("answer", `"response":{"mode":"RESPOND_NODE","waitTimeout":"PT30S"}`)
	id := e.newFlow(e.orgA, e.projA, "answers", Build(DocSpec{Name: "a", Starts: []Node{{ID: "t", Block: block}}}))
	p := e.publish(e.orgA, id)
	if !p.Report.Has(trigger.CodeTV5, flow.SeverityWarning) || len(p.Version.Warnings) == 0 {
		t.Fatalf("a RESPOND_NODE trigger with no respond node did not warn: %v", p.Report.Issues)
	}
	with := Build(DocSpec{Name: "b", Starts: []Node{{ID: "t", Block: block, Next: "respond"}},
		Extra:      []string{`{"id":"respond","name":"Respond","element":"task","type":"orcon.respond@1","parameters":{}}`},
		ExtraEdges: []string{`{"id":"e-respond","from":{"node":"respond","port":"main"},"to":{"node":"end"},"kind":"SEQUENCE"}`}})
	e.saveDraft(e.orgA, id, with)
	if p = e.publish(e.orgA, id); p.Report.Has(trigger.CodeTV5, flow.SeverityWarning) {
		t.Fatalf("a reachable respond node still warns: %v", p.Report.Issues)
	}
}

func localFileInside(t *testing.T, f Factory) {
	t.Helper()
	e := newEnv(t, f, trigger.Settings{LocalFileEnabled: true, LocalFileAllowedPaths: []string{"/srv/inbox"}})
	id := e.newFlow(e.orgA, e.projA, "files", Build(DocSpec{Name: "a", Starts: []Node{{ID: "t", Type: TypeLocalFile, Block: `{"family":"LISTEN","listen":{"localPath":"/srv/inbox/orders"}}`}}}))
	e.publish(e.orgA, id)
}

func dryRun(t *testing.T, f Factory) {
	t.Helper()
	e := newEnv(t, f, trigger.Settings{})
	owner := e.newFlow(e.orgA, e.projA, "owner", Build(DocSpec{Name: "a", Starts: []Node{{ID: "t", Block: WebhookBlock("taken")}}}))
	e.publish(e.orgA, owner)
	id := e.newFlow(e.orgA, e.projA, "dry", Build(DocSpec{Name: "b", Starts: []Node{{ID: "t", Block: WebhookBlock("taken")}}}))
	report, err := e.svc.Validate(e.ctx(), e.orgA, id)
	must(t, "validate", err)
	if !report.Has(trigger.CodeTV1, flow.SeverityError) {
		t.Fatalf("Validate missed the collision: %v", report.Issues)
	}
	if n := versionCount(t, e, id); n != 0 {
		t.Fatalf("Validate created %d versions", n)
	}
}
