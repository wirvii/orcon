// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"time"

	"github.com/oapi-codegen/runtime/types"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/adapter/httpapi/page"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/execution/store"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// execSortFields are the sort fields of listOrgExecutions (the allowlist of the contract, without the minus).
var execSortFields = []string{"createdAt"}

// OrgExecutions serves listOrgExecutions (SPEC-019 R-UX-BE-03): the executions of the projects the caller
// reads, filtered and newest first. The list holds metadata only.
type OrgExecutions struct {
	access   ProjectAccessResolver
	projects identity.ProjectStore
	reader   store.ProjectExecutionReader
	codec    *page.Codec
}

// NewOrgExecutions returns the handler over the project access resolver, the project listing, the execution
// read side and the page-token codec.
func NewOrgExecutions(access ProjectAccessResolver, projects identity.ProjectStore, reader store.ProjectExecutionReader, codec *page.Codec) *OrgExecutions {
	return &OrgExecutions{access: access, projects: projects, reader: reader, codec: codec}
}

// ListOrgExecutions lists the executions of the organization the caller can read, one page at a time.
// A project filter on a project the caller cannot read gives an empty page.
func (e *OrgExecutions) ListOrgExecutions(ctx context.Context, req apigen.ListOrgExecutionsRequestObject) (apigen.ListOrgExecutionsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, err := scopeOf(ctx)
	if err != nil {
		return nil, err
	}
	limit, err := pageLimit(req.Params.Limit)
	if err != nil {
		return nil, err
	}
	terms, err := page.ParseSort(derefString(execSortParam(req.Params.Sort)), execSortFields, "-createdAt")
	if err != nil {
		return nil, err
	}
	filter := executionFilterOf(req.Params)
	binding := page.Binding(append([]string{"sort=" + page.String(terms)}, filter.binding()...)...)
	cursor, err := cursorFor(e.codec, derefString(req.Params.PageToken), org.String(), binding, terms)
	if err != nil {
		return nil, err
	}
	principal, ok := middleware.PrincipalFrom(ctx)
	if !ok || principal.UserID.IsZero() {
		return nil, identity.ErrUnauthenticated
	}
	access, err := e.access.ProjectAccess(ctx, principal.UserID, org)
	if err != nil {
		return nil, err
	}
	readable, err := authz.ReadableProjects(ctx, e.projects, org, access, page.MaxLimit, maxListRows)
	if err != nil {
		return nil, err
	}
	var rows []store.Execution
	var items []listItem
	for i := range readable {
		pid := readable[i].ID
		if filter.project != "" && id.UUID(pid).String() != filter.project {
			continue
		}
		batch, err := e.readProject(ctx, org, pid, filter.query)
		if err != nil {
			return nil, err
		}
		for j := range batch {
			items = append(items, listItem{
				keys: map[string]string{"createdAt": sortKeyTime(batch[j].CreatedAt)},
				id:   id.UUID(batch[j].ID).String(), idx: len(rows),
			})
			rows = append(rows, batch[j])
		}
	}
	window, more, last := listWindow(items, terms, limit, cursor)

	body := apigen.ExecutionList{Items: []apigen.Execution{}}
	for _, it := range window {
		body.Items = append(body.Items, executionItemBody(&rows[it.idx]))
	}
	if more {
		token, err := tokenFor(e.codec, org.String(), binding, last)
		if err != nil {
			return nil, err
		}
		body.NextPageToken = &token
	}
	return apigen.ListOrgExecutions200JSONResponse{Body: body}, nil
}

// readProject reads the executions of one project that match the query, newest first, page by page,
// at most maxListRows of them.
func (e *OrgExecutions) readProject(ctx context.Context, org orgscope.Org, project identity.ProjectID, query store.ExecutionQuery) ([]store.Execution, error) {
	var out []store.Execution
	var after store.Cursor
	for len(out) < maxListRows {
		batch, err := e.reader.ListProjectExecutions(ctx, org, project, query, after, page.MaxLimit)
		if err != nil {
			return nil, err
		}
		out = append(out, batch...)
		if len(batch) < page.MaxLimit {
			break
		}
		last := batch[len(batch)-1]
		after = store.Cursor{CreatedAt: last.CreatedAt, ID: id.UUID(last.ID)}
	}
	return out, nil
}

// executionListFilter is the filter of listOrgExecutions: the project it narrows to (text, or empty) and
// the query the store runs for each project.
type executionListFilter struct {
	project string
	query   store.ExecutionQuery
	// The text of each filter, in the form the page token binds to.
	projectText, flowText, idText, statusText, uncertainText, afterText, beforeText string
}

// binding returns the query parts a page token is bound to, so a token never serves another query.
func (f executionListFilter) binding() []string {
	return []string{
		"project=" + f.projectText, "flow=" + f.flowText, "id=" + f.idText, "status=" + f.statusText,
		"uncertain=" + f.uncertainText, "after=" + f.afterText, "before=" + f.beforeText,
	}
}

// executionFilterOf reads the filter of a listOrgExecutions request.
func executionFilterOf(p apigen.ListOrgExecutionsParams) executionListFilter {
	var f executionListFilter
	if p.ProjectId != nil {
		f.project = p.ProjectId.String()
		f.projectText = f.project
	}
	if p.FlowId != nil {
		f.query.FlowID = flow.FlowID(*p.FlowId)
		f.flowText = p.FlowId.String()
	}
	if p.Id != nil {
		f.query.ID = store.ExecutionID(*p.Id)
		f.idText = p.Id.String()
	}
	if p.Status != nil {
		f.query.Status = string(*p.Status)
		f.statusText = f.query.Status
	}
	if p.Uncertain != nil && *p.Uncertain {
		f.query.UncertainOnly = true
		f.uncertainText = "true"
	}
	if p.CreatedAfter != nil {
		f.query.CreatedAfter = *p.CreatedAfter
		f.afterText = p.CreatedAfter.UTC().Format(time.RFC3339Nano)
	}
	if p.CreatedBefore != nil {
		f.query.CreatedBefore = *p.CreatedBefore
		f.beforeText = p.CreatedBefore.UTC().Format(time.RFC3339Nano)
	}
	return f
}

// execSortParam returns the sort expression of listOrgExecutions as text, or nil when it is absent.
func execSortParam(s *apigen.ListOrgExecutionsParamsSort) *string {
	if s == nil {
		return nil
	}
	v := string(*s)
	return &v
}

// executionItemBody is an execution of a list as the contract states it. The list holds metadata only,
// so the output summary is never included; a run of a snapshot has no flow version number.
func executionItemBody(e *store.Execution) apigen.Execution {
	eid := types.UUID(e.ID)
	hasUncertain := e.HasUncertainOperation
	body := apigen.Execution{
		Id: &eid, FlowId: types.UUID(e.FlowID), ProjectId: types.UUID(e.ProjectID),
		Mode: apigen.ExecutionMode(e.Mode), Status: apigen.ExecutionStatus(e.Status),
		CreatedAt: e.CreatedAt, HasUncertainOperation: &hasUncertain,
	}
	if e.FlowVersionNumber > 0 {
		version := int(e.FlowVersionNumber)
		body.FlowVersion = &version
	}
	if !e.StartedAt.IsZero() {
		started := e.StartedAt
		body.StartedAt = &started
	}
	if !e.FinishedAt.IsZero() {
		finished := e.FinishedAt
		body.FinishedAt = &finished
	}
	return body
}
