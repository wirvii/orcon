// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

func importExt(t *testing.T, mark string) jsonvalue.Value {
	t.Helper()
	v, err := jsonvalue.Parse([]byte(`{"orcon.import":` + mark + `}`))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestImportExtShape(t *testing.T) {
	const base = `"n8nType":"n8n-nodes-base.httpRequest","n8nTypeVersion":4.2`
	tests := []struct {
		name     string
		mark     string
		nodeType string
		want     string // "" means valid
	}{
		{"closed shape, full", `{` + base + `,"n8nNodeId":"a1","reviewStatus":"NEEDS_REVIEW","reviewReasons":["HTTP_WRITE"]}`, "", ""},
		{"without review", `{` + base + `}`, "", ""},
		{"reviewed keeps the reasons", `{` + base + `,"reviewStatus":"REVIEWED","reviewReasons":["HTTP_METHOD_EXPRESSION","UNSAFE_OPERATION"]}`, "", ""},
		{"original on an unsupported node", `{` + base + `,"original":{"type":"x"}}`, "orcon.unsupported@1", ""},
		{"original on another node", `{` + base + `,"original":{"type":"x"}}`, "", "original"},
		{"missing type", `{"n8nTypeVersion":1}`, "", "n8nType"},
		{"missing version", `{"n8nType":"x"}`, "", "n8nTypeVersion"},
		{"version is a string", `{"n8nType":"x","n8nTypeVersion":"4.2"}`, "", "n8nTypeVersion"},
		{"empty node id", `{` + base + `,"n8nNodeId":""}`, "", "n8nNodeId"},
		{"unknown member", `{` + base + `,"extra":1}`, "", "extra"},
		{"status without reasons", `{` + base + `,"reviewStatus":"NEEDS_REVIEW"}`, "", "together"},
		{"reasons without status", `{` + base + `,"reviewReasons":["HTTP_WRITE"]}`, "", "together"},
		{"status outside the set", `{` + base + `,"reviewStatus":"needs_review","reviewReasons":["HTTP_WRITE"]}`, "", "reviewStatus"},
		{"reason outside the set", `{` + base + `,"reviewStatus":"NEEDS_REVIEW","reviewReasons":["HTTP_POST"]}`, "", "reviewReasons"},
		{"empty reasons", `{` + base + `,"reviewStatus":"NEEDS_REVIEW","reviewReasons":[]}`, "", "non-empty"},
		{"repeated reason", `{` + base + `,"reviewStatus":"NEEDS_REVIEW","reviewReasons":["HTTP_WRITE","HTTP_WRITE"]}`, "", "repeats"},
		{"not an object", `[]`, "", "object"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := load(t, "linear")
			doc.Nodes[1].Ext = importExt(t, tc.mark)
			if tc.nodeType != "" {
				doc.Nodes[1].Type = tc.nodeType
			}
			report := flow.Validate(&doc)
			var sb strings.Builder
			for _, e := range report.Errors() {
				sb.WriteString(e.Path + " " + e.Message + "\n")
			}
			msgs := sb.String()
			if tc.want == "" {
				if report.HasErrors() {
					t.Fatalf("a valid mark was rejected:\n%s", msgs)
				}
				return
			}
			if !strings.Contains(msgs, tc.want) || !strings.Contains(msgs, "/nodes/1/ext/orcon.import") {
				t.Fatalf("want an error on /nodes/1/ext/orcon.import mentioning %q, got:\n%s", tc.want, msgs)
			}
		})
	}
}

func TestImportExtOnlyWhereDefined(t *testing.T) {
	mark := `{"n8nType":"x","n8nTypeVersion":1}`
	t.Run("not in the document ext", func(t *testing.T) {
		doc := load(t, "linear")
		doc.Ext = importExt(t, mark)
		if report := flow.Validate(&doc); !report.HasErrors() {
			t.Fatal("orcon.import in the document ext must fail")
		}
	})
	t.Run("another reserved key still fails in a node", func(t *testing.T) {
		doc := load(t, "linear")
		v, err := jsonvalue.Parse([]byte(`{"orcon.mine":1}`))
		if err != nil {
			t.Fatal(err)
		}
		doc.Nodes[1].Ext = v
		report := flow.Validate(&doc)
		var found bool
		for _, e := range report.Errors() {
			found = found || e.Code == flow.CodeExt
		}
		if !found {
			t.Fatal("orcon.mine must fail with the EXT code")
		}
	})
}

func TestImportedNodesNeedReviewBeforeActivation(t *testing.T) {
	doc := load(t, "linear")
	doc.Nodes[1].Ext = importExt(t, `{"n8nType":"x","n8nTypeVersion":1,"reviewStatus":"NEEDS_REVIEW","reviewReasons":["HTTP_WRITE"]}`)
	doc.Nodes[2].Ext = importExt(t, `{"n8nType":"x","n8nTypeVersion":1}`)

	// Publication does not fail on the mark: the validator accepts the flow.
	if report := flow.Validate(&doc); report.HasErrors() {
		t.Fatalf("a marked flow must still validate: %v", report.Errors())
	}
	pending := flow.PendingImportReview(&doc)
	if len(pending) != 1 || pending[0] != doc.Nodes[1].ID {
		t.Fatalf("pending = %v", pending)
	}
	err := flow.CheckImportReview(&doc)
	var re *flow.ImportReviewError
	if !errors.Is(err, flow.ErrImportReviewRequired) || !errors.As(err, &re) || !strings.Contains(err.Error(), string(doc.Nodes[1].ID)) {
		t.Fatalf("activation check = %v", err)
	}
	if flow.MarkImportReviewed(&doc, doc.Nodes[2].ID) || flow.MarkImportReviewed(&doc, "missing") {
		t.Error("only a node marked NEEDS_REVIEW can be reviewed")
	}
	if !flow.MarkImportReviewed(&doc, doc.Nodes[1].ID) {
		t.Fatal("review failed")
	}
	if flow.MarkImportReviewed(&doc, doc.Nodes[1].ID) {
		t.Error("a second review must report false")
	}
	if err := flow.CheckImportReview(&doc); err != nil {
		t.Fatalf("after review: %v", err)
	}
	if report := flow.Validate(&doc); report.HasErrors() {
		t.Fatalf("the reviewed flow must validate (reasons kept): %v", report.Errors())
	}
	imp, _ := doc.Nodes[1].Ext.Get(flow.ImportExtKey)
	if reasons, _ := imp.Get("reviewReasons"); reasons.Len() != 1 {
		t.Error("the reasons must stay after review")
	}
}
