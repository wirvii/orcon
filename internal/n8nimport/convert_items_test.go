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

// TestConvertItemListsOutcomes checks the outcome, the Orcon type and the
// finding fields of each Filter, Sort and Limit node of the fixture.
func TestConvertItemListsOutcomes(t *testing.T) {
	conv := convertFixture(t, "item-lists")
	tests := []struct {
		node     string
		outcome  Outcome
		orcon    string
		findings []string
	}{
		{"Adults", OutcomeConverted, typeOrconFilter, nil},
		{"Old filter", OutcomeUnsupported, "", []string{"typeVersion"}},
		{"Odd option", OutcomeUnsupported, "", []string{"parameters.options.keepDiscarded"}},
		{"Newest first", OutcomeAdapted, typeOrconSort, []string{"parameters.sortFieldsUi", "parameters.options.disableDotNotation"}},
		{"Shuffle", OutcomeConverted, typeOrconSort, nil},
		{"Sort by code", OutcomeUnsupported, "", []string{"parameters.type"}},
		{"Sort twice", OutcomeUnsupported, "", []string{"parameters.sortFieldsUi.sortField[1].fieldName"}},
		{"Last five", OutcomeConverted, typeOrconLimit, nil},
		{"First one", OutcomeConverted, typeOrconLimit, nil},
		{"Dynamic limit", OutcomeUnsupported, "", []string{"parameters.maxItems"}},
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

// TestConvertItemListsParameters checks the parameters the converted nodes
// carry: the filter condition as one boolean template, with the case option
// applied, and the explicit orders and limits.
func TestConvertItemListsParameters(t *testing.T) {
	out, err := convertFixture(t, "item-lists").FlowJSON()
	if err != nil {
		t.Fatal(err)
	}
	flat := strings.ReplaceAll(string(out), " ", "")
	for _, want := range []string{
		// Ignore case lowers both sides of the string comparison.
		`"condition":"={{(Number(($json.age))>=Number(18))&&(String(($json.country)).toLowerCase()===String(\"co\").toLowerCase())}}"`,
		`"fields":[{"field":"created.at","order":"DESCENDING"},{"field":"name","order":"ASCENDING"}]`,
		`"mode":"RANDOM"`,
		`"maxItems":5,"keep":"LAST_ITEMS"`,
		`"maxItems":1,"keep":"FIRST_ITEMS"`,
	} {
		if !strings.Contains(flat, strings.ReplaceAll(want, " ", "")) {
			t.Errorf("converted flow lacks %s\n%s", want, out)
		}
	}
}

// TestConvertItemListsGolden pins the whole converted flow of the fixture. A
// change to the output is a change to the flow format that a person must
// review, so the golden file is updated by hand.
func TestConvertItemListsGolden(t *testing.T) {
	got, err := convertFixture(t, "item-lists").FlowJSON()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", "golden", "item-lists.flow.json")
	if os.Getenv("ORCON_UPDATE_GOLDEN") != "" {
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path) //nolint:gosec // G304: fixed test data directory
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("converted flow differs from %s", path)
	}
}

func TestConvertItemListsReportExplainsEachNode(t *testing.T) {
	md := convertFixture(t, "item-lists").Markdown()
	for _, want := range []string{
		"| Adults | `n8n-nodes-base.filter` v2.2 | CONVERTED | `orcon.filter@1` |",
		"| Newest first | `n8n-nodes-base.sort` v1 | ADAPTED | `orcon.sort@1` |",
		"the code type sorts with JavaScript, which needs the Code node",
		"the field a is listed twice",
		"the maximum must be a whole number of at least 1 and not an expression",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("report lacks %q", want)
		}
	}
}

// TestConvertFilterKeepsASecretOut checks that a Filter whose comparison value
// looks like a secret stays unsupported and the value is not in the flow.
func TestConvertFilterKeepsASecretOut(t *testing.T) {
	data := []byte(`{"name":"Secret filter","nodes":[
		{"id":"a","name":"Check token","type":"n8n-nodes-base.filter","typeVersion":2,
		 "parameters":{"conditions":{"conditions":[{"id":"c","leftValue":"={{ $json.token }}","rightValue":"sk-PLANTEDvalue0123456789","operator":{"type":"string","operation":"equals"}}],"combinator":"and"}}}],"connections":{}}`)
	conv, err := ConvertWorkflow(data, testFlowID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if o := outcomeOf(t, conv, "Check token"); o.Outcome != OutcomeUnsupported {
		t.Fatalf("outcome %s, want UNSUPPORTED", o.Outcome)
	}
	out, err := conv.FlowJSON()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "sk-PLANTEDvalue0123456789") {
		t.Errorf("a secret value reached the converted flow")
	}
}
