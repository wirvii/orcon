// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/oapi-codegen/runtime/types"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/adapter/httpapi/page"
	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/execution"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/internal/platform/orgscope"
	"github.com/wirvii/orcon/internal/platform/orgscope/issue"
)

// fakeExecutionStore is the execution store of a test: it answers like the real one, from the
// organization of the scope only.
type fakeExecutionStore struct {
	org        identity.OrgID
	executions []store.Execution
	nodes      []store.NodeExecution
	outputs    []store.NodeOutputRecord
	events     []store.Event
}

func (f *fakeExecutionStore) mine(org orgscope.Org) bool { return f.org == identity.OrgIDOf(org) }

func (f *fakeExecutionStore) GetExecution(_ context.Context, org orgscope.Org, exec store.ExecutionID) (store.Execution, error) {
	if f.mine(org) {
		for i := range f.executions {
			if f.executions[i].ID == exec {
				return f.executions[i], nil
			}
		}
	}
	return store.Execution{}, flow.ErrNotFound
}

func (f *fakeExecutionStore) ListExecutions(_ context.Context, org orgscope.Org, filter store.ExecutionFilter, after store.Cursor, limit int) ([]store.Execution, error) {
	var out []store.Execution
	if !f.mine(org) {
		return out, nil
	}
	rows := slices.Clone(f.executions)
	slices.SortFunc(rows, func(a, b store.Execution) int { return b.CreatedAt.Compare(a.CreatedAt) })
	for i := range rows {
		e := rows[i]
		if e.ProjectID != filter.ProjectID {
			continue
		}
		if !after.IsZero() && !e.CreatedAt.Before(after.CreatedAt) {
			continue
		}
		out = append(out, e)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func (f *fakeExecutionStore) ListNodeExecutions(_ context.Context, org orgscope.Org, exec store.ExecutionID, after store.Cursor, limit int) ([]store.NodeExecution, error) {
	var out []store.NodeExecution
	if !f.mine(org) {
		return out, nil
	}
	for i := range f.nodes {
		n := &f.nodes[i]
		if n.ExecutionID == exec && (after.IsZero() || n.CreatedAt.After(after.CreatedAt)) && len(out) < limit {
			out = append(out, *n)
		}
	}
	return out, nil
}

func (f *fakeExecutionStore) ListNodeOutputs(_ context.Context, org orgscope.Org, exec store.ExecutionID, after store.Cursor, limit int) ([]store.NodeOutputRecord, error) {
	var out []store.NodeOutputRecord
	if !f.mine(org) {
		return out, nil
	}
	for _, o := range f.outputs {
		if (after.IsZero() || o.CreatedAt.After(after.CreatedAt)) && len(out) < limit {
			out = append(out, o)
		}
	}
	return out, nil
}

func (f *fakeExecutionStore) ListExecutionEvents(_ context.Context, org orgscope.Org, exec store.ExecutionID, after int64, limit int) ([]store.Event, error) {
	var out []store.Event
	if !f.mine(org) {
		return out, nil
	}
	for i := range f.events {
		e := &f.events[i]
		if e.ExecutionID == exec && e.SequenceNumber > after && len(out) < limit {
			out = append(out, *e)
		}
	}
	return out, nil
}

type fakeRevisions struct{ doc []byte }

func (f fakeRevisions) GetVersion(context.Context, orgscope.Org, flow.FlowVersionID) (flow.VersionRecord, error) {
	return flow.VersionRecord{Number: 3, Document: f.doc}, nil
}

func (f fakeRevisions) GetSnapshot(context.Context, orgscope.Org, flow.FlowSnapshotID) (flow.SnapshotRecord, error) {
	return flow.SnapshotRecord{Document: f.doc}, nil
}

// fakeDocs reads the marks the test wants, whatever the bytes.
type fakeDocs struct {
	marks map[string][]string
	err   error
}

func (f fakeDocs) Read([]byte) (*flow.Document, error) {
	if f.err != nil {
		return nil, f.err
	}
	var doc flow.Document
	for node, paths := range f.marks {
		doc.Nodes = append(doc.Nodes, flow.Node{ID: flow.NodeID(node), SensitivePaths: paths})
	}
	return &doc, nil
}

// readDecider allows or denies the read-data permission of the test.
type readDecider struct{ allowData bool }

func (d readDecider) Authorize(context.Context, authz.Request) (authz.Decision, orgscope.Org, error) {
	if d.allowData {
		return authz.DecisionAllow, orgscope.Org{}, nil
	}
	return authz.DecisionForbidden, orgscope.Org{}, nil
}

type auditRecorder struct {
	entries []audit.Entry
	fail    error
}

func (a *auditRecorder) AppendOrgEntry(_ context.Context, _ orgscope.Org, e audit.Entry) error {
	if a.fail != nil {
		return a.fail
	}
	a.entries = append(a.entries, e)
	return nil
}

type executionsRig struct {
	t      *testing.T
	h      *Executions
	st     *fakeExecutionStore
	log    *auditRecorder
	ctx    context.Context //nolint:containedctx // the rig carries the request context of every call it makes
	other  context.Context //nolint:containedctx // the context of a request authorized in another organization
	org    identity.OrgID
	flowID flow.FlowID
	proj   identity.ProjectID
	exec   store.ExecutionID
	ids    id.Generator
}

var executionsBase = time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)

func newExecutionsRig(t *testing.T, docs fakeDocs, allowData bool) *executionsRig {
	t.Helper()
	ids := id.NewSequence()
	org := identity.OrgID(ids.New())
	otherOrg := identity.OrgID(ids.New())
	codec, err := page.NewCodec(make([]byte, page.MinKeyLength))
	if err != nil {
		t.Fatal(err)
	}
	principal := identity.Principal{ID: identity.PrincipalID(ids.New()), Kind: identity.PrincipalUser, UserID: identity.UserID(ids.New()), CredentialType: identity.CredentialSession}
	scope := func(o identity.OrgID) context.Context {
		s, err := issue.New(id.UUID(o))
		if err != nil {
			t.Fatal(err)
		}
		return authz.WithOrgScope(middleware.WithPrincipal(t.Context(), principal), s)
	}
	r := &executionsRig{
		t: t, st: &fakeExecutionStore{org: org}, log: &auditRecorder{}, ctx: scope(org), other: scope(otherOrg),
		org: org, flowID: flow.FlowID(ids.New()), proj: identity.ProjectID(ids.New()), ids: ids,
	}
	r.exec = store.ExecutionID(ids.New())
	r.h = NewExecutions(r.st, fakeRevisions{doc: []byte("{}")}, docs, readDecider{allowData: allowData}, r.log, clock.NewFake(executionsBase), ids, codec)
	return r
}

func (r *executionsRig) addExecution(n int, status string) store.Execution {
	r.t.Helper()
	e := store.Execution{
		ID: store.ExecutionID(r.ids.New()), OrgID: r.org, ProjectID: r.proj, FlowID: r.flowID,
		FlowVersionID: flow.FlowVersionID{1}, Status: status, Mode: "MANUAL",
		CreatedAt: executionsBase.Add(time.Duration(n) * time.Minute), StartedAt: executionsBase,
	}
	r.st.executions = append(r.st.executions, e)
	return e
}

func problemKind(err error) string {
	switch {
	case errors.Is(err, identity.ErrNotFound):
		return "not-found"
	case errors.Is(err, page.ErrInvalidToken):
		return "invalid-page-token"
	default:
		return fmt.Sprint(err)
	}
}

// TestListExecutionsFiltersSortsAndPages: the list of a project is newest first, filters by flow and
// status, and pages with a token that only works for the same query.
func TestListExecutionsFiltersSortsAndPages(t *testing.T) {
	r := newExecutionsRig(t, fakeDocs{}, true)
	for i := range 5 {
		status := "SUCCEEDED"
		if i%2 == 0 {
			status = "FAILED"
		}
		r.addExecution(i, status)
	}
	limit := 2
	req := apigen.ListExecutionsRequestObject{ProjectId: types.UUID(r.proj), Params: apigen.ListExecutionsParams{Limit: &limit}}
	var seen []time.Time
	for range 3 {
		resp, err := r.h.ListExecutions(r.ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		body := answerOf[apigen.ListExecutions200JSONResponse](t, resp).Body
		for i := range body.Items {
			it := &body.Items[i]
			if it.DataHidden || it.FlowVersion != 3 || it.Mode != "MANUAL" {
				t.Fatalf("item = %+v", it)
			}
			seen = append(seen, it.CreatedAt)
		}
		if body.NextPageToken == nil {
			break
		}
		req.Params.PageToken = body.NextPageToken
	}
	if len(seen) != 5 || !slices.IsSortedFunc(seen, func(a, b time.Time) int { return b.Compare(a) }) {
		t.Fatalf("seen = %v, want five executions newest first", seen)
	}

	status := apigen.ListExecutionsParamsStatus("FAILED")
	resp, err := r.h.ListExecutions(r.ctx, apigen.ListExecutionsRequestObject{ProjectId: types.UUID(r.proj), Params: apigen.ListExecutionsParams{Status: &status}})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(answerOf[apigen.ListExecutions200JSONResponse](t, resp).Body.Items); n != 3 {
		t.Fatalf("failed executions = %d, want 3", n)
	}

	// A token of another query is refused.
	req.Params.Status = &status
	if _, err := r.h.ListExecutions(r.ctx, req); problemKind(err) != "invalid-page-token" {
		t.Fatalf("token of another query: %v", err)
	}
	// Ascending order is allowed; an unknown field is not.
	asc := apigen.ListExecutionsParamsSort("createdAt")
	resp, err = r.h.ListExecutions(r.ctx, apigen.ListExecutionsRequestObject{ProjectId: types.UUID(r.proj), Params: apigen.ListExecutionsParams{Sort: &asc}})
	if err != nil {
		t.Fatal(err)
	}
	if first := answerOf[apigen.ListExecutions200JSONResponse](t, resp).Body.Items[0]; !first.CreatedAt.Equal(executionsBase) {
		t.Fatalf("oldest first: %v", first.CreatedAt)
	}
}

// TestExecutionReadsHideAnotherOrganization: an execution of another organization answers as one that
// does not exist, for every operation of the read side, and nothing is audited.
func TestExecutionReadsHideAnotherOrganization(t *testing.T) {
	r := newExecutionsRig(t, fakeDocs{}, true)
	e := r.addExecution(1, "SUCCEEDED")
	eid := types.UUID(e.ID)
	if _, err := r.h.GetExecution(r.other, apigen.GetExecutionRequestObject{ExecutionId: eid}); problemKind(err) != "not-found" {
		t.Fatalf("get: %v", err)
	}
	if _, err := r.h.ListExecutionNodes(r.other, apigen.ListExecutionNodesRequestObject{ExecutionId: eid}); problemKind(err) != "not-found" {
		t.Fatalf("nodes: %v", err)
	}
	if _, err := r.h.ListExecutionEvents(r.other, apigen.ListExecutionEventsRequestObject{ExecutionId: eid}); problemKind(err) != "not-found" {
		t.Fatalf("events: %v", err)
	}
	if _, err := r.h.ListExecutionData(r.other, apigen.ListExecutionDataRequestObject{ExecutionId: eid}); problemKind(err) != "not-found" {
		t.Fatalf("data: %v", err)
	}
	resp, err := r.h.ListExecutions(r.other, apigen.ListExecutionsRequestObject{ProjectId: types.UUID(r.proj)})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(answerOf[apigen.ListExecutions200JSONResponse](t, resp).Body.Items); n != 0 {
		t.Fatalf("another organization lists %d executions", n)
	}
	if len(r.log.entries) != 0 {
		t.Fatalf("refused reads were audited: %d", len(r.log.entries))
	}
	// A page token of one organization means nothing to another.
	limit := 1
	r.addExecution(2, "SUCCEEDED")
	first, err := r.h.ListExecutions(r.ctx, apigen.ListExecutionsRequestObject{ProjectId: types.UUID(r.proj), Params: apigen.ListExecutionsParams{Limit: &limit}})
	if err != nil {
		t.Fatal(err)
	}
	token := answerOf[apigen.ListExecutions200JSONResponse](t, first).Body.NextPageToken
	if token == nil {
		t.Fatal("no next page")
	}
	_, err = r.h.ListExecutions(r.other, apigen.ListExecutionsRequestObject{ProjectId: types.UUID(r.proj), Params: apigen.ListExecutionsParams{PageToken: token}})
	if problemKind(err) != "invalid-page-token" {
		t.Fatalf("token across organizations: %v", err)
	}
}

// TestGetExecutionHidesDataWithoutThePermission: dataHidden is true for a caller without
// execution:read-data, false for one with it.
func TestGetExecutionHidesDataWithoutThePermission(t *testing.T) {
	for _, allow := range []bool{false, true} {
		r := newExecutionsRig(t, fakeDocs{}, allow)
		e := r.addExecution(1, "SUCCEEDED")
		resp, err := r.h.GetExecution(r.ctx, apigen.GetExecutionRequestObject{ExecutionId: types.UUID(e.ID)})
		if err != nil {
			t.Fatal(err)
		}
		body := answerOf[apigen.GetExecution200JSONResponse](t, resp).Body
		if body.DataHidden == allow || body.Output != nil {
			t.Fatalf("allow=%v: dataHidden=%v output=%v", allow, body.DataHidden, body.Output)
		}
	}
}

// TestListExecutionNodesAndEventsPage: nodes and events page with tokens and show only identifiers.
func TestListExecutionNodesAndEventsPage(t *testing.T) {
	r := newExecutionsRig(t, fakeDocs{}, true)
	e := r.addExecution(1, "SUCCEEDED")
	for i := range 3 {
		r.st.nodes = append(r.st.nodes, store.NodeExecution{
			ID: store.NodeExecutionID{byte(i + 1)}, ExecutionID: e.ID, NodeID: fmt.Sprintf("n%d", i), Status: "SUCCEEDED",
			AttemptCount: 2, CreatedAt: executionsBase.Add(time.Duration(i) * time.Second),
		})
		r.st.events = append(r.st.events, store.Event{
			ExecutionID: e.ID, SequenceNumber: int64(i + 1), Type: "node.succeeded", OccurredAt: executionsBase,
			Reference: []byte(fmt.Sprintf(`{"nodeId":"n%d","edgeId":""}`, i)), Data: []byte(`{"detail":"secret value"}`),
		})
	}
	limit := 2
	var nodeIDs []string
	req := apigen.ListExecutionNodesRequestObject{ExecutionId: types.UUID(e.ID), Params: apigen.ListExecutionNodesParams{Limit: &limit}}
	for range 3 {
		resp, err := r.h.ListExecutionNodes(r.ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		body := answerOf[apigen.ListExecutionNodes200JSONResponse](t, resp).Body
		for _, n := range body.Items {
			if n.AttemptCount != 2 {
				t.Fatalf("attempts = %d", n.AttemptCount)
			}
			nodeIDs = append(nodeIDs, n.NodeId)
		}
		if body.NextPageToken == nil {
			break
		}
		req.Params.PageToken = body.NextPageToken
	}
	if strings.Join(nodeIDs, ",") != "n0,n1,n2" {
		t.Fatalf("nodes = %v", nodeIDs)
	}

	var seqs []int
	ereq := apigen.ListExecutionEventsRequestObject{ExecutionId: types.UUID(e.ID), Params: apigen.ListExecutionEventsParams{Limit: &limit}}
	for range 3 {
		resp, err := r.h.ListExecutionEvents(r.ctx, ereq)
		if err != nil {
			t.Fatal(err)
		}
		body := answerOf[apigen.ListExecutionEvents200JSONResponse](t, resp).Body
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "secret value") {
			t.Fatalf("an event carried data: %s", raw)
		}
		for _, ev := range body.Items {
			seqs = append(seqs, ev.SequenceNumber)
			if ev.NodeId == nil || ev.EdgeId != nil {
				t.Fatalf("event = %+v", ev)
			}
		}
		if body.NextPageToken == nil {
			break
		}
		ereq.Params.PageToken = body.NextPageToken
	}
	if fmt.Sprint(seqs) != "[1 2 3]" {
		t.Fatalf("sequence numbers = %v", seqs)
	}
	// A token of the node list does not open the event list.
	ereq.Params.PageToken = req.Params.PageToken
	if _, err := r.h.ListExecutionEvents(r.ctx, ereq); problemKind(err) != "invalid-page-token" {
		t.Fatalf("token of another list: %v", err)
	}
}

func batchDoc(t *testing.T, items ...string) []byte {
	t.Helper()
	var batch []execution.Item
	for _, raw := range items {
		v, err := jsonvalue.Parse([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		batch = append(batch, execution.Item{JSON: v})
	}
	doc, err := execution.EncodeBatch(execution.NewBatch(batch...))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

// TestListExecutionDataMasksAuditsAndFailsClosed: marked paths leave as the mask, the first page is
// audited without data, a repeat page is not audited again, a failing audit stops the data, and
// marks that cannot be read stop it too.
func TestListExecutionDataMasksAuditsAndFailsClosed(t *testing.T) {
	r := newExecutionsRig(t, fakeDocs{marks: map[string][]string{"send": {"/token", "/rows/*/ssn"}}}, true)
	e := r.addExecution(1, "SUCCEEDED")
	doc := batchDoc(t, `{"token":"abc-123","name":"Ana","rows":[{"ssn":"1","ok":true},{"ssn":"2"}]}`, `{"token":"zzz"}`)
	for i, node := range []string{"send", "other"} {
		r.st.outputs = append(r.st.outputs, store.NodeOutputRecord{
			ID: store.PayloadID{byte(i + 1)}, NodeExecutionID: store.NodeExecutionID{byte(i + 1)}, NodeID: node, Port: "main",
			ItemCount: 2, Document: doc, CreatedAt: executionsBase.Add(time.Duration(i) * time.Second),
		})
	}
	limit := 1
	req := apigen.ListExecutionDataRequestObject{ExecutionId: types.UUID(e.ID), Params: apigen.ListExecutionDataParams{Limit: &limit}}
	resp, err := r.h.ListExecutionData(r.ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	first := answerOf[apigen.ListExecutionData200JSONResponse](t, resp).Body
	raw, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, leaked := range []string{"abc-123", "zzz", `"ssn":"1"`, `"ssn":"2"`} {
		if strings.Contains(text, leaked) {
			t.Fatalf("%q leaked: %s", leaked, text)
		}
	}
	if !strings.Contains(text, `"name":"Ana"`) || strings.Count(text, execution.Redacted) != 4 || first.Items[0].ItemCount != 2 {
		t.Fatalf("first page = %s", text)
	}
	if len(r.log.entries) != 1 || r.log.entries[0].Action != "execution.data_viewed" || strings.Contains(string(r.log.entries[0].Body), "abc-123") {
		t.Fatalf("audit = %+v", r.log.entries)
	}
	if first.NextPageToken == nil {
		t.Fatal("no second page")
	}

	// The second page belongs to the same read: no new entry. The node with no marks shows its data.
	req.Params.PageToken = first.NextPageToken
	resp, err = r.h.ListExecutionData(r.ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	second := answerOf[apigen.ListExecutionData200JSONResponse](t, resp).Body
	if len(second.Items) != 1 || second.Items[0].NodeId != "other" || second.Items[0].DataItems[0]["token"] != "abc-123" {
		t.Fatalf("second page = %+v", second)
	}
	if len(r.log.entries) != 1 {
		t.Fatalf("a later page was audited: %d entries", len(r.log.entries))
	}

	// When the audit entry cannot be stored the data is not served.
	r.log.fail = errors.New("audit store down")
	req.Params.PageToken = nil
	if resp, err := r.h.ListExecutionData(r.ctx, req); err == nil {
		t.Fatalf("data served without an audit entry: %+v", resp)
	}
	r.log.fail = nil

	// When the marks cannot be read the data is not served either.
	broken := newExecutionsRig(t, fakeDocs{err: errors.New("unreadable document")}, true)
	be := broken.addExecution(1, "SUCCEEDED")
	broken.st.outputs = r.st.outputs
	if resp, err := broken.h.ListExecutionData(broken.ctx, apigen.ListExecutionDataRequestObject{ExecutionId: types.UUID(be.ID)}); err == nil {
		t.Fatalf("data served without its marks: %+v", resp)
	}
}

// answerOf returns the response as the 200 type the test expects.
func answerOf[T any](t *testing.T, resp any) T {
	t.Helper()
	out, ok := resp.(T)
	if !ok {
		t.Fatalf("response = %T, want %T", resp, out)
	}
	return out
}
