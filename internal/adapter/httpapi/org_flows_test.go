// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"testing"
	"time"

	"github.com/oapi-codegen/runtime/types"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
)

// orgFlowsOf returns the body of a 200 listing of the flows of an organization.
func orgFlowsOf(t *testing.T, resp apigen.ListOrgFlowsResponseObject) apigen.FlowList {
	t.Helper()
	out, ok := resp.(apigen.ListOrgFlows200JSONResponse)
	if !ok {
		t.Fatalf("listOrgFlows answered %T, want 200", resp)
	}
	return out.Body
}

// TestListOrgFlowsListsReadableFlowsOfEveryProject: the organization list holds the flows of every
// project the caller reads, newest first, each with the count of its executions of the last day;
// a project filter narrows it to one project.
func TestListOrgFlowsListsReadableFlowsOfEveryProject(t *testing.T) {
	r := newProjectsRig(t)
	ops, err := r.createProject("Ops", "")
	if err != nil {
		t.Fatal(err)
	}
	sales, err := r.createProject("Sales", "")
	if err != nil {
		t.Fatal(err)
	}
	opsID, salesID := *ops.Body.Id, *sales.Body.Id
	for _, c := range []struct {
		project types.UUID
		name    string
	}{{opsID, "Ops first"}, {salesID, "Sales first"}} {
		r.clk.Advance(time.Second)
		if _, err := r.h.CreateFlow(r.ctx, apigen.CreateFlowRequestObject{ProjectId: c.project, Body: &apigen.CreateFlowRequest{Name: c.name}}); err != nil {
			t.Fatalf("create %s: %v", c.name, err)
		}
	}

	all, err := r.h.ListOrgFlows(r.ctx, apigen.ListOrgFlowsRequestObject{OrgId: types.UUID(r.org)})
	if err != nil {
		t.Fatal(err)
	}
	items := orgFlowsOf(t, all).Items
	if len(items) != 2 || items[0].Name != "Sales first" || items[1].Name != "Ops first" {
		t.Fatalf("organization flows = %+v, want the two flows newest first", items)
	}
	for _, it := range items {
		if it.ExecutionCountLast24h == nil || *it.ExecutionCountLast24h != 0 {
			t.Errorf("flow %s: executionCountLast24h = %v, want 0 for a flow that never ran", it.Name, it.ExecutionCountLast24h)
		}
		if it.LastExecution != nil {
			t.Errorf("flow %s: lastExecution = %+v, want none", it.Name, it.LastExecution)
		}
	}

	only, err := r.h.ListOrgFlows(r.ctx, apigen.ListOrgFlowsRequestObject{
		OrgId: types.UUID(r.org), Params: apigen.ListOrgFlowsParams{ProjectId: &opsID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := orgFlowsOf(t, only).Items; len(got) != 1 || got[0].Name != "Ops first" {
		t.Errorf("flows of project Ops = %+v, want only Ops first", got)
	}
}
