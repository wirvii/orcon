// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// httpCatalog declares the credential type of the fixture, so the converter
// can point the node at it. The installed catalog declares none yet.
const httpCatalog = `{"connectors":[{"id":"http","effort":"DECLARATIVE",
  "credentials":{"httpHeaderAuth":"http.header"},"units":[]}]}`

func convertHTTPFixture(t *testing.T) *Conversion {
	t.Helper()
	cat, err := ParseCatalog([]byte(httpCatalog))
	if err != nil {
		t.Fatal(err)
	}
	conv, err := ConvertWorkflow(readWorkflow(t, "http-request"), testFlowID, cat)
	if err != nil {
		t.Fatal(err)
	}
	return conv
}

// nodeParams returns the parameters of a converted node by its name.
func nodeParams(t *testing.T, conv *Conversion, name string) jsonvalue.Value {
	t.Helper()
	for i := range conv.Document.Nodes {
		n := conv.Document.Nodes[i]
		if n.Name == name {
			return n.Parameters
		}
	}
	t.Fatalf("no converted node named %q", name)
	return jsonvalue.Value{}
}

// TestConvertHTTPOutcomes checks each HTTP Request case of the fixture.
func TestConvertHTTPOutcomes(t *testing.T) {
	conv := convertHTTPFixture(t)
	tests := []struct {
		node    string
		outcome Outcome
		orcon   string
		fields  []string
	}{
		{"Get quotes", OutcomeConverted, typeOrconHTTP, nil},
		{"Create ticket", OutcomeNeedsReview, typeOrconHTTP, []string{"credentials.httpHeaderAuth", "parameters.method"}},
		{"Dynamic url", OutcomeUnsupported, "", []string{"parameters.url"}},
		{"Expression method", OutcomeUnsupported, "", []string{"parameters.method"}},
		{"Form upload", OutcomeUnsupported, "", []string{"parameters.contentType"}},
		{"With timeout", OutcomeUnsupported, "", []string{"parameters.options"}},
		{"Delete record", OutcomeNeedsReview, typeOrconHTTP, []string{"parameters.method"}},
		{"Old version", OutcomeUnsupported, "", []string{"typeVersion"}},
	}
	for _, tt := range tests {
		t.Run(tt.node, func(t *testing.T) {
			o := outcomeOf(t, conv, tt.node)
			if o.Outcome != tt.outcome {
				t.Errorf("outcome %s, want %s", o.Outcome, tt.outcome)
			}
			if o.OrconType != tt.orcon {
				t.Errorf("orcon type %q, want %q", o.OrconType, tt.orcon)
			}
			if got := fieldsOf(o.Findings); strings.Join(got, "|") != strings.Join(tt.fields, "|") {
				t.Errorf("finding fields %v, want %v", got, tt.fields)
			}
		})
	}
}

// TestConvertHTTPParameters checks what the converted nodes carry.
func TestConvertHTTPParameters(t *testing.T) {
	conv := convertHTTPFixture(t)
	get := marshalParams(t, nodeParams(t, conv, "Get quotes"))
	if want := `{"method":"GET","url":"https://api.example.test/quotes?status=open&limit=20","headers":{"Accept":"application/json"}}`; get != want {
		t.Errorf("Get quotes parameters\n got %s\nwant %s", get, want)
	}
	ticket := marshalParams(t, nodeParams(t, conv, "Create ticket"))
	want := `{"method":"POST","url":"https://api.example.test/tickets","body":{"title":"Lead","priority":2},"credential":"Ticket token"}`
	if ticket != want {
		t.Errorf("Create ticket parameters\n got %s\nwant %s", ticket, want)
	}
}

// TestConvertHTTPNeedsSecretAndReview checks the credential reference, its
// finding, and the review flag of a write method.
func TestConvertHTTPNeedsSecretAndReview(t *testing.T) {
	conv := convertHTTPFixture(t)
	ticket := outcomeOf(t, conv, "Create ticket")
	if !hasCode(ticket.Findings, FindNeedsSecret) {
		t.Errorf("no NEEDS_SECRET finding on the credential")
	}
	if !hasCode(ticket.Findings, FindReviewRequired) {
		t.Errorf("a POST is not marked for review")
	}
	for i := range conv.Document.Nodes {
		n := conv.Document.Nodes[i]
		if n.Name != "Create ticket" {
			continue
		}
		imp, ok := n.Ext.Get(flow.ImportExtKey)
		if !ok {
			t.Fatal("no orcon.import member")
		}
		if rs, _ := imp.Get("reviewStatus"); rs.Kind() != jsonvalue.KindString {
			t.Errorf("reviewStatus missing on a write node")
		}
	}
	if s, err := conv.FlowJSON(); err != nil || !bytes.Contains(s, []byte("Ticket token")) {
		t.Errorf("the credential reference name is missing from the flow (err %v)", err)
	}
}

// TestConvertHTTPWithoutCatalogTypeIsUnsupported checks that the installed
// catalog, which declares no credential type, leaves an authenticated node
// unsupported rather than dropping its credential.
func TestConvertHTTPWithoutCatalogTypeIsUnsupported(t *testing.T) {
	conv := convertFixture(t, "http-request")
	o := outcomeOf(t, conv, "Create ticket")
	if o.Outcome != OutcomeUnsupported {
		t.Errorf("outcome %s, want UNSUPPORTED", o.Outcome)
	}
	if !hasCode(o.Findings, FindUnsupportedParameter) {
		t.Errorf("no finding names the missing credential type")
	}
}

// TestConvertHTTPGoldenFlow pins the converted HTTP Request nodes of the
// fixture with the test catalog.
func TestConvertHTTPGoldenFlow(t *testing.T) {
	got, err := convertHTTPFixture(t).FlowJSON()
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "golden", "http-request.flow.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("converted flow differs from testdata/golden/http-request.flow.json")
	}
}

// TestConvertHTTPSecretLiteralStaysOut checks that a literal secret header keeps
// the node unsupported and its value out of the flow.
func TestConvertHTTPSecretLiteralStaysOut(t *testing.T) {
	data := []byte(`{"name":"Literal secret","nodes":[
		{"id":"a","name":"Call api","type":"n8n-nodes-base.httpRequest","typeVersion":4.2,
		 "parameters":{"method":"GET","url":"https://api.example.test/x","sendHeaders":true,"specifyHeaders":"keypair",
		  "headerParameters":{"parameters":[{"name":"Authorization","value":"Bearer PLANTEDbearerValue123"}]},"options":{}}}],"connections":{}}`)
	conv, err := ConvertWorkflow(data, testFlowID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if o := outcomeOf(t, conv, "Call api"); o.Outcome != OutcomeUnsupported {
		t.Fatalf("outcome %s, want UNSUPPORTED", o.Outcome)
	}
	out, err := conv.FlowJSON()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "PLANTEDbearerValue123") {
		t.Errorf("a literal secret reached the converted flow")
	}
}

func marshalParams(t *testing.T, v jsonvalue.Value) string {
	t.Helper()
	b, err := jsonvalue.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestConvertHTTPResponseFormat checks that the response format option of n8n
// becomes the responseFormat parameter, and that what cannot be kept leaves the
// node unsupported.
func TestConvertHTTPResponseFormat(t *testing.T) {
	node := func(options string) []byte {
		return []byte(`{"name":"Format","nodes":[
		{"id":"a","name":"Page","type":"n8n-nodes-base.httpRequest","typeVersion":4.2,
		 "parameters":{"method":"GET","url":"https://example.test/page","options":` + options + `}}],"connections":{}}`)
	}
	response := func(inner string) string { return `{"response":{"response":` + inner + `}}` }
	tests := []struct {
		name    string
		options string
		want    string
		field   string
	}{
		{"json", response(`{"responseFormat":"json"}`), `{"method":"GET","url":"https://example.test/page","responseFormat":"json"}`, ""},
		{"text", response(`{"responseFormat":"text"}`), `{"method":"GET","url":"https://example.test/page","responseFormat":"text"}`, ""},
		{"file", response(`{"responseFormat":"file","outputPropertyName":"data"}`), `{"method":"GET","url":"https://example.test/page","responseFormat":"file"}`, ""},
		{"autodetect is the default", response(`{"responseFormat":"autodetect"}`), `{"method":"GET","url":"https://example.test/page"}`, ""},
		{"no options", `{}`, `{"method":"GET","url":"https://example.test/page"}`, ""},
		{"other property name", response(`{"responseFormat":"file","outputPropertyName":"report"}`), "", "parameters.options.response.response.outputPropertyName"},
		{"never error", response(`{"neverError":true}`), "", "parameters.options.response.response.neverError"},
		{"expression format", response(`{"responseFormat":"={{ $json.f }}"}`), "", "parameters.options.response.response.responseFormat"},
		{"unknown format", response(`{"responseFormat":"xml"}`), "", "parameters.options.response.response.responseFormat"},
		{"timeout beside the format", `{"timeout":5000,"response":{"response":{"responseFormat":"text"}}}`, "", "parameters.options"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conv, err := ConvertWorkflow(node(tt.options), testFlowID, nil)
			if err != nil {
				t.Fatal(err)
			}
			o := outcomeOf(t, conv, "Page")
			if tt.field != "" {
				if o.Outcome != OutcomeUnsupported || strings.Join(fieldsOf(o.Findings), "|") != tt.field {
					t.Fatalf("outcome %s with fields %v, want UNSUPPORTED at %s", o.Outcome, fieldsOf(o.Findings), tt.field)
				}
				return
			}
			if o.Outcome != OutcomeConverted {
				t.Fatalf("outcome %s, want CONVERTED (%v)", o.Outcome, fieldsOf(o.Findings))
			}
			if got := marshalParams(t, nodeParams(t, conv, "Page")); got != tt.want {
				t.Errorf("parameters\n got %s\nwant %s", got, tt.want)
			}
		})
	}
}
