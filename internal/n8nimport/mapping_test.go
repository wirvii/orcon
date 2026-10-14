// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport_test

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/n8nimport"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// importNode parses one node as an n8n export writes it.
func importNode(t *testing.T, nodeJSON string) n8nimport.Node {
	t.Helper()
	tp, err := n8nimport.ParseTemplate([]byte(`{"nodes":[` + nodeJSON + `]}`))
	if err != nil {
		t.Fatal(err)
	}
	return tp.Nodes[0]
}

// slackCatalog declares the slack node of typeVersion 2.2 with two
// operations, the way an installed connector would.
func slackCatalog(t *testing.T) *n8nimport.Catalog {
	t.Helper()
	cat, err := n8nimport.NewCatalog(n8nimport.Connector{
		ID: "slack", Effort: n8nimport.EffortDeclarative, OrconType: "slack.action@1",
		Credentials: map[string]string{"slackApi": "slack.token"},
		Units: []n8nimport.UnitMap{
			{Unit: n8nimport.Unit{Type: slack, TypeVersion: "2.2", Resource: "message", Operation: "post"}, OrconOperation: "message.send"},
			{Unit: n8nimport.Unit{Type: slack, TypeVersion: "2.2", Resource: "channel", Operation: "getAll"}, OrconOperation: "channel.getMany"},
			{Unit: n8nimport.Unit{Type: slack, TypeVersion: "2.2"}, OrconOperation: "message.send"},
			{Unit: n8nimport.Unit{Type: slack, TypeVersion: "2.2", Resource: "message", Operation: "delete"}, OrconOperation: "message.delete", Unsafe: true},
		},
	}, n8nimport.Connector{
		ID: "planned", Effort: n8nimport.EffortDeclarative, // no Orcon type: planned, not installed
		Units: []n8nimport.UnitMap{{Unit: n8nimport.Unit{Type: gmail, TypeVersion: "2", Resource: "message", Operation: "getAll"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func slackNode(t *testing.T, version, params string) n8nimport.Node {
	t.Helper()
	return importNode(t, fmt.Sprintf(`{"id":"n1","name":"Say hi","type":%q,"typeVersion":%s,"parameters":%s}`, slack, version, params))
}

func TestNodeMappingNoSilentSubstitution(t *testing.T) {
	cat := slackCatalog(t)
	tests := []struct {
		name  string
		node  n8nimport.Node
		code  n8nimport.FindingCode
		field string
	}{
		{"unknown type", importNode(t, `{"id":"a","name":"Mystery","type":"n8n-nodes-base.unknownThing","typeVersion":1,"parameters":{}}`), n8nimport.FindUnmappedType, "type"},
		{"community node", importNode(t, `{"id":"a","name":"Mystery","type":"n8n-nodes-someone.odd","typeVersion":1,"parameters":{}}`), n8nimport.FindUnmappedType, "type"},
		{"AI node", importNode(t, `{"id":"a","name":"Mystery","type":"@n8n/n8n-nodes-langchain.agent","typeVersion":1,"parameters":{}}`), n8nimport.FindUnmappedType, "type"},
		{"typeVersion without mapping", slackNode(t, "1", `{"resource":"message","operation":"post"}`), n8nimport.FindUnmappedTypeVersion, "typeVersion"},
		{"pair outside the map", slackNode(t, "2.2", `{"resource":"message","operation":"update"}`), n8nimport.FindUnmappedOperation, "parameters.operation"},
		{"typeVersion that is not a number", importNode(t, `{"id":"a","name":"Odd","type":"n8n-nodes-base.if","typeVersion":"two","parameters":{}}`), n8nimport.FindUnmappedTypeVersion, "typeVersion"},
		{"core node without counterpart", importNode(t, `{"id":"a","name":"Table","type":"n8n-nodes-base.dataTable","typeVersion":1,"parameters":{}}`), n8nimport.FindUnsupportedCore, "type"},
		{"planned connector", importNode(t, fmt.Sprintf(`{"id":"a","name":"Mail","type":%q,"typeVersion":2,"parameters":{"resource":"message","operation":"getAll"}}`, gmail)), n8nimport.FindPlannedOnly, "type"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := n8nimport.MapNode(&tc.node, cat)
			if m.Mapped() || m.OrconType != "" || m.Operation != "" || !m.Ext.IsZero() {
				t.Fatalf("the node was approximated: %+v", m)
			}
			var found bool
			for _, f := range m.Findings {
				if f.Code == tc.code && f.Field == tc.field && f.Node == tc.node.Name && f.NodeID == tc.node.ID && f.Message != "" {
					found = true
				}
			}
			if !found {
				t.Fatalf("no %s finding on %s for the node %q: %+v", tc.code, tc.field, tc.node.Name, m.Findings)
			}
		})
	}
}

func TestTypeVersionPreserved(t *testing.T) {
	cat := slackCatalog(t)
	tests := []struct {
		name    string
		node    n8nimport.Node
		version string
	}{
		{"connector node", slackNode(t, "2.2", `{"resource":"message","operation":"post"}`), "2.2"},
		{"core node", importNode(t, `{"id":"n9","name":"Check","type":"n8n-nodes-base.if","typeVersion":2.1,"parameters":{}}`), "2.1"},
		{"typeVersion left out", importNode(t, `{"id":"n9","name":"Check","type":"n8n-nodes-base.if","parameters":{}}`), "1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := n8nimport.MapNode(&tc.node, cat)
			if !m.Mapped() {
				t.Fatalf("not mapped: %+v", m.Findings)
			}
			imp, ok := m.Ext.Get(flow.ImportExtKey)
			if !ok {
				t.Fatalf("ext has no %s: %v", flow.ImportExtKey, m.Ext)
			}
			if got := strMember(imp, "n8nType"); got != tc.node.Type {
				t.Errorf("n8nType = %q, want %q", got, tc.node.Type)
			}
			v, _ := imp.Get("n8nTypeVersion")
			if text, ok := v.NumberText(); !ok || text != tc.version {
				t.Errorf("n8nTypeVersion = %v, want the number %s", v, tc.version)
			}
			if got := strMember(imp, "n8nNodeId"); got != tc.node.ID {
				t.Errorf("n8nNodeId = %q, want %q", got, tc.node.ID)
			}
		})
	}
}

func strMember(v jsonvalue.Value, key string) string {
	m, ok := v.Get(key)
	if !ok {
		return ""
	}
	s, _ := m.Str()
	return s
}

func TestHttpWriteImportedUnsafe(t *testing.T) {
	tests := []struct {
		name   string
		params string
		safety n8nimport.Safety
		review bool
		field  string
		reason string
	}{
		{"GET", `{"method":"GET","url":"https://example.test"}`, n8nimport.SafetyRead, false, "", ""},
		{"HEAD in lower case", `{"method":"head"}`, n8nimport.SafetyRead, false, "", ""},
		{"method left out", `{"url":"https://example.test"}`, n8nimport.SafetyRead, false, "", ""},
		{"POST", `{"method":"POST"}`, n8nimport.SafetyUnsafe, true, "parameters.method", "HTTP_WRITE"},
		{"PUT", `{"method":"PUT"}`, n8nimport.SafetyUnsafe, true, "parameters.method", "HTTP_WRITE"},
		{"PATCH", `{"method":"PATCH"}`, n8nimport.SafetyUnsafe, true, "parameters.method", "HTTP_WRITE"},
		{"DELETE", `{"method":"DELETE"}`, n8nimport.SafetyUnsafe, true, "parameters.method", "HTTP_WRITE"},
		{"post in lower case", `{"method":"post"}`, n8nimport.SafetyUnsafe, true, "parameters.method", "HTTP_WRITE"},
		{"method by expression", `{"method":"={{ $json.verb }}"}`, n8nimport.SafetyUnsafe, true, "parameters.method", "HTTP_METHOD_EXPRESSION"},
		{"typeVersion 1 spelling", `{"requestMethod":"POST"}`, n8nimport.SafetyUnsafe, true, "parameters.requestMethod", "HTTP_WRITE"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n := importNode(t, `{"id":"h","name":"Call","type":"n8n-nodes-base.httpRequest","typeVersion":4.2,"parameters":`+tc.params+`}`)
			m := n8nimport.MapNode(&n, nil)
			if m.OrconType != "orcon.http@1" {
				t.Fatalf("type = %q, want orcon.http@1", m.OrconType)
			}
			if m.Safety != tc.safety {
				t.Errorf("safety = %q, want %q", m.Safety, tc.safety)
			}
			if m.ReviewRequired() != tc.review {
				t.Errorf("review required = %v, want %v", m.ReviewRequired(), tc.review)
			}
			imp, _ := m.Ext.Get(flow.ImportExtKey)
			status, hasStatus := imp.Get("reviewStatus")
			if hasStatus != tc.review {
				t.Fatalf("ext review status present = %v, want %v", hasStatus, tc.review)
			}
			if tc.review {
				if got, _ := status.Str(); got != flow.ReviewNeeds {
					t.Errorf("reviewStatus = %q, want %q", got, flow.ReviewNeeds)
				}
				reasons, _ := imp.Get("reviewReasons")
				if first, _ := reasons.At(0); reasons.Len() != 1 || strMemberValue(first) != tc.reason {
					t.Errorf("reviewReasons = %v, want [%s]", reasons, tc.reason)
				}
				for _, f := range m.Findings {
					if f.Code == n8nimport.FindReviewRequired && f.Field != tc.field {
						t.Errorf("finding field = %q, want %q", f.Field, tc.field)
					}
				}
			}
		})
	}
}

func strMemberValue(v jsonvalue.Value) string {
	s, _ := v.Str()
	return s
}

func TestUnsafeConnectorOperationNeedsReview(t *testing.T) {
	n := slackNode(t, "2.2", `{"resource":"message","operation":"delete"}`)
	m := n8nimport.MapNode(&n, slackCatalog(t))
	if !m.Mapped() || m.Operation != "message.delete" || m.Safety != n8nimport.SafetyUnsafe || !m.ReviewRequired() {
		t.Fatalf("mapping = %+v", m)
	}
	imp, _ := m.Ext.Get(flow.ImportExtKey)
	reasons, _ := imp.Get("reviewReasons")
	if first, _ := reasons.At(0); strMemberValue(first) != flow.ReasonUnsafeOperation {
		t.Errorf("reviewReasons = %v", reasons)
	}
	safe := slackNode(t, "2.2", `{"resource":"message","operation":"post"}`)
	if sm := n8nimport.MapNode(&safe, slackCatalog(t)); sm.ReviewRequired() || sm.Safety != "" {
		t.Errorf("a safe operation must not be marked: %+v", sm)
	}
}

func TestLegacyAndRenamedCoreNodes(t *testing.T) {
	start := importNode(t, `{"id":"s","name":"Start","type":"n8n-nodes-base.start","typeVersion":1,"parameters":{}}`)
	if m := n8nimport.MapNode(&start, nil); m.OrconType != "orcon.manual@1" {
		t.Errorf("start maps to %+v, want orcon.manual@1", m)
	}
	api := importNode(t, `{"id":"a","name":"Instance","type":"n8n-nodes-base.n8n","typeVersion":1,"parameters":{}}`)
	m := n8nimport.MapNode(&api, nil)
	if !m.Mapped() || !strings.Contains(m.Construct, "n8n REST API") {
		t.Errorf("the n8n API node maps to %+v, want a connector over the n8n REST API", m)
	}
}

func TestImportedFlowValidates(t *testing.T) {
	data, err := os.ReadFile("../flow/testdata/conformance/linear.json")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := jsonvalue.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := flow.Read(raw)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []n8nimport.Node{
		importNode(t, `{"id":"w","name":"Write","type":"n8n-nodes-base.httpRequest","typeVersion":4.2,"parameters":{"method":"POST"}}`),
		importNode(t, `{"id":"e","name":"Expr","type":"n8n-nodes-base.httpRequest","typeVersion":4.2,"parameters":{"method":"={{ $json.m }}"}}`),
		slackNode(t, "2.2", `{"resource":"message","operation":"delete"}`),
	}
	for i := range nodes {
		doc.Nodes[i].Ext = n8nimport.MapNode(&nodes[i], slackCatalog(t)).Ext
	}
	if report := flow.Validate(&doc); report.HasErrors() {
		t.Fatalf("an imported flow must validate: %v", report.Errors())
	}
	pending := flow.PendingImportReview(&doc)
	if len(pending) != 3 {
		t.Fatalf("pending = %v, want the three marked nodes", pending)
	}
	// The same mark in the document's ext is still an error.
	bad := doc
	bad.Ext = doc.Nodes[0].Ext
	if report := flow.Validate(&bad); !report.HasErrors() {
		t.Error("orcon.import in the document ext must fail")
	}
}

func TestImportedUnsafeBlocksActivation(t *testing.T) {
	post := importNode(t, `{"id":"w","name":"Write","type":"n8n-nodes-base.httpRequest","typeVersion":4,"parameters":{"method":"POST"}}`)
	get := importNode(t, `{"id":"r","name":"Read","type":"n8n-nodes-base.httpRequest","typeVersion":4,"parameters":{"method":"GET"}}`)
	del := importNode(t, `{"id":"d","name":"Erase","type":"n8n-nodes-base.httpRequest","typeVersion":4,"parameters":{"method":"DELETE"}}`)
	doc := &flow.Document{}
	for _, n := range []n8nimport.Node{post, get, del} {
		m := n8nimport.MapNode(&n, nil)
		doc.Nodes = append(doc.Nodes, flow.Node{ID: flow.NodeID(n.ID), Name: n.Name, Ext: m.Ext})
	}

	err := flow.CheckImportReview(doc)
	var re *flow.ImportReviewError
	if !errors.Is(err, flow.ErrImportReviewRequired) || !errors.As(err, &re) {
		t.Fatalf("activation of a flow with unreviewed writes: %v", err)
	}
	if got := fmt.Sprint(re.Nodes); got != "[w d]" {
		t.Errorf("pending = %s, want [w d]", got)
	}
	if !flow.MarkImportReviewed(doc, "w") || !flow.MarkImportReviewed(doc, "d") {
		t.Fatal("review failed")
	}
	if err := flow.CheckImportReview(doc); err != nil {
		t.Fatalf("every marked node was reviewed, got %v", err)
	}
}

func TestCredentialMappingNoSecrets(t *testing.T) {
	const secret = "SECRET-VALUE-0123"
	n := importNode(t, fmt.Sprintf(`{"id":"c","name":"Say hi","type":%q,"typeVersion":2.2,
	  "parameters":{"resource":"message","operation":"post"},
	  "credentials":{
	    "slackApi":{"id":"77","name":"Team Slack","accessToken":%q,"data":{"apiKey":%q}},
	    "oddApi":{"id":"78","name":"Odd","password":%q}}}`, slack, secret, secret, secret))
	m := n8nimport.MapNode(&n, slackCatalog(t))
	if !m.Mapped() {
		t.Fatalf("not mapped: %+v", m.Findings)
	}
	if len(m.Credentials) != 2 {
		t.Fatalf("credentials = %+v", m.Credentials)
	}
	for _, c := range m.Credentials {
		switch c.N8nType {
		case "slackApi":
			if c.Name != "Team Slack" || c.OrconType != "slack.token" {
				t.Errorf("slackApi = %+v", c)
			}
		case "oddApi":
			if c.OrconType != "" {
				t.Errorf("oddApi has no declared counterpart but maps to %q", c.OrconType)
			}
		default:
			t.Errorf("unexpected credential %+v", c)
		}
	}
	var unmapped int
	for _, f := range m.Findings {
		if f.Code == n8nimport.FindUnmappedCredential {
			unmapped++
			if f.Field != "credentials.oddApi" {
				t.Errorf("finding field = %q", f.Field)
			}
		}
	}
	if unmapped != 1 {
		t.Errorf("unmapped credential findings = %d, want 1", unmapped)
	}
	// Nothing of the secret may be anywhere in the result.
	text := fmt.Sprintf("%+v", m)
	if ext, err := jsonvalue.Marshal(m.Ext); err == nil {
		text += string(ext)
	}
	if strings.Contains(text, secret) {
		t.Fatalf("a credential value reached the mapping: %s", text)
	}
}

func TestN8nOperationPairMapping(t *testing.T) {
	cat := slackCatalog(t)
	tests := []struct {
		name      string
		params    string
		operation string
		code      n8nimport.FindingCode
		field     string
	}{
		{"pair becomes resource.verb", `{"resource":"message","operation":"post"}`, "message.send", "", ""},
		{"getAll becomes getMany", `{"resource":"channel","operation":"getAll"}`, "channel.getMany", "", ""},
		{"node without a choice", `{}`, "message.send", "", ""},
		{"pair outside the map", `{"resource":"channel","operation":"post"}`, "", n8nimport.FindUnmappedOperation, "parameters.operation"},
		{"resource by expression", `{"resource":"={{ $json.r }}","operation":"post"}`, "", n8nimport.FindSelectorExpression, "parameters.resource"},
		{"operation by expression", `{"resource":"message","operation":"={{ $json.o }}"}`, "", n8nimport.FindSelectorExpression, "parameters.operation"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			n := slackNode(t, "2.2", tc.params)
			m := n8nimport.MapNode(&n, cat)
			if m.Operation != tc.operation {
				t.Errorf("operation = %q, want %q", m.Operation, tc.operation)
			}
			if tc.code == "" {
				if !m.Mapped() || len(m.Findings) != 0 || m.OrconType != "slack.action@1" {
					t.Fatalf("expected a clean mapping, got %+v", m)
				}
				return
			}
			if m.Mapped() {
				t.Fatalf("the node must stay unmapped so the draft cannot publish: %+v", m)
			}
			if len(m.Findings) != 1 || m.Findings[0].Code != tc.code || m.Findings[0].Field != tc.field {
				t.Fatalf("findings = %+v, want one %s on %s", m.Findings, tc.code, tc.field)
			}
		})
	}
}

func TestTrackTimeSavedIsAdaptedWithANote(t *testing.T) {
	n := importNode(t, `{"id":"t","name":"Saved","type":"n8n-nodes-base.timeSaved","typeVersion":1,"parameters":{"minutesSaved":5}}`)
	m := n8nimport.MapNode(&n, nil)
	if m.OrconType != "orcon.noop@1" || !m.Mapped() || m.ReviewRequired() {
		t.Fatalf("mapping = %+v", m)
	}
	if len(m.Findings) != 1 || m.Findings[0].Code != n8nimport.FindAdapted || !strings.Contains(m.Findings[0].Message, "does not record the minutes saved") {
		t.Errorf("the report must say Orcon does not record the minutes: %+v", m.Findings)
	}
}
