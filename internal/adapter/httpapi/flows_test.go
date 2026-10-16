// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/oapi-codegen/runtime/types"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/conditional"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/adapter/httpapi/page"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/trigger"
	"github.com/wirvii/orcon/internal/trigger/triggertest"
)

// flowRig is the flow handlers over the in-memory store and the real trigger service, with one flow
// in it, for the signed-in user of the rig.
type flowRig struct {
	*draftRig
	flows     *Flows
	principal identity.Principal
}

func newFlowRig(t *testing.T) *flowRig {
	t.Helper()
	return newFlowRigWith(t, func(src trigger.FlowSource) trigger.FlowSource { return src })
}

// newFlowRigWith is newFlowRig with the flow source of the trigger service wrapped by wrap.
func newFlowRigWith(t *testing.T, wrap func(trigger.FlowSource) trigger.FlowSource) *flowRig {
	t.Helper()
	r := newDraftRig(t)
	mem := r.grants.Memory
	ids := id.NewSequence()
	for range 1000 {
		ids.New() // keep the identities of the service apart from those the rig created
	}
	svc, err := trigger.New(trigger.Config{
		Store: mem, Flows: wrap(mem), Catalog: triggertest.DefaultCatalog(), Credentials: &triggertest.Credentials{}, Clock: r.clk, IDs: ids,
	})
	if err != nil {
		t.Fatal(err)
	}
	principal, _ := middleware.PrincipalFrom(r.ctx)
	if err := mem.CreatePrincipal(r.ctx, identity.PrincipalRecord{ID: principal.ID, Kind: identity.PrincipalUser, CreatedAt: r.clk.Now(), UpdatedAt: r.clk.Now()}); err != nil {
		t.Fatal(err)
	}
	codec, err := page.NewCodec(make([]byte, page.MinKeyLength))
	if err != nil {
		t.Fatal(err)
	}
	flows := NewFlows(flow.NewManager(mem, r.clk, ids), svc, mem, flow.NewDrafts(mem, r.clk, ids), codec)
	return &flowRig{draftRig: r, flows: flows, principal: principal}
}

// asResp returns v as the response type T, or fails the test.
func asResp[T any](t *testing.T, v any) T {
	t.Helper()
	out, ok := v.(T)
	if !ok {
		t.Fatalf("response = %T, want %T", v, out)
	}
	return out
}

// validDoc is a flow document that publishes: a webhook start and its end.
func validDoc() string {
	return string(triggertest.Build(triggertest.DocSpec{Name: "Welcome", Starts: []triggertest.Node{{ID: "t", Block: triggertest.WebhookBlock("welcome")}}}))
}

// saveValid stores a publishable draft and returns its ETag.
func (r *flowRig) saveValid(t *testing.T) string {
	t.Helper()
	resp, err := r.replace(t, `"0"`, validDoc())
	if err != nil {
		t.Fatal(err)
	}
	return *asResp[apigen.ReplaceDraft200JSONResponse](t, resp).Headers.ETag
}

func (r *flowRig) flowTag(t *testing.T) string {
	t.Helper()
	resp, err := r.flows.GetFlow(r.ctx, apigen.GetFlowRequestObject{FlowId: r.flowID})
	if err != nil {
		t.Fatal(err)
	}
	return *asResp[apigen.GetFlow200JSONResponse](t, resp).Headers.ETag
}

func (r *flowRig) publish(t *testing.T, draftTag, note string) (apigen.FlowVersion, error) {
	t.Helper()
	body := &apigen.PublishFlowRequest{}
	if note != "" {
		body.Note = &note
	}
	resp, err := r.flows.PublishFlow(r.ctx, apigen.PublishFlowRequestObject{FlowId: r.flowID, Params: apigen.PublishFlowParams{IfMatch: draftTag}, Body: body})
	if err != nil {
		return apigen.FlowVersion{}, err
	}
	return asResp[apigen.PublishFlow200JSONResponse](t, resp).Body, nil
}

func (r *flowRig) activate(t *testing.T, tag string) (apigen.Flow, error) {
	t.Helper()
	resp, err := r.flows.ActivateFlow(r.ctx, apigen.ActivateFlowRequestObject{FlowId: r.flowID, Params: apigen.ActivateFlowParams{IfMatch: tag}})
	if err != nil {
		return apigen.Flow{}, err
	}
	return asResp[apigen.ActivateFlow200JSONResponse](t, resp).Body, nil
}

func (r *flowRig) actions() []string {
	var out []string
	entries := r.grants.Changes()
	for i := range entries {
		out = append(out, entries[i].Action)
	}
	return out
}

func TestGetFlowReturnsTheFlowWithTheRevisionOfItsRowAsETag(t *testing.T) {
	r := newFlowRig(t)
	resp, err := r.flows.GetFlow(r.ctx, apigen.GetFlowRequestObject{FlowId: r.flowID})
	if err != nil {
		t.Fatal(err)
	}
	ok := asResp[apigen.GetFlow200JSONResponse](t, resp)
	if ok.Body.Name != "Welcome" || ok.Body.ActivationState != apigen.FlowActivationStateINACTIVE || ok.Body.ActiveVersion != nil || *ok.Headers.ETag != `"0"` {
		t.Fatalf("flow = %+v, ETag %s; want the inactive flow at \"0\" with no active version", ok.Body, *ok.Headers.ETag)
	}
	if _, err := r.flows.GetFlow(r.ctx, apigen.GetFlowRequestObject{FlowId: apigen.FlowId(id.NewSequence().New())}); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("unknown flow = %v, want ErrNotFound", err)
	}
}

func TestUpdateFlowRenamesAndMovesTheETagNotTheDraftRevision(t *testing.T) {
	r := newFlowRig(t)
	name := "Welcome v2"
	resp, err := r.flows.UpdateFlow(r.ctx, apigen.UpdateFlowRequestObject{FlowId: r.flowID, Params: apigen.UpdateFlowParams{IfMatch: `"0"`}, Body: &apigen.UpdateFlowRequest{Name: &name}})
	if err != nil {
		t.Fatal(err)
	}
	ok := asResp[apigen.UpdateFlow200JSONResponse](t, resp)
	if ok.Body.Name != name || ok.Body.DraftRevision != 0 || *ok.Headers.ETag != `"1"` {
		t.Fatalf("renamed = %+v, ETag %s; want the new name, draft revision 0 and ETag \"1\"", ok.Body, *ok.Headers.ETag)
	}
	if got := r.actions(); !slices.Equal(got, []string{"flow.updated"}) {
		t.Fatalf("audit = %v, want one flow.updated in the transaction of the rename", got)
	}
}

func TestUpdateFlowRefusesAStaleOrUnreadableTagAndABadName(t *testing.T) {
	r := newFlowRig(t)
	name := "Other"
	update := func(tag string, n *string) error {
		_, err := r.flows.UpdateFlow(r.ctx, apigen.UpdateFlowRequestObject{FlowId: r.flowID, Params: apigen.UpdateFlowParams{IfMatch: tag}, Body: &apigen.UpdateFlowRequest{Name: n}})
		return err
	}
	for _, tag := range []string{`"9"`, `W/"0"`, `*`, `junk`} {
		err := update(tag, &name)
		if w := answer(t, err); w.Code != http.StatusPreconditionFailed {
			t.Errorf("If-Match %s: status %d (%v), want 412", tag, w.Code, err)
		}
	}
	empty := ""
	long := strings.Repeat("x", 201)
	for _, bad := range []*string{&empty, &long} {
		err := update(`"0"`, bad)
		var issues *documentIssuesError
		if !errors.As(err, &issues) || issues.fields[0].Pointer != "/name" {
			t.Errorf("name %q: err = %v, want a field error at /name", *bad, err)
		}
	}
	if err := update(`"0"`, nil); err != nil {
		t.Errorf("an empty patch at the current revision = %v, want a no-op", err)
	}
	if err := update(`"3"`, nil); !errors.Is(err, conditional.ErrMismatch) {
		t.Errorf("an empty patch at a stale revision = %v, want a mismatch", err)
	}
	if len(r.actions()) != 0 {
		t.Errorf("audit = %v after refused renames, want none", r.actions())
	}
}

func TestUpdateFlowToANameInUseIsAConflict(t *testing.T) {
	r := newFlowRig(t)
	proj, _ := r.createProject("Other", "")
	if _, err := r.h.CreateFlow(r.ctx, apigen.CreateFlowRequestObject{ProjectId: *proj.Body.Id, Body: &apigen.CreateFlowRequest{Name: "Taken"}}); err != nil {
		t.Fatal(err)
	}
	same := "Welcome"
	first, _ := r.flows.GetFlow(r.ctx, apigen.GetFlowRequestObject{FlowId: r.flowID})
	pid := asResp[apigen.GetFlow200JSONResponse](t, first).Body.ProjectId
	if _, err := r.h.CreateFlow(r.ctx, apigen.CreateFlowRequestObject{ProjectId: pid, Body: &apigen.CreateFlowRequest{Name: "Second"}}); err != nil {
		t.Fatal(err)
	}
	taken := "Second"
	_, err := r.flows.UpdateFlow(r.ctx, apigen.UpdateFlowRequestObject{FlowId: r.flowID, Params: apigen.UpdateFlowParams{IfMatch: `"0"`}, Body: &apigen.UpdateFlowRequest{Name: &taken}})
	if w := answer(t, err); w.Code != http.StatusConflict {
		t.Fatalf("rename to a name in use = %d (%v), want 409", w.Code, err)
	}
	if _, err := r.flows.UpdateFlow(r.ctx, apigen.UpdateFlowRequestObject{FlowId: r.flowID, Params: apigen.UpdateFlowParams{IfMatch: `"0"`}, Body: &apigen.UpdateFlowRequest{Name: &same}}); err != nil {
		t.Fatalf("rename to its own name = %v", err)
	}
}

func TestPublishFlowCreatesTheVersionWithItsNoteAndPublisher(t *testing.T) {
	r := newFlowRig(t)
	tag := r.saveValid(t)
	version, err := r.publish(t, tag, "Retry the CRM step")
	if err != nil {
		t.Fatal(err)
	}
	if version.Version != 1 || version.Note == nil || *version.Note != "Retry the CRM step" || version.PublishedBy == nil ||
		*version.PublishedBy != types.UUID(r.principal.ID) || version.Document != nil || version.RolledBackFromVersion != nil {
		t.Fatalf("version = %+v, want version 1 with the note and the publisher and no document", version)
	}
	got, _ := r.flows.GetFlow(r.ctx, apigen.GetFlowRequestObject{FlowId: r.flowID})
	body := asResp[apigen.GetFlow200JSONResponse](t, got)
	if body.Body.ActiveVersion == nil || *body.Body.ActiveVersion != 1 || body.Body.ActivationState != apigen.FlowActivationStateINACTIVE {
		t.Fatalf("flow after publishing = %+v, want active version 1 and still inactive", body.Body)
	}
	if got := r.actions(); !slices.Equal(got, []string{"flow.published"}) {
		t.Fatalf("audit = %v, want flow.published", got)
	}
}

func TestPublishFlowWithAStaleDraftTagIs412WithTheCurrentDraftRevision(t *testing.T) {
	r := newFlowRig(t)
	r.saveValid(t)
	for _, tag := range []string{`"0"`, `junk`} {
		_, err := r.publish(t, tag, "")
		var stale *staleDraftError
		if !errors.As(err, &stale) || stale.revision != 1 {
			t.Fatalf("If-Match %s: err = %v, want a stale draft at revision 1", tag, err)
		}
	}
	if len(r.actions()) != 0 {
		t.Errorf("audit = %v after refused publications, want none", r.actions())
	}
}

func TestPublishFlowRefusesADraftWithErrorsAsFieldErrors(t *testing.T) {
	r := newFlowRig(t)
	broken := `{"format":"orcon/flow@1","id":"01930000-0000-7000-8000-00000000000a","name":"Welcome","settings":{},"variables":[],
"nodes":[{"id":"a","name":"A","element":"task","type":"orcon.set@1","parameters":{}}],"edges":[],"editor":{}}`
	resp, err := r.replace(t, `"0"`, broken)
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.publish(t, *asResp[apigen.ReplaceDraft200JSONResponse](t, resp).Headers.ETag, "")
	var issues *documentIssuesError
	if !errors.As(err, &issues) || len(issues.fields) == 0 {
		t.Fatalf("err = %v, want field errors for the findings of the draft", err)
	}
	if w := answer(t, err); w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	if len(r.actions()) != 0 {
		t.Errorf("audit = %v after a refused publication, want none", r.actions())
	}
}

func TestPublishFlowNoteHasAtMost500Characters(t *testing.T) {
	r := newFlowRig(t)
	tag := r.saveValid(t)
	_, err := r.publish(t, tag, strings.Repeat("n", 501))
	var issues *documentIssuesError
	if !errors.As(err, &issues) || issues.fields[0].Pointer != "/note" {
		t.Fatalf("err = %v, want a field error at /note", err)
	}
	if _, err := r.publish(t, tag, strings.Repeat("n", 500)); err != nil {
		t.Fatalf("a note of 500 characters = %v, want it accepted", err)
	}
}

func TestActivateAndDeactivateTakeTheRevisionOfTheFlowRow(t *testing.T) {
	r := newFlowRig(t)
	if _, err := r.activate(t, r.flowTag(t)); !errors.Is(err, trigger.ErrNotPublished) {
		t.Fatalf("activate before publishing = %v, want ErrNotPublished", err)
	} else if w := answer(t, err); w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", w.Code)
	}
	if _, err := r.publish(t, r.saveValid(t), ""); err != nil {
		t.Fatal(err)
	}
	_, err := r.activate(t, `"0"`)
	if w := answer(t, err); w.Code != http.StatusPreconditionFailed {
		t.Fatalf("activate at a stale revision = %d (%v), want 412", w.Code, err)
	}
	active, err := r.activate(t, r.flowTag(t))
	if err != nil || active.ActivationState != apigen.FlowActivationStateACTIVE || active.ActiveVersion == nil || *active.ActiveVersion != 1 {
		t.Fatalf("activated = %+v, %v; want ACTIVE on version 1", active, err)
	}
	resp, err := r.flows.DeactivateFlow(r.ctx, apigen.DeactivateFlowRequestObject{FlowId: r.flowID, Params: apigen.DeactivateFlowParams{IfMatch: `"0"`}})
	if w := answer(t, err); resp != nil || w.Code != http.StatusPreconditionFailed {
		t.Fatalf("deactivate at a stale revision = %d (%v), want 412", w.Code, err)
	}
	resp, err = r.flows.DeactivateFlow(r.ctx, apigen.DeactivateFlowRequestObject{FlowId: r.flowID, Params: apigen.DeactivateFlowParams{IfMatch: r.flowTag(t)}})
	if err != nil || asResp[apigen.DeactivateFlow200JSONResponse](t, resp).Body.ActivationState != apigen.FlowActivationStateINACTIVE {
		t.Fatalf("deactivated = %#v, %v; want INACTIVE", resp, err)
	}
	if got := r.actions(); !slices.Equal(got, []string{"flow.published", "flow.activated", "flow.deactivated"}) {
		t.Fatalf("audit = %v, want published, activated, deactivated in order", got)
	}
}

func TestRollbackFlowRepublishesAnEarlierVersionWithItsOrigin(t *testing.T) {
	r := newFlowRig(t)
	tag := r.saveValid(t)
	for range 2 {
		if _, err := r.publish(t, tag, ""); err != nil {
			t.Fatal(err)
		}
	}
	note := "back to the first"
	rollback := func(ifMatch string, version int) (apigen.RollbackFlowResponseObject, error) {
		return r.flows.RollbackFlow(r.ctx, apigen.RollbackFlowRequestObject{
			FlowId: r.flowID, Params: apigen.RollbackFlowParams{IfMatch: ifMatch}, Body: &apigen.RollbackFlowRequest{Version: version, Note: &note},
		})
	}
	if _, err := rollback(`"0"`, 1); answer(t, err).Code != http.StatusPreconditionFailed {
		t.Fatalf("rollback at a stale revision = %v, want 412", err)
	}
	if _, err := rollback(r.flowTag(t), 9); answer(t, err).Code != http.StatusNotFound {
		t.Fatalf("rollback to a version that does not exist = %v, want 404", err)
	}
	resp, err := rollback(r.flowTag(t), 1)
	if err != nil {
		t.Fatal(err)
	}
	v := asResp[apigen.RollbackFlow200JSONResponse](t, resp).Body
	if v.Version != 3 || v.RolledBackFromVersion == nil || *v.RolledBackFromVersion != 1 || *v.Note != note {
		t.Fatalf("rollback version = %+v, want version 3 from version 1 with its note", v)
	}
	if got := r.actions(); !slices.Equal(got, []string{"flow.published", "flow.published", "flow.rolled_back"}) {
		t.Fatalf("audit = %v", got)
	}
}

func TestListFlowVersionsPagesNewestFirstAndReadsOneWithItsDocument(t *testing.T) {
	r := newFlowRig(t)
	tag := r.saveValid(t)
	for range 3 {
		if _, err := r.publish(t, tag, ""); err != nil {
			t.Fatal(err)
		}
	}
	list := func(sortBy *apigen.ListFlowVersionsParamsSort, token *string, limit int) apigen.FlowVersionList {
		t.Helper()
		resp, err := r.flows.ListFlowVersions(r.ctx, apigen.ListFlowVersionsRequestObject{FlowId: r.flowID, Params: apigen.ListFlowVersionsParams{Limit: &limit, PageToken: token, Sort: sortBy}})
		if err != nil {
			t.Fatal(err)
		}
		return asResp[apigen.ListFlowVersions200JSONResponse](t, resp).Body
	}
	numbers := func(l apigen.FlowVersionList) []int {
		var out []int
		for _, v := range l.Items {
			if v.Document != nil {
				t.Errorf("version %d of a listing carries its document", v.Version)
			}
			out = append(out, v.Version)
		}
		return out
	}
	first := list(nil, nil, 2)
	if !slices.Equal(numbers(first), []int{3, 2}) || first.NextPageToken == nil {
		t.Fatalf("first page = %v token %v, want 3, 2 and a token", numbers(first), first.NextPageToken)
	}
	if second := list(nil, first.NextPageToken, 2); !slices.Equal(numbers(second), []int{1}) || second.NextPageToken != nil {
		t.Fatalf("second page = %v, want 1 and no token", numbers(second))
	}
	asc := apigen.ListFlowVersionsParamsSort("version")
	if got := numbers(list(&asc, nil, 10)); !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("ascending = %v", got)
	}
	resp, err := r.flows.GetFlowVersion(r.ctx, apigen.GetFlowVersionRequestObject{FlowId: r.flowID, Version: 2})
	one := asResp[apigen.GetFlowVersion200JSONResponse](t, resp).Body
	if err != nil || one.Version != 2 || one.Document == nil || len(one.Document.Nodes) == 0 {
		t.Fatalf("version 2 = %+v, %v; want its document", one, err)
	}
	if _, err := r.flows.GetFlowVersion(r.ctx, apigen.GetFlowVersionRequestObject{FlowId: r.flowID, Version: 9}); answer(t, err).Code != http.StatusNotFound {
		t.Fatalf("unknown version = %v, want 404", err)
	}
	other := apigen.FlowId(id.NewSequence().New())
	if _, err := r.flows.ListFlowVersions(r.ctx, apigen.ListFlowVersionsRequestObject{FlowId: other}); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("versions of an unknown flow = %v, want ErrNotFound", err)
	}
}

func TestValidateDraftReportsTheIssuesAndTheDraftRevision(t *testing.T) {
	r := newFlowRig(t)
	validate := func() apigen.ValidationResult {
		t.Helper()
		resp, err := r.flows.ValidateDraft(r.ctx, apigen.ValidateDraftRequestObject{FlowId: r.flowID})
		if err != nil {
			t.Fatal(err)
		}
		return asResp[apigen.ValidateDraft200JSONResponse](t, resp).Body
	}
	r.saveValid(t)
	if got := validate(); !got.Valid || got.DraftRevision != 1 || len(got.Issues) != 0 {
		t.Fatalf("valid draft = %+v", got)
	}
	broken := `{"format":"orcon/flow@1","id":"01930000-0000-7000-8000-00000000000a","name":"Welcome","settings":{},"variables":[],
"nodes":[{"id":"a","name":"A","element":"task","type":"orcon.set@1","parameters":{}}],"edges":[],"editor":{}}`
	if _, err := r.replace(t, `"1"`, broken); err != nil {
		t.Fatal(err)
	}
	if got := validate(); got.Valid || got.DraftRevision != 2 || len(got.Issues) == 0 || got.Issues[0].Severity != apigen.ValidationIssueSeverityERROR {
		t.Fatalf("broken draft = %+v, want invalid with errors at revision 2", got)
	}
	if len(r.actions()) != 0 {
		t.Errorf("validation wrote audit entries %v", r.actions())
	}
}

func TestDeleteFlowTakesTheRevisionOfTheRowAndHidesTheFlow(t *testing.T) {
	r := newFlowRig(t)
	del := func(tag string) error {
		_, err := r.flows.DeleteFlow(r.ctx, apigen.DeleteFlowRequestObject{FlowId: r.flowID, Params: apigen.DeleteFlowParams{IfMatch: tag}})
		return err
	}
	if err := del(`"4"`); answer(t, err).Code != http.StatusPreconditionFailed {
		t.Fatalf("delete at a stale revision = %v, want 412", err)
	}
	if err := del(`"0"`); err != nil {
		t.Fatal(err)
	}
	if _, err := r.flows.GetFlow(r.ctx, apigen.GetFlowRequestObject{FlowId: r.flowID}); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("read after delete = %v, want ErrNotFound", err)
	}
	if err := del(`"0"`); !errors.Is(err, identity.ErrNotFound) {
		t.Fatalf("delete twice = %v, want ErrNotFound", err)
	}
	if got := r.actions(); !slices.Equal(got, []string{"flow.deleted"}) {
		t.Fatalf("audit = %v, want one flow.deleted", got)
	}
}

func TestLifecycleOperationsAnswer503WithoutATriggerService(t *testing.T) {
	r := newFlowRig(t)
	r.flows.lifecycle = nil
	_, err := r.activate(t, `"0"`)
	if w := answer(t, err); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("activate without a service = %d (%v), want 503", w.Code, err)
	}
	if _, err := r.flows.ValidateDraft(r.ctx, apigen.ValidateDraftRequestObject{FlowId: r.flowID}); !errors.Is(err, errUnavailable) {
		t.Fatalf("validate without a service = %v, want unavailable", err)
	}
	if _, err := r.flows.GetFlow(r.ctx, apigen.GetFlowRequestObject{FlowId: r.flowID}); err != nil {
		t.Fatalf("reading still works: %v", err)
	}
}

func TestUpdateDraftAppliesThePatchAtTheCurrentRevision(t *testing.T) {
	r := newFlowRig(t)
	tag := r.saveValid(t)
	patch := apigen.JsonPatch{
		{Op: apigen.Test, Path: "/name", Value: "Welcome"},
		{Op: apigen.Replace, Path: "/name", Value: "Welcome v2"},
	}
	resp, err := r.drafts.UpdateDraft(r.ctx, apigen.UpdateDraftRequestObject{FlowId: r.flowID, Params: apigen.UpdateDraftParams{IfMatch: tag}, Body: &patch})
	if err != nil {
		t.Fatal(err)
	}
	ok := asResp[apigen.UpdateDraft200JSONResponse](t, resp)
	if ok.Body.Revision != 2 || *ok.Headers.ETag != `"2"` || ok.Body.Document.Name == nil || *ok.Body.Document.Name != "Welcome v2" {
		t.Fatalf("patched = revision %d ETag %s document %+v; want revision 2 and the new name", ok.Body.Revision, *ok.Headers.ETag, ok.Body.Document)
	}
}

func TestUpdateDraftWithAStaleTagOrAFailingOperationChangesNothing(t *testing.T) {
	r := newFlowRig(t)
	tag := r.saveValid(t)
	update := func(ifMatch string, patch apigen.JsonPatch) error {
		_, err := r.drafts.UpdateDraft(r.ctx, apigen.UpdateDraftRequestObject{FlowId: r.flowID, Params: apigen.UpdateDraftParams{IfMatch: ifMatch}, Body: &patch})
		return err
	}
	rename := apigen.JsonPatch{{Op: apigen.Replace, Path: "/name", Value: "Other"}}
	var stale *staleDraftError
	if err := update(`"0"`, rename); !errors.As(err, &stale) || stale.revision != 1 {
		t.Fatalf("stale tag = %v, want a stale draft at revision 1", err)
	}
	failing := apigen.JsonPatch{
		{Op: apigen.Replace, Path: "/name", Value: "Other"},
		{Op: apigen.Test, Path: "/name", Value: "Not this"},
	}
	var issues *documentIssuesError
	if err := update(tag, failing); !errors.As(err, &issues) || issues.fields[0].Pointer != "/1" {
		t.Fatalf("failing test operation = %v, want a field error at /1", err)
	}
	breaking := apigen.JsonPatch{{Op: apigen.Remove, Path: "/nodes"}}
	if err := update(tag, breaking); !errors.As(err, &issues) || !strings.HasPrefix(issues.fields[0].Pointer, "/document") {
		t.Fatalf("a patch that breaks the document = %v, want field errors under /document", err)
	}
	got, _ := r.drafts.GetDraft(r.ctx, apigen.GetDraftRequestObject{FlowId: r.flowID})
	if read := asResp[apigen.GetDraft200JSONResponse](t, got); read.Body.Revision != 1 || len(read.Body.Document.Nodes) == 0 {
		t.Fatalf("the draft after refused patches = %+v, want it untouched at revision 1", read.Body)
	}
}
