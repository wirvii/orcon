// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/httpapi"
	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/adapter/httpapi/page"
	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// allowAll is the authorizer of the read-side test: every question is allowed.
type allowAll struct{}

func (allowAll) Authorize(context.Context, authz.Request) (authz.Decision, orgscope.Org, error) {
	return authz.DecisionAllow, orgscope.Org{}, nil
}

// recordingAudit keeps the entries the read side appends.
type recordingAudit struct {
	mu      sync.Mutex
	entries []audit.Entry
}

func (r *recordingAudit) AppendOrgEntry(_ context.Context, _ orgscope.Org, e audit.Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, e)
	return nil
}

// checkExecutionReads reads the finished execution through the handlers of the read side over
// the real store: the node runs, the events and the data, with the paths the flow marks as
// sensitive masked, and nothing visible from another organization.
func checkExecutionReads(t *testing.T, env *storetest.EngineEnv, st engineStore, exec store.ExecutionID) {
	t.Helper()
	codec, err := page.NewCodec(make([]byte, page.MinKeyLength))
	if err != nil {
		t.Fatal(err)
	}
	log := &recordingAudit{}
	reader, ok := env.Store.(httpapi.ExecutionReader)
	if !ok {
		t.Fatalf("the %s store does not implement the execution reads", env.Kind)
	}
	packages, ok := env.Store.(httpapi.ConnectorPackages)
	if !ok {
		t.Fatalf("the %s store does not implement the connector package reads", env.Kind)
	}
	h := httpapi.NewExecutions(reader, st, flowReader{}, packages, allowAll{}, log, env.Clock, env.IDs, codec)
	scope := storetest.ScopeOf(env.Org)
	principal := identity.Principal{ID: identity.PrincipalID(env.IDs.New()), Kind: identity.PrincipalUser, UserID: identity.UserID(env.IDs.New()), CredentialType: identity.CredentialSession}
	ctx := authz.WithOrgScope(middleware.WithPrincipal(t.Context(), principal), scope)
	eid := apigen.ExecutionId(exec)

	got, err := h.GetExecution(ctx, apigen.GetExecutionRequestObject{ExecutionId: eid})
	if err != nil {
		t.Fatalf("get execution: %v", err)
	}
	body := answer[apigen.GetExecution200JSONResponse](t, got).Body
	if body.Status != apigen.ExecutionStatusSUCCEEDED || body.FlowVersion != 1 || body.DataHidden || body.FinishedAt == nil || body.StartedAt == nil {
		t.Fatalf("execution = %+v", body)
	}

	list, err := h.ListExecutions(ctx, apigen.ListExecutionsRequestObject{ProjectId: apigen.ProjectId(env.Project)})
	if err != nil {
		t.Fatalf("list executions: %v", err)
	}
	if items := answer[apigen.ListExecutions200JSONResponse](t, list).Body.Items; len(items) == 0 {
		t.Fatalf("the project lists no execution")
	}

	nodes, err := h.ListExecutionNodes(ctx, apigen.ListExecutionNodesRequestObject{ExecutionId: eid})
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	seen := map[string]int{}
	for _, n := range answer[apigen.ListExecutionNodes200JSONResponse](t, nodes).Body.Items {
		seen[n.NodeId] = n.AttemptCount
	}
	if _, ok := seen["check"]; !ok {
		t.Fatalf("nodes = %v, want the check node among them", seen)
	}

	limit := 1
	first, err := h.ListExecutionEvents(ctx, apigen.ListExecutionEventsRequestObject{ExecutionId: eid, Params: apigen.ListExecutionEventsParams{Limit: &limit}})
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	firstPage := answer[apigen.ListExecutionEvents200JSONResponse](t, first).Body
	if len(firstPage.Items) != 1 || firstPage.Items[0].SequenceNumber != 1 || firstPage.NextPageToken == nil {
		t.Fatalf("first page of events = %+v", firstPage)
	}
	second, err := h.ListExecutionEvents(ctx, apigen.ListExecutionEventsRequestObject{ExecutionId: eid, Params: apigen.ListExecutionEventsParams{Limit: &limit, PageToken: firstPage.NextPageToken}})
	if err != nil {
		t.Fatalf("second page of events: %v", err)
	}
	if items := answer[apigen.ListExecutionEvents200JSONResponse](t, second).Body.Items; len(items) != 1 || items[0].SequenceNumber != 2 {
		t.Fatalf("second page of events = %+v", items)
	}

	data, err := h.ListExecutionData(ctx, apigen.ListExecutionDataRequestObject{ExecutionId: eid})
	if err != nil {
		t.Fatalf("list data: %v", err)
	}
	dataBody := answer[apigen.ListExecutionData200JSONResponse](t, data).Body
	masked := false
	for _, entry := range dataBody.Items {
		raw, err := json.Marshal(entry.DataItems)
		if err != nil {
			t.Fatal(err)
		}
		switch entry.NodeId {
		case "check":
			// The node marks /amount, so the value never leaves.
			masked = strings.Contains(string(raw), `"amount":"[REDACTED]"`) && !strings.Contains(string(raw), "42")
		case "hook":
			// A mark covers the field of the marked node only (ADR-0017 section 3).
			if !strings.Contains(string(raw), `"amount":42`) {
				t.Fatalf("the unmarked node lost its data: %s", raw)
			}
		}
	}
	if !masked {
		t.Fatalf("the data of the check node is not masked: %+v", dataBody)
	}
	if len(log.entries) != 1 || log.entries[0].Action != "execution.data_viewed" {
		t.Fatalf("audit entries = %+v, want one execution.data_viewed", log.entries)
	}

	// Another organization sees nothing of the execution, whatever the operation.
	other := authz.WithOrgScope(middleware.WithPrincipal(t.Context(), principal), storetest.ScopeOf(identity.OrgID(env.IDs.New())))
	if _, err := h.GetExecution(other, apigen.GetExecutionRequestObject{ExecutionId: eid}); err == nil {
		t.Fatalf("an execution of another organization was served")
	}
	if _, err := h.ListExecutionData(other, apigen.ListExecutionDataRequestObject{ExecutionId: eid}); err == nil {
		t.Fatalf("the data of another organization was served")
	}
	if len(log.entries) != 1 {
		t.Fatalf("a refused read was audited: %d entries", len(log.entries))
	}
}

// answer returns the response as the 200 type the test expects.
func answer[T any](t *testing.T, resp any) T {
	t.Helper()
	out, ok := resp.(T)
	if !ok {
		t.Fatalf("response = %T, want %T", resp, out)
	}
	return out
}
