// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
)

// unpublished reads the flow and returns its hasUnpublishedChanges.
func (r *flowRig) unpublished(t *testing.T) bool {
	t.Helper()
	resp, err := r.flows.GetFlow(r.ctx, apigen.GetFlowRequestObject{FlowId: r.flowID})
	if err != nil {
		t.Fatal(err)
	}
	return asResp[apigen.GetFlow200JSONResponse](t, resp).Body.HasUnpublishedChanges
}

// TestGetFlowComparesTheDraftWithTheActiveVersion (SPEC-002 R-ACT-01): a flow that was never
// published has changes, a flow right after a publication or a rollback has none even though its
// draft was saved before, and a draft saved after the publication has them again.
func TestGetFlowComparesTheDraftWithTheActiveVersion(t *testing.T) {
	r := newFlowRig(t)
	if !r.unpublished(t) {
		t.Fatal("a flow that was never published reports no changes")
	}
	tag := r.saveValid(t)
	if !r.unpublished(t) {
		t.Fatal("a saved draft of a flow that was never published reports no changes")
	}
	if _, err := r.publish(t, tag, ""); err != nil {
		t.Fatal(err)
	}
	if r.unpublished(t) {
		t.Fatal("the flow reports changes right after its publication")
	}
	patch := apigen.JsonPatch{{Op: apigen.Replace, Path: "/name", Value: "Welcome v2"}}
	if _, err := r.drafts.UpdateDraft(r.ctx, apigen.UpdateDraftRequestObject{FlowId: r.flowID, Params: apigen.UpdateDraftParams{IfMatch: tag}, Body: &patch}); err != nil {
		t.Fatal(err)
	}
	if !r.unpublished(t) {
		t.Fatal("the flow reports no changes after its draft changed")
	}
	rollback, err := r.flows.RollbackFlow(r.ctx, apigen.RollbackFlowRequestObject{
		FlowId: r.flowID, Params: apigen.RollbackFlowParams{IfMatch: r.flowTag(t)}, Body: &apigen.RollbackFlowRequest{Version: 1},
	})
	if err != nil || rollback == nil {
		t.Fatalf("rollback = %v, %v", rollback, err)
	}
	if r.unpublished(t) {
		t.Fatal("the flow reports changes right after a rollback made the draft the content of version 1")
	}
}

// listedUnpublished returns hasUnpublishedChanges of the flow as the project listing shows it.
func (r *flowRig) listedUnpublished(t *testing.T) bool {
	t.Helper()
	got, err := r.flows.GetFlow(r.ctx, apigen.GetFlowRequestObject{FlowId: r.flowID})
	if err != nil {
		t.Fatal(err)
	}
	project := asResp[apigen.GetFlow200JSONResponse](t, got).Body.ProjectId
	resp, err := r.h.ListFlows(r.ctx, apigen.ListFlowsRequestObject{ProjectId: project})
	if err != nil {
		t.Fatal(err)
	}
	items := flowsOf(t, resp).Items
	if len(items) != 1 {
		t.Fatalf("listed %d flows, want 1", len(items))
	}
	return items[0].HasUnpublishedChanges
}

// TestListFlowsComparesTheDraftWithTheActiveVersion: the listing agrees with the read of one flow.
// Saving the same content again, reformatted and with its members in another order, is not a
// change; changing one value is; publishing again clears it.
func TestListFlowsComparesTheDraftWithTheActiveVersion(t *testing.T) {
	r := newFlowRig(t)
	check := func(step string, want bool) {
		t.Helper()
		if got := r.listedUnpublished(t); got != want {
			t.Fatalf("%s: listing hasUnpublishedChanges = %v, want %v", step, got, want)
		}
		if got := r.unpublished(t); got != want {
			t.Fatalf("%s: get hasUnpublishedChanges = %v, want %v", step, got, want)
		}
	}
	tag := r.saveValid(t)
	check("a saved draft that was never published", true)
	if _, err := r.publish(t, tag, ""); err != nil {
		t.Fatal(err)
	}
	check("right after publishing", false)

	var doc map[string]any
	if err := json.Unmarshal([]byte(validDoc()), &doc); err != nil {
		t.Fatal(err)
	}
	same := reverseObject(t, doc)
	resp, err := r.replace(t, tag, same)
	if err != nil {
		t.Fatal(err)
	}
	tag = *asResp[apigen.ReplaceDraft200JSONResponse](t, resp).Headers.ETag
	check("after saving the same content reformatted and reordered", false)

	doc["name"] = "Welcome v2"
	changed, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	resp, err = r.replace(t, tag, string(changed))
	if err != nil {
		t.Fatal(err)
	}
	tag = *asResp[apigen.ReplaceDraft200JSONResponse](t, resp).Headers.ETag
	check("after changing one value", true)

	if _, err := r.publish(t, tag, ""); err != nil {
		t.Fatal(err)
	}
	check("after publishing the change", false)
}

// reverseObject writes the top-level members of doc in reverse alphabetical order, indented.
func reverseObject(t *testing.T, doc map[string]any) string {
	t.Helper()
	keys := slices.Sorted(maps.Keys(doc))
	slices.Reverse(keys)
	var b strings.Builder
	b.WriteString("{\n")
	for i, k := range keys {
		name, err := json.Marshal(k)
		if err != nil {
			t.Fatal(err)
		}
		value, err := json.MarshalIndent(doc[k], "    ", "  ")
		if err != nil {
			t.Fatal(err)
		}
		b.WriteString("  " + string(name) + " :  " + string(value))
		if i < len(keys)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString("}")
	return b.String()
}
