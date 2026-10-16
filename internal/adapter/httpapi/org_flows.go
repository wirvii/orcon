// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"time"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/adapter/httpapi/page"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

// orgFlowSortFields are the sort fields of listOrgFlows (the allowlist of the contract, without the minus).
var orgFlowSortFields = []string{"updatedAt", "lastExecutionAt"}

// activityWindow is how far back the execution count of a flow list reaches (SPEC-019 R-UX-BE-01).
const activityWindow = 24 * time.Hour

// ListOrgFlows lists the flows of every project of the organization that the caller can read, each
// with its latest execution and its executions of the last day (SPEC-019 R-UX-BE-01). The filters
// narrow the list; a flow of a project the caller cannot read is never listed, and a project
// filter on such a project gives an empty page.
func (p *Projects) ListOrgFlows(ctx context.Context, req apigen.ListOrgFlowsRequestObject) (apigen.ListOrgFlowsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, err := scopeOf(ctx)
	if err != nil {
		return nil, err
	}
	limit, err := pageLimit(req.Params.Limit)
	if err != nil {
		return nil, err
	}
	terms, err := page.ParseSort(derefString(orgFlowSortParam(req.Params.Sort)), orgFlowSortFields, "-updatedAt")
	if err != nil {
		return nil, err
	}
	q := derefString(req.Params.Q)
	state := derefString(orgFlowStateParam(req.Params.ActivationState))
	project := ""
	if req.Params.ProjectId != nil {
		project = req.Params.ProjectId.String()
	}
	filter := flow.ListFilter{NamePrefix: q, ActivationState: flow.ActivationState(state)}
	binding := page.Binding("sort="+page.String(terms), "q="+q, "activationState="+state, "project="+project)
	cursor, err := cursorFor(p.codec, derefString(req.Params.PageToken), org.String(), binding, terms)
	if err != nil {
		return nil, err
	}
	principal, ok := middleware.PrincipalFrom(ctx)
	if !ok || principal.UserID.IsZero() {
		return nil, identity.ErrUnauthenticated
	}
	access, err := p.access.ProjectAccess(ctx, principal.UserID, org)
	if err != nil {
		return nil, err
	}
	readable, err := authz.ReadableProjects(ctx, p.store, org, access, page.MaxLimit, maxListRows)
	if err != nil {
		return nil, err
	}
	since := p.clock.Now().Add(-activityWindow)
	var rows []flow.Activity
	var items []listItem
	for i := range readable {
		pid := readable[i].ID
		if project != "" && id.UUID(pid).String() != project {
			continue
		}
		batch, err := p.readActivity(ctx, org, pid, since)
		if err != nil {
			return nil, err
		}
		for j := range batch {
			a := batch[j]
			if !filter.Match(a.Flow) {
				continue
			}
			items = append(items, listItem{
				keys: map[string]string{
					"updatedAt":       sortKeyTime(a.Flow.UpdatedAt),
					"lastExecutionAt": lastExecutionKey(a.LastExecution),
				},
				id: id.UUID(a.Flow.ID).String(), idx: len(rows),
			})
			rows = append(rows, a)
		}
	}
	window, more, last := listWindow(items, terms, limit, cursor)

	body := apigen.FlowList{Items: []apigen.Flow{}}
	for _, it := range window {
		body.Items = append(body.Items, flowActivityBody(&rows[it.idx]))
	}
	if more {
		token, err := tokenFor(p.codec, org.String(), binding, last)
		if err != nil {
			return nil, err
		}
		body.NextPageToken = &token
	}
	return apigen.ListOrgFlows200JSONResponse{Body: body}, nil
}

// lastExecutionKey is the sort key of a flow by its latest execution: the zero-padded instant it was
// accepted, or the empty string for a flow that never ran (so it sorts last in descending order).
func lastExecutionKey(e *flow.LastExecution) string {
	if e == nil {
		return ""
	}
	return sortKeyTime(e.CreatedAt)
}

// orgFlowSortParam returns the sort expression of listOrgFlows as text, or nil when it is absent.
func orgFlowSortParam(s *apigen.ListOrgFlowsParamsSort) *string {
	if s == nil {
		return nil
	}
	v := string(*s)
	return &v
}

// orgFlowStateParam returns the activation state filter of listOrgFlows as text, or nil when absent.
func orgFlowStateParam(s *apigen.ListOrgFlowsParamsActivationState) *string {
	if s == nil {
		return nil
	}
	v := string(*s)
	return &v
}
