// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
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
