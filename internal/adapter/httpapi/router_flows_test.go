// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/idempotency"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/config"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
	"github.com/wirvii/orcon/internal/trigger"
)

// flowServer serves the draft and flow operations the router tests use over the real handlers and
// refuses the others, as the composition root does.
type flowServer struct {
	apigen.StrictServerInterface
	drafts *Drafts
	flows  *Flows
}

func (s flowServer) GetDraft(ctx context.Context, r apigen.GetDraftRequestObject) (apigen.GetDraftResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.drafts.GetDraft(ctx, r)
}

func (s flowServer) ReplaceDraft(ctx context.Context, r apigen.ReplaceDraftRequestObject) (apigen.ReplaceDraftResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.drafts.ReplaceDraft(ctx, r)
}

func (s flowServer) UpdateDraft(ctx context.Context, r apigen.UpdateDraftRequestObject) (apigen.UpdateDraftResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.drafts.UpdateDraft(ctx, r)
}

func (s flowServer) PublishFlow(ctx context.Context, r apigen.PublishFlowRequestObject) (apigen.PublishFlowResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.flows.PublishFlow(ctx, r)
}

// flowRouter is the real router over the handlers of a flowRig, with one flow in it.
type flowRouter struct {
	*flowRig
	router *Router
}

func newFlowRouter(t *testing.T) *flowRouter {
	t.Helper()
	return newFlowRouterWith(t, func(src trigger.FlowSource) trigger.FlowSource { return src })
}

func newFlowRouterWith(t *testing.T, wrap func(trigger.FlowSource) trigger.FlowSource) *flowRouter {
	t.Helper()
	r := newFlowRigWith(t, wrap)
	principal := r.principal
	principal.OrgID = r.org
	principal.CredentialType = identity.CredentialPersonalKey
	principal.CredentialID = "key"
	rt, err := New(&Deps{
		Role:          config.RoleAPI,
		Clock:         r.clk,
		IDs:           id.NewSequence(),
		Authenticator: fakeAuthn{"token": principal},
		Authorizer:    authzFunc(func(authz.Request) authz.Decision { return authz.DecisionAllow }),
		Idempotency:   idempotency.NewMemoryStore(r.clk),
		Server:        flowServer{StrictServerInterface: Unavailable{}, drafts: r.drafts, flows: r.flows},
	})
	if err != nil {
		t.Fatalf("build the router: %v", err)
	}
	return &flowRouter{flowRig: r, router: rt}
}

// send sends one request over HTTP, with the headers given (an empty value is sent as an empty header).
func (f *flowRouter) send(method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	f.t.Helper()
	req := httptest.NewRequestWithContext(f.t.Context(), method, path, strings.NewReader(body))
	req.RemoteAddr = "192.0.2.10:4000"
	req.Header.Set("Authorization", "Bearer token")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header[http.CanonicalHeaderKey(k)] = []string{v}
	}
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec
}

func (f *flowRouter) draftPath() string {
	return "/api/v1/flows/" + id.UUID(f.flowID).String() + "/draft"
}

// bigDocument is a document with the given number of nodes and edges: a ring of nodes, edges between neighbors.
func bigDocument(nodes, edges int) string {
	var b strings.Builder
	b.WriteString(`{"format":"orcon/flow@1","id":"01930000-0000-7000-8000-00000000000a","name":"Big","settings":{},"variables":[],"nodes":[`)
	for i := range nodes {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"id":"n%d","name":"N","element":"task","type":"orcon.set@1","parameters":{}}`, i)
	}
	b.WriteString(`],"edges":[`)
	for i := range edges {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"id":"e%d","from":{"node":"n%d","port":"main"},"to":{"node":"n%d"},"kind":"SEQUENCE"}`, i, i%nodes, (i+1)%nodes)
	}
	b.WriteString(`],"editor":{}}`)
	return b.String()
}

func TestEmptyIfMatchIs428OnEveryOperationThatRequiresIt(t *testing.T) {
	f := newFlowRouter(t)
	spec := f.router.Spec()
	// Bodies that satisfy the schema of the operations whose body is required, so the request
	// reaches the parameter binding instead of failing on its body.
	bodies := map[string]string{
		"replaceCredentialSecret": `{"secretFields":{"token":"x"}}`,
		"replaceDraft":            `{"document":` + oneNodeDoc + `}`,
		"updateDraft":             `[{"op":"remove","path":"/nodes/0"}]`,
		"rollbackFlow":            `{"version":1}`,
	}
	checked := 0
	for i := range spec.Operations {
		op := &spec.Operations[i]
		if !op.IfMatchRequired {
			continue
		}
		path := op.Path
		for _, name := range op.PathParams {
			path = strings.ReplaceAll(path, "{"+name+"}", id.UUID(f.flowID).String())
		}
		body := ""
		if op.HasBody {
			body = bodies[op.ID]
			if body == "" {
				body = `{}`
			}
		}
		headers := map[string]string{"If-Match": "", "Content-Type": "application/json"}
		for media := range op.BodyMedia {
			headers["Content-Type"] = media
			break
		}
		rec := f.send(op.Method, "/api/v1"+path, body, headers)
		if rec.Code != http.StatusPreconditionRequired {
			t.Errorf("%s with an empty If-Match: %d %s, want 428", op.ID, rec.Code, rec.Body.String())
		}
		checked++
	}
	// The contract had ten such operations when the rule was written; a new one joins the check by itself.
	if checked < 10 {
		t.Errorf("operations that require If-Match = %d, want at least 10", checked)
	}
}

func TestSaveCapsOfNodesAndEdgesOverHTTP(t *testing.T) {
	f := newFlowRouter(t)
	put := func(tag, doc string) *httptest.ResponseRecorder {
		return f.send(http.MethodPut, f.draftPath(), `{"document":`+doc+`}`, map[string]string{"If-Match": tag})
	}
	if rec := put(`"0"`, bigDocument(flow.MaxDraftNodes, flow.MaxDraftEdges)); rec.Code != http.StatusOK {
		t.Fatalf("PUT at the caps: %d %s", rec.Code, rec.Body.String())
	}
	if rec := put(`"1"`, bigDocument(flow.MaxDraftNodes+1, 0)); rec.Code != http.StatusBadRequest {
		t.Errorf("PUT with one node over the cap: %d, want 400", rec.Code)
	}
	if rec := put(`"1"`, bigDocument(flow.MaxDraftNodes, flow.MaxDraftEdges+1)); rec.Code != http.StatusBadRequest {
		t.Errorf("PUT with one edge over the cap: %d, want 400", rec.Code)
	}
	patch := func(tag, body string) *httptest.ResponseRecorder {
		return f.send(http.MethodPatch, f.draftPath(), body, map[string]string{"If-Match": tag, "Content-Type": "application/json-patch+json"})
	}
	node := `{"op":"add","path":"/nodes/-","value":{"id":"extra","name":"N","element":"task","type":"orcon.set@1","parameters":{}}}`
	if rec := patch(`"1"`, `[`+node+`]`); rec.Code != http.StatusBadRequest {
		t.Errorf("PATCH with one node over the cap: %d %s, want 400", rec.Code, rec.Body.String())
	}
	edge := `{"op":"add","path":"/edges/-","value":{"id":"extra","from":{"node":"n0","port":"main"},"to":{"node":"n1"},"kind":"SEQUENCE"}}`
	if rec := patch(`"1"`, `[`+edge+`]`); rec.Code != http.StatusBadRequest {
		t.Errorf("PATCH with one edge over the cap: %d %s, want 400", rec.Code, rec.Body.String())
	}
	// Under the caps a patch still goes through: remove one edge, then add it back.
	rm := `{"op":"remove","path":"/edges/0"}`
	if rec := patch(`"1"`, `[`+rm+`,`+edge+`]`); rec.Code != http.StatusOK {
		t.Errorf("PATCH that stays at the caps: %d %s, want 200", rec.Code, rec.Body.String())
	}
}

// barrierSource holds every publication at its read of the project until all of them have read the
// flow, so they overlap: a publication that reads after another has landed is a sequential retry,
// which the contract allows, not a concurrent one.
type barrierSource struct {
	trigger.FlowSource
	arrived sync.WaitGroup
	release chan struct{}
}

func (b *barrierSource) GetProject(ctx context.Context, org orgscope.Org, project identity.ProjectID) (identity.ProjectRecord, error) {
	b.arrived.Done()
	select {
	case <-b.release:
	case <-time.After(10 * time.Second):
	}
	return b.FlowSource.GetProject(ctx, org, project)
}

func TestConcurrentPublishWithTheSameIfMatchMakesOneVersion(t *testing.T) {
	const racers = 8
	barrier := &barrierSource{release: make(chan struct{})}
	barrier.arrived.Add(racers)
	f := newFlowRouterWith(t, func(src trigger.FlowSource) trigger.FlowSource {
		barrier.FlowSource = src
		return barrier
	})
	tag := f.saveValid(t)
	go func() {
		barrier.arrived.Wait()
		close(barrier.release)
	}()
	var wg sync.WaitGroup
	codes := make([]int, racers)
	start := make(chan struct{})
	for i := range racers {
		wg.Go(func() {
			<-start
			rec := f.send(http.MethodPost, "/api/v1/flows/"+id.UUID(f.flowID).String()+"/publish", `{}`, map[string]string{"If-Match": tag})
			codes[i] = rec.Code
		})
	}
	close(start)
	wg.Wait()
	count := map[int]int{}
	for _, c := range codes {
		count[c]++
	}
	if count[http.StatusOK] != 1 || count[http.StatusPreconditionFailed] != racers-1 {
		t.Fatalf("answers = %v, want one 200 and %d 412", count, racers-1)
	}
	versions, err := f.flows.ListFlowVersions(f.ctx, apigen.ListFlowVersionsRequestObject{FlowId: f.flowID})
	if err != nil {
		t.Fatal(err)
	}
	list := asResp[apigen.ListFlowVersions200JSONResponse](t, versions)
	if n := len(list.Body.Items); n != 1 {
		t.Errorf("versions = %d, want 1", n)
	}
}
