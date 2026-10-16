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

// TestConvertListNodesOutcomes checks the outcome, the Orcon type and the
// finding fields of each Split Out, Aggregate and Remove Duplicates node of
// the fixture.
func TestConvertListNodesOutcomes(t *testing.T) {
	conv := convertFixture(t, "list-nodes")
	tests := []struct {
		node     string
		outcome  Outcome
		orcon    string
		findings []string
	}{
		{"Split tags", OutcomeConverted, typeOrconSplitOut, nil},
		{"Split people", OutcomeAdapted, typeOrconSplitOut, []string{"parameters.fieldToSplitOut"}},
		{"Split two", OutcomeUnsupported, "", []string{"parameters.fieldToSplitOut"}},
		{"Split binary", OutcomeUnsupported, "", []string{"parameters.options.includeBinary"}},
		{"Split selected", OutcomeConverted, typeOrconSplitOut, nil},
		{"Gather names", OutcomeConverted, typeOrconAggregate, nil},
		{"Gather all", OutcomeConverted, typeOrconAggregate, nil},
		{"Gather dotted", OutcomeAdapted, typeOrconAggregate, []string{"parameters.options.disableDotNotation"}},
		{"Gather unique", OutcomeUnsupported, "", []string{"parameters.options.keepOnlyUnique"}},
		{"Dedupe all", OutcomeConverted, typeOrconRemoveDuplicates, nil},
		{"Dedupe email", OutcomeConverted, typeOrconRemoveDuplicates, nil},
		{"Dedupe history", OutcomeUnsupported, "", []string{"parameters.operation"}},
		{"Dedupe except", OutcomeAdapted, typeOrconRemoveDuplicates, []string{"parameters.options.disableDotNotation"}},
		{"Dedupe nothing named", OutcomeUnsupported, "", []string{"parameters.fieldsToCompare"}},
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

// TestConvertListNodesParameters checks the parameters the converted nodes
// carry.
func TestConvertListNodesParameters(t *testing.T) {
	out, err := convertFixture(t, "list-nodes").FlowJSON()
	if err != nil {
		t.Fatal(err)
	}
	flat := strings.ReplaceAll(string(out), " ", "")
	for _, want := range []string{
		`"field":"tags","destinationField":"tag","include":"ALL_OTHER_FIELDS"`,
		`"field":"lines","destinationField":"line","include":"SELECTED_OTHER_FIELDS","includeFields":["id","customer.name"]`,
		`"mode":"INDIVIDUAL_FIELDS","fields":[{"field":"name","outputField":"names"},{"field":"tags"}],"mergeLists":true,"keepMissing":true`,
		`"mode":"ALL_ITEM_DATA","destinationField":"rows","include":"SELECTED_FIELDS","includeFields":["a","b"]`,
		`"compare":"ALL_FIELDS"`,
		`"compare":"SELECTED_FIELDS","fields":["email"],"removeOtherFields":true`,
		`"compare":"ALL_FIELDS_EXCEPT","fields":["seenAt","meta.at"]`,
	} {
		if !strings.Contains(flat, strings.ReplaceAll(want, " ", "")) {
			t.Errorf("converted flow lacks %s", want)
		}
	}
}

// TestConvertListNodesGolden pins the whole converted flow of the fixture. A
// change to the output is a change to the flow format that a person must
// review, so the golden file is updated by hand.
func TestConvertListNodesGolden(t *testing.T) {
	got, err := convertFixture(t, "list-nodes").FlowJSON()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", "golden", "list-nodes.flow.json")
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

func TestConvertListNodesReportExplainsEachNode(t *testing.T) {
	md := convertFixture(t, "list-nodes").Markdown()
	for _, want := range []string{
		"| Split tags | `n8n-nodes-base.splitOut` v1 | CONVERTED | `orcon.splitOut@1` |",
		"| Gather dotted | `n8n-nodes-base.aggregate` v1 | ADAPTED | `orcon.aggregate@1` |",
		"remembering items across executions needs flow data",
		"several fields at once are not converted",
		"the Aggregate option keepOnlyUnique is not converted",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("report lacks %q", want)
		}
	}
}
