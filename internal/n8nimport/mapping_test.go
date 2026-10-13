// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport_test

import (
	"errors"
	"fmt"
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
		{"pair outside the map", slackNode(t, "2.2", `{"resource":"message","operation":"delete"}`), n8nimport.FindUnmappedOperation, "parameters.operation"},
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
			imp, ok := m.Ext.Get(n8nimport.ImportExtKey)
			if !ok {
				t.Fatalf("ext has no %s: %v", n8nimport.ImportExtKey, m.Ext)
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
	}{
		{"GET", `{"method":"GET","url":"https://example.test"}`, n8nimport.SafetyRead, false, ""},
		{"HEAD in lower case", `{"method":"head"}`, n8nimport.SafetyRead, false, ""},
		{"method left out", `{"url":"https://example.test"}`, n8nimport.SafetyRead, false, ""},
		{"POST", `{"method":"POST"}`, n8nimport.SafetyUnsafe, true, "parameters.method"},
		{"PUT", `{"method":"PUT"}`, n8nimport.SafetyUnsafe, true, "parameters.method"},
		{"PATCH", `{"method":"PATCH"}`, n8nimport.SafetyUnsafe, true, "parameters.method"},
		{"DELETE", `{"method":"DELETE"}`, n8nimport.SafetyUnsafe, true, "parameters.method"},
		{"post in lower case", `{"method":"post"}`, n8nimport.SafetyUnsafe, true, "parameters.method"},
		{"method by expression", `{"method":"={{ $json.verb }}"}`, n8nimport.SafetyUnsafe, true, "parameters.method"},
		{"typeVersion 1 spelling", `{"requestMethod":"POST"}`, n8nimport.SafetyUnsafe, true, "parameters.requestMethod"},
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
			imp, _ := m.Ext.Get(n8nimport.ImportExtKey)
			review, hasReview := imp.Get("review")
			if hasReview != tc.review {
				t.Fatalf("ext review mark = %v, want %v", hasReview, tc.review)
			}
			if tc.review {
				if req, _ := review.Get("required"); !isTrue(req) {
					t.Errorf("review.required = %v", req)
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

func isTrue(v jsonvalue.Value) bool {
	b, ok := v.Bool()
	return ok && b
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

	err := n8nimport.CheckActivation(doc)
	var re *n8nimport.ReviewError
	if !errors.Is(err, n8nimport.ErrReviewRequired) || !errors.As(err, &re) {
		t.Fatalf("activation of a flow with unreviewed writes: %v", err)
	}
	if got := fmt.Sprint(re.Nodes); got != "[w d]" {
		t.Errorf("pending = %s, want [w d]", got)
	}
	if !strings.Contains(err.Error(), "w, d") {
		t.Errorf("the error does not name the nodes: %v", err)
	}

	if !n8nimport.MarkReviewed(doc, "w") {
		t.Fatal("reviewing w failed")
	}
	if n8nimport.MarkReviewed(doc, "w") {
		t.Error("a second review of w must report false")
	}
	if n8nimport.MarkReviewed(doc, "r") || n8nimport.MarkReviewed(doc, "missing") {
		t.Error("a node without a mark, or a missing one, cannot be reviewed")
	}
	if err := n8nimport.CheckActivation(doc); !errors.Is(err, n8nimport.ErrReviewRequired) {
		t.Fatalf("d is still unreviewed, got %v", err)
	}
	if !n8nimport.MarkReviewed(doc, "d") {
		t.Fatal("reviewing d failed")
	}
	if err := n8nimport.CheckActivation(doc); err != nil {
		t.Fatalf("every marked node was reviewed, got %v", err)
	}
	// The reasons stay in the node after the review, as an audit trail.
	imp, _ := doc.Nodes[0].Ext.Get(n8nimport.ImportExtKey)
	review, _ := imp.Get("review")
	if reasons, _ := review.Get("reasons"); reasons.Len() != 1 {
		t.Errorf("review reasons were lost: %v", review)
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
