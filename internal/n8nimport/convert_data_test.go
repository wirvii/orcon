// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package n8nimport

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestConvertSetAndNoOpGolden pins the whole converted flow of the Edit Fields
// and No Operation fixture. A change to the output is a change to the flow
// format that a person must review, so the golden file is updated by hand.
func TestConvertSetAndNoOpGolden(t *testing.T) {
	conv := convertFixture(t, "set-and-noop")
	got, err := conv.FlowJSON()
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "golden", "set-and-noop.flow.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("converted flow differs from testdata/golden/set-and-noop.flow.json")
	}
}

// TestConvertSetAndNoOpOutcomes checks the outcome and the finding fields of
// each Edit Fields and No Operation node of the fixture.
func TestConvertSetAndNoOpOutcomes(t *testing.T) {
	conv := convertFixture(t, "set-and-noop")
	tests := []struct {
		node     string
		outcome  Outcome
		orcon    string
		findings []string // field of each finding, in order
	}{
		{"Pass through", OutcomeConverted, typeOrconNoop, nil},
		{"Shape lead", OutcomeAdapted, typeOrconSet, []string{"parameters.assignments.assignments[3].value"}},
		{"Tag contact", OutcomeAdapted, typeOrconSet, []string{"parameters.assignments.assignments[1].value"}},
		{"Raw output", OutcomeUnsupported, "", []string{"parameters.mode"}},
		{"Dotted field", OutcomeUnsupported, "", []string{"parameters.assignments.assignments[0].name"}},
		{"Old fields", OutcomeUnsupported, "", []string{"typeVersion"}},
		{"Typed mismatch", OutcomeUnsupported, "", []string{"parameters.assignments.assignments[0].value"}},
		{"Paused step", OutcomeAdapted, typeOrconNoop, []string{"disabled"}},
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
			if got := fieldsOf(o.Findings); strings.Join(got, "|") != strings.Join(tt.findings, "|") {
				t.Errorf("finding fields %v, want %v", got, tt.findings)
			}
		})
	}
}

// TestConvertSetKeepsExpressionsAndDropsNothing checks that the converted Set
// nodes keep every field, with its expression text as written.
func TestConvertSetKeepsExpressionsAndDropsNothing(t *testing.T) {
	conv := convertFixture(t, "set-and-noop")
	out, err := conv.FlowJSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`{"name":"greeting","value":"=Hello {{ $json.name }}"}`,
		`{"name":"meta","value":{"channel":"=whatsapp","fixed":1}}`,
		`{"name":"tags","value":["lead","whatsapp"]}`,
		`"includeOtherFields":true`,
	} {
		if !strings.Contains(strings.ReplaceAll(string(out), " ", ""), strings.ReplaceAll(want, " ", "")) {
			t.Errorf("converted flow lacks %s", want)
		}
	}
}

// TestConvertSetReportExplainsEachNode checks the report rows of the Set and
// No Operation nodes, the text a person reads to decide what to rebuild.
func TestConvertSetReportExplainsEachNode(t *testing.T) {
	md := convertFixture(t, "set-and-noop").Markdown()
	for _, want := range []string{
		"| Pass through | `n8n-nodes-base.noOp` v1 | CONVERTED | `orcon.noop@1` |",
		"| Shape lead | `n8n-nodes-base.set` v3.4 | ADAPTED | `orcon.set@1` |",
		"| Raw output | `n8n-nodes-base.set` v3.4 | UNSUPPORTED |",
		"only the manual mode is converted",
		"a field name must be a literal name without dots",
		"does not match its declared type number",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("report lacks %q", want)
		}
	}
}

// TestConvertSetRejectsSecretsInFields checks that a Set node whose fields
// carry a secret stays unsupported and its value is not in the flow.
func TestConvertSetRejectsSecretsInFields(t *testing.T) {
	data := []byte(`{"name":"Secret set","nodes":[
		{"id":"a","name":"Keep token","type":"n8n-nodes-base.set","typeVersion":3.4,
		 "parameters":{"mode":"manual","includeOtherFields":false,
		  "assignments":{"assignments":[{"id":"x","name":"accessToken","value":"PLANTEDaccessTokenValue","type":"string"}]},
		  "options":{}}}],"connections":{}}`)
	conv, err := ConvertWorkflow(data, testFlowID, nil)
	if err != nil {
		t.Fatal(err)
	}
	o := outcomeOf(t, conv, "Keep token")
	if o.Outcome != OutcomeUnsupported {
		t.Fatalf("outcome %s, want UNSUPPORTED", o.Outcome)
	}
	if !hasCode(o.Findings, FindSecretValue) {
		t.Errorf("no SECRET_VALUE finding")
	}
	out, err := conv.FlowJSON()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "PLANTEDaccessTokenValue") {
		t.Errorf("a secret value reached the converted flow")
	}
}
