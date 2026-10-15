// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

const testFlowID = flow.ID("01930000-0000-7000-8000-0000000000aa")

// readWorkflow loads a golden workflow from testdata/workflows.
func readWorkflow(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "workflows", name+".json")) //nolint:gosec // G304: fixed test data directory
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func convertFixture(t *testing.T, name string) *Conversion {
	t.Helper()
	conv, err := ConvertWorkflow(readWorkflow(t, name), testFlowID, nil)
	if err != nil {
		t.Fatalf("ConvertWorkflow(%s): %v", name, err)
	}
	return conv
}

// outcomeOf finds the report entry of a node by its n8n name.
func outcomeOf(t *testing.T, conv *Conversion, name string) NodeOutcome {
	t.Helper()
	for _, n := range conv.Nodes {
		if n.Name == name {
			return n
		}
	}
	t.Fatalf("no report entry for node %q", name)
	return NodeOutcome{}
}

func TestConvertGoldenWorkflows(t *testing.T) {
	tests := []struct {
		name     string
		fixture  string
		outcomes map[string]Outcome
		types    map[string]string
		// problems is the number of validation problems the produced flow has.
		problems int
	}{
		{
			name:    "lead intake keeps routing and marks what is not on main",
			fixture: "whatsapp-lead-intake",
			outcomes: map[string]Outcome{
				"WhatsApp Webhook":    OutcomeConverted,
				"Is Text Message":     OutcomeConverted,
				"Route by Intent":     OutcomeConverted,
				"Create Lead":         OutcomeUnsupported,
				"Save to CRM":         OutcomeUnsupported,
				"Send Quote":          OutcomeUnsupported,
				"Wait 15 min":         OutcomeConverted,
				"Ask for Appointment": OutcomeUnsupported,
				"Lead Agent":          OutcomeUnsupported,
				"OpenAI Chat Model":   OutcomeUnsupported,
				"Merge":               OutcomeAdapted,
				"Notify Team":         OutcomeUnsupported,
			},
			types: map[string]string{
				"WhatsApp Webhook": "orcon.webhook@1",
				"Is Text Message":  "orcon.if@1",
				"Route by Intent":  "orcon.switch@1",
				"Wait 15 min":      "orcon.wait@1",
				"Merge":            "orcon.merge@1",
			},
			// The AI sub-node is connected only through an AI connection, which is
			// reported and not carried, so the flow has it unreachable.
			problems: 1,
		},
		{
			name:    "follow-up reminders converts schedules and waits",
			fixture: "scheduled-followup",
			outcomes: map[string]Outcome{
				"Every 15 minutes": OutcomeConverted,
				"Weekday morning":  OutcomeConverted,
				"Weekly digest":    OutcomeUnsupported,
				"Wait two days":    OutcomeConverted,
				"Build reminder":   OutcomeUnsupported,
				"Manual start":     OutcomeConverted,
				"Join reminders":   OutcomeAdapted,
				"Notify owner":     OutcomeUnsupported,
			},
			types: map[string]string{
				"Every 15 minutes": "orcon.schedule@1",
				"Manual start":     "orcon.manual@1",
			},
			problems: 0,
		},
		{
			name:    "legacy edge cases stay visible",
			fixture: "edge-cases",
			outcomes: map[string]Outcome{
				"Legacy start":        OutcomeConverted,
				"Old IF":              OutcomeUnsupported,
				"Paid branch":         OutcomeUnsupported,
				"Combine by position": OutcomeUnsupported,
				"Lead lookup":         OutcomeConverted,
				"Dynamic wait":        OutcomeUnsupported,
				"Acme message":        OutcomeUnsupported,
			},
			types: map[string]string{
				"Legacy start": "orcon.manual@1",
				"Lead lookup":  "orcon.webhook@1",
			},
			problems: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conv := convertFixture(t, tt.fixture)
			if got := len(conv.Nodes); got != len(tt.outcomes) {
				t.Fatalf("report has %d entries, want %d", got, len(tt.outcomes))
			}
			for name, want := range tt.outcomes {
				if got := outcomeOf(t, conv, name).Outcome; got != want {
					t.Errorf("%s: outcome %s, want %s", name, got, want)
				}
			}
			for name, want := range tt.types {
				if got := outcomeOf(t, conv, name).OrconType; got != want {
					t.Errorf("%s: Orcon type %q, want %q", name, got, want)
				}
			}
			if got := len(conv.Problems); got != tt.problems {
				t.Errorf("validation problems = %d, want %d: %v", got, tt.problems, conv.Problems)
			}
		})
	}
}

// TestNoSilentDrop checks the rule of the import: every n8n node has one
// report entry and one flow node, and every node that is not converted says
// why.
func TestNoSilentDrop(t *testing.T) {
	for _, fixture := range []string{"whatsapp-lead-intake", "scheduled-followup", "edge-cases"} {
		t.Run(fixture, func(t *testing.T) {
			conv := convertFixture(t, fixture)
			wf, err := parseWorkflow(readWorkflow(t, fixture))
			if err != nil {
				t.Fatal(err)
			}
			nonSticky := 0
			for _, n := range wf.nodes {
				if !IsStickyNote(n.Type) {
					nonSticky++
				}
			}
			if len(conv.Nodes) != nonSticky || len(conv.Document.Nodes) != nonSticky {
				t.Fatalf("n8n nodes %d, report entries %d, flow nodes %d", nonSticky, len(conv.Nodes), len(conv.Document.Nodes))
			}
			for _, n := range conv.Nodes {
				if n.Outcome == OutcomeUnsupported && len(n.Findings) == 0 {
					t.Errorf("%s is unsupported without a finding", n.Name)
				}
				if n.Outcome == OutcomeUnsupported && n.OrconType != "" {
					t.Errorf("%s is unsupported but names type %q", n.Name, n.OrconType)
				}
			}
		})
	}
}

// TestUnsupportedNodeKeepsOriginal checks that a node Orcon cannot run keeps
// the n8n node in its ext, so nothing is lost when the flow is rebuilt.
func TestUnsupportedNodeKeepsOriginal(t *testing.T) {
	conv := convertFixture(t, "whatsapp-lead-intake")
	var found bool
	for _, n := range conv.Document.Nodes {
		if n.Name != "Save to CRM" {
			continue
		}
		found = true
		if n.Type != unsupportedType {
			t.Fatalf("type %q, want %s", n.Type, unsupportedType)
		}
		imp, ok := n.Ext.Get(flow.ImportExtKey)
		if !ok {
			t.Fatal("no orcon.import member")
		}
		orig, ok := imp.Get("original")
		if !ok {
			t.Fatal("the original n8n node is not kept")
		}
		if got, _ := orig.Get("type"); got.Kind() != jsonvalue.KindString {
			t.Errorf("original lacks its type")
		}
		if _, ok := orig.Get("parameters"); !ok {
			t.Errorf("original lacks its parameters")
		}
	}
	if !found {
		t.Fatal("Save to CRM is not in the document")
	}
}

func TestSwitchPortsAndFallback(t *testing.T) {
	conv := convertFixture(t, "whatsapp-lead-intake")
	var ports []string
	defaults := map[string]bool{}
	for _, e := range conv.Document.Edges {
		if e.From.Node != "11111111-0000-4000-8000-000000000004" {
			continue
		}
		ports = append(ports, e.From.Port)
		if e.Default != nil && *e.Default {
			defaults[string(e.To)] = true
		}
	}
	want := []string{"case:0", "case:1", "main"}
	if diff := cmp.Diff(want, ports); diff != "" {
		t.Errorf("switch ports mismatch (-want +got):\n%s", diff)
	}
	if !defaults["11111111-0000-4000-8000-000000000005"] {
		t.Errorf("the fallback output is not the default edge to Create Lead")
	}
}

func TestDisabledNodeStaysDisabled(t *testing.T) {
	conv := convertFixture(t, "edge-cases")
	var disabled bool
	for _, n := range conv.Document.Nodes {
		if n.Name == "Paid branch" {
			disabled = n.Disabled != nil && *n.Disabled
		}
	}
	if !disabled {
		t.Fatal("the disabled n8n node is not disabled in the flow")
	}
	if got := outcomeOf(t, conv, "Paid branch").Findings; !hasCode(got, FindDisabled) {
		t.Errorf("the disabled node has no DISABLED finding: %v", got)
	}
}

func TestEditorKeepsPositionsAndStickyNotes(t *testing.T) {
	conv := convertFixture(t, "whatsapp-lead-intake")
	ed := conv.Document.Editor
	if _, ok := ed.Get("nodes"); !ok {
		t.Fatal("editor has no node positions")
	}
	notes, ok := ed.Get("stickyNotes")
	if !ok || notes.Len() != 1 {
		t.Fatalf("sticky notes = %v, want one", notes)
	}
}

func TestConvertRejectsBadInput(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantErr error
	}{
		{name: "not json", data: `{"nodes": [`},
		{name: "no nodes array", data: `{"name": "x"}`, wantErr: ErrNoNodes},
		{name: "node without type", data: `{"nodes": [{"name": "x"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ConvertWorkflow([]byte(tt.data), testFlowID, nil)
			if err == nil {
				t.Fatal("no error")
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("error %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestConvertRejectsDuplicateNames(t *testing.T) {
	data := `{"nodes": [
		{"id": "a", "name": "Same", "type": "n8n-nodes-base.noOp", "typeVersion": 1},
		{"id": "b", "name": "Same", "type": "n8n-nodes-base.noOp", "typeVersion": 1}]}`
	if _, err := ConvertWorkflow([]byte(data), testFlowID, nil); err == nil {
		t.Fatal("two nodes with one name were accepted")
	}
}

func TestConvertReadsWrappedTemplate(t *testing.T) {
	inner := readWorkflow(t, "scheduled-followup")
	wrapped := []byte(`{"workflow": {"id": 7, "totalViews": 3, "workflow": ` + string(inner) + `}}`)
	conv, err := ConvertWorkflow(wrapped, testFlowID, nil)
	if err != nil {
		t.Fatal(err)
	}
	plain := convertFixture(t, "scheduled-followup")
	if len(conv.Nodes) != len(plain.Nodes) {
		t.Errorf("wrapped conversion has %d nodes, plain has %d", len(conv.Nodes), len(plain.Nodes))
	}
}

func TestOutcomeCountsAndMarkdown(t *testing.T) {
	conv := convertFixture(t, "scheduled-followup")
	k := conv.OutcomeCounts()
	want := OutcomeCounts{Nodes: 8, Converted: 4, Adapted: 1, Unsupported: 3}
	if diff := cmp.Diff(want, k); diff != "" {
		t.Errorf("counts mismatch (-want +got):\n%s", diff)
	}
	md := conv.Markdown()
	for _, s := range []string{"# n8n import report: Follow-up reminders", "| Weekly digest |", "UNSUPPORTED_PARAMETER"} {
		if !strings.Contains(md, s) {
			t.Errorf("report lacks %q", s)
		}
	}
}

func TestMarkdownEscapesCells(t *testing.T) {
	if got := cell("a|b\nc"); got != `a\|b c` {
		t.Errorf("cell() = %q", got)
	}
}

func hasCode(fs []Finding, code FindingCode) bool {
	for _, f := range fs {
		if f.Code == code {
			return true
		}
	}
	return false
}

func TestFlowJSONRoundTrips(t *testing.T) {
	conv := convertFixture(t, "scheduled-followup")
	data, err := conv.FlowJSON()
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonvalue.Parse(data)
	if err != nil {
		t.Fatalf("FlowJSON is not JSON: %v", err)
	}
	doc, err := flow.Read(v)
	if err != nil {
		t.Fatalf("FlowJSON is not a flow document: %v", err)
	}
	if len(doc.Nodes) != len(conv.Document.Nodes) || len(doc.Edges) != len(conv.Document.Edges) {
		t.Errorf("round trip changed the shape: nodes %d/%d, edges %d/%d",
			len(doc.Nodes), len(conv.Document.Nodes), len(doc.Edges), len(conv.Document.Edges))
	}
}

func TestScheduleRules(t *testing.T) {
	tests := []struct {
		name    string
		item    string
		want    flow.TriggerRule
		wantErr bool
	}{
		{name: "seconds default interval", item: `{"field": "seconds"}`, want: flow.TriggerRule{Kind: flow.RuleSeconds, Every: 1}},
		{name: "minutes", item: `{"field": "minutes", "minutesInterval": 5}`, want: flow.TriggerRule{Kind: flow.RuleMinutes, Every: 5}},
		{name: "hours on the hour", item: `{"field": "hours", "hoursInterval": 2, "triggerAtMinute": 0}`, want: flow.TriggerRule{Kind: flow.RuleHours, Every: 2}},
		{name: "hours off the hour", item: `{"field": "hours", "hoursInterval": 2, "triggerAtMinute": 15}`, wantErr: true},
		{name: "days at a time", item: `{"field": "days", "daysInterval": 1, "triggerAtHour": 7, "triggerAtMinute": 5}`, want: flow.TriggerRule{Kind: flow.RuleDays, Every: 1, At: "07:05"}},
		{name: "day time out of range", item: `{"field": "days", "triggerAtHour": 25}`, wantErr: true},
		{name: "fractional interval", item: `{"field": "minutes", "minutesInterval": 1.5}`, wantErr: true},
		{name: "cron", item: `{"field": "cronExpression", "expression": "0 8 * * 1-5"}`, want: flow.TriggerRule{Kind: flow.RuleCron, Cron: "0 8 * * 1-5"}},
		{name: "cron as an expression", item: `{"field": "cronExpression", "expression": "={{ $json.cron }}"}`, wantErr: true},
		{name: "weeks are not converted", item: `{"field": "weeks"}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := jsonvalue.Parse([]byte(tt.item))
			if err != nil {
				t.Fatal(err)
			}
			got, err := scheduleRule(v)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error %v, want error %v", err, tt.wantErr)
			}
			if !tt.wantErr && (got.Kind != tt.want.Kind || got.Every != tt.want.Every || got.At != tt.want.At || got.Cron != tt.want.Cron) {
				t.Errorf("rule = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestWebhookModesAndMethods(t *testing.T) {
	tests := []struct {
		name   string
		params string
		want   flow.ResponseMode
		wantOK bool
	}{
		{name: "default answers on receipt", params: `{}`, want: flow.ResponseImmediate, wantOK: true},
		{name: "last node", params: `{"responseMode": "lastNode"}`, want: flow.ResponseLastNode, wantOK: true},
		{name: "respond node", params: `{"responseMode": "responseNode"}`, want: flow.ResponseRespondNode, wantOK: true},
		{name: "streaming is not converted", params: `{"responseMode": "streaming"}`, wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := jsonvalue.Parse([]byte(tt.params))
			if err != nil {
				t.Fatal(err)
			}
			mode, findings, ok := responseMode(&Node{Name: "Hook"}, v)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (findings %v)", ok, tt.wantOK, findings)
			}
			if ok && mode != tt.want {
				t.Errorf("mode = %s, want %s", mode, tt.want)
			}
		})
	}
	list, err := jsonvalue.Parse([]byte(`{"httpMethod": ["get", "post"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := webhookMethods(list); len(got) != 2 || got[0] != "GET" || got[1] != "POST" {
		t.Errorf("methods = %v, want [GET POST]", got)
	}
	none, err := jsonvalue.Parse([]byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := webhookMethods(none); len(got) != 1 || got[0] != "GET" {
		t.Errorf("default methods = %v, want [GET]", got)
	}
}
