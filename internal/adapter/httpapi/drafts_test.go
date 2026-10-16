// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

// draftRig is the draft handlers over the rig of the project handlers, with one flow in it.
type draftRig struct {
	*projectsRig
	drafts *Drafts
	flowID apigen.FlowId
}

func newDraftRig(t *testing.T) *draftRig {
	t.Helper()
	r := newProjectsRig(t)
	proj, err := r.createProject("Acme", "")
	if err != nil {
		t.Fatal(err)
	}
	created, err := r.h.CreateFlow(r.ctx, apigen.CreateFlowRequestObject{ProjectId: *proj.Body.Id, Body: &apigen.CreateFlowRequest{Name: "Welcome"}})
	if err != nil {
		t.Fatal(err)
	}
	made, ok := created.(apigen.CreateFlow201JSONResponse)
	if !ok {
		t.Fatalf("create flow = %T", created)
	}
	fid := *made.Body.Id
	return &draftRig{projectsRig: r, drafts: NewDrafts(flow.NewDrafts(r.grants.Memory, r.clk, id.NewSequence())), flowID: fid}
}

func document(t *testing.T, raw string) apigen.FlowDocument {
	t.Helper()
	var doc apigen.FlowDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

const oneNodeDoc = `{"format":"orcon/flow@1","id":"01930000-0000-7000-8000-00000000000a","name":"Welcome","settings":{},"variables":[],
"nodes":[{"id":"start","name":"Start","element":"startEvent","type":"orcon.start@1","parameters":{}}],"edges":[],"editor":{}}`

func (r *draftRig) replace(t *testing.T, ifMatch, raw string) (apigen.ReplaceDraftResponseObject, error) { //nolint:ireturn // the generated interface requires it
	t.Helper()
	doc := document(t, raw)
	return r.drafts.ReplaceDraft(r.ctx, apigen.ReplaceDraftRequestObject{
		FlowId: r.flowID, Params: apigen.ReplaceDraftParams{IfMatch: ifMatch}, Body: &apigen.ReplaceDraftRequest{Document: doc},
	})
}

// answer renders err the way the router does.
func answer(t *testing.T, err error) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	errorHandler{}.response(w, httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/flows/x/draft", nil), err)
	return w
}

func TestGetDraftReturnsTheEmptyDraftOfANewFlowWithItsETag(t *testing.T) {
	r := newDraftRig(t)
	resp, err := r.drafts.GetDraft(r.ctx, apigen.GetDraftRequestObject{FlowId: r.flowID})
	if err != nil {
		t.Fatal(err)
	}
	ok, isOK := resp.(apigen.GetDraft200JSONResponse)
	if !isOK || ok.Body.Revision != 0 || len(ok.Body.Document.Nodes) != 0 || *ok.Headers.ETag != `"0"` {
		t.Fatalf("response = %#v, want revision 0, no nodes and ETag \"0\"", resp)
	}
}

func TestGetDraftOfAnUnknownFlowIsNotFound(t *testing.T) {
	r := newDraftRig(t)
	_, err := r.drafts.GetDraft(r.ctx, apigen.GetDraftRequestObject{FlowId: apigen.FlowId(id.NewSequence().New())})
	if !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestReplaceDraftSavesAndMovesTheRevision(t *testing.T) {
	r := newDraftRig(t)
	resp, err := r.replace(t, `"0"`, oneNodeDoc)
	if err != nil {
		t.Fatal(err)
	}
	ok, isOK := resp.(apigen.ReplaceDraft200JSONResponse)
	if !isOK || ok.Body.Revision != 1 || *ok.Headers.ETag != `"1"` || len(ok.Body.Document.Nodes) != 1 {
		t.Fatalf("saved = %+v, want revision 1 with one node", ok)
	}
	got, err := r.drafts.GetDraft(r.ctx, apigen.GetDraftRequestObject{FlowId: r.flowID})
	read, isRead := got.(apigen.GetDraft200JSONResponse)
	if err != nil || !isRead || len(read.Body.Document.Nodes) != 1 {
		t.Fatalf("read back = %#v, %v", got, err)
	}
}

func TestReplaceDraftWithAStaleRevisionIs412WithTheCurrentRevision(t *testing.T) {
	r := newDraftRig(t)
	if _, err := r.replace(t, `"0"`, oneNodeDoc); err != nil {
		t.Fatal(err)
	}
	for _, tag := range []string{`"0"`, `W/"1"`, `*`} {
		_, err := r.replace(t, tag, oneNodeDoc)
		var stale *staleDraftError
		if !errors.As(err, &stale) || stale.revision != 1 {
			t.Fatalf("If-Match %s: err = %v, want a stale draft at revision 1", tag, err)
		}
	}
	_, err := r.replace(t, `"0"`, oneNodeDoc)
	w := answer(t, err)
	var body struct {
		Code                 string `json:"code"`
		CurrentDraftRevision int    `json:"currentDraftRevision"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusPreconditionFailed || w.Header().Get("ETag") != `"1"` || body.CurrentDraftRevision != 1 || body.Code != "precondition-failed" {
		t.Fatalf("answer = %d %s %v, want 412 with ETag \"1\" and currentDraftRevision 1", w.Code, w.Header().Get("ETag"), body)
	}
}

func TestReplaceDraftOfAnUnknownFlowIsNotFound(t *testing.T) {
	r := newDraftRig(t)
	r.flowID = apigen.FlowId(id.NewSequence().New())
	if _, err := r.replace(t, `"0"`, oneNodeDoc); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestReplaceDraftWithABrokenDocumentAnswersFieldErrors(t *testing.T) {
	r := newDraftRig(t)
	_, err := r.replace(t, `"0"`, `{"format":"orcon/flow@1","nodes":[{"id":"a"}],"edges":[]}`)
	var issues *documentIssuesError
	if !errors.As(err, &issues) || len(issues.fields) == 0 || !strings.HasPrefix(issues.fields[0].Pointer, "/document") {
		t.Fatalf("err = %v, want field errors under /document", err)
	}
	if w := answer(t, err); w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	got, _ := r.drafts.GetDraft(r.ctx, apigen.GetDraftRequestObject{FlowId: r.flowID})
	if read, isRead := got.(apigen.GetDraft200JSONResponse); !isRead || read.Body.Revision != 0 {
		t.Error("a refused save moved the revision")
	}
}

func TestReplaceDraftWithAnUnknownFormatAnswersAFieldError(t *testing.T) {
	r := newDraftRig(t)
	_, err := r.replace(t, `"0"`, `{"format":"orcon/flow@99","nodes":[],"edges":[]}`)
	var issues *documentIssuesError
	if !errors.As(err, &issues) || issues.fields[0].Pointer != "/document/format" {
		t.Fatalf("err = %v, want a field error at /document/format", err)
	}
}
