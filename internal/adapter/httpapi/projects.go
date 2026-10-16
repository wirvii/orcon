// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oapi-codegen/runtime/types"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/conditional"
	"github.com/wirvii/orcon/internal/adapter/httpapi/idempotency"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/adapter/httpapi/page"
	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// Projects serves the project and flow operations of API2-core (SPEC-005 section 3.4 and 3.5,
// SPEC-016 SH4): listProjects, createProject, listFlows and createFlow. Each one runs after
// authorization, so the organization it works in is the one the authorization decided
// (authz.OrgScopeFrom); a handler never builds a scope from a request value.
type Projects struct {
	access    ProjectAccessResolver
	projects  *identity.Projects
	store     identity.ProjectStore
	flows     *flow.Creator
	flowStore flow.Store
	codec     *page.Codec
	clock     clock.Clock
}

// ProjectAccessResolver says which projects of an organization a user can read (the SE6
// evaluator implements it). It is asked once per request, never once per project.
type ProjectAccessResolver interface {
	ProjectAccess(ctx context.Context, user identity.UserID, org orgscope.Org) (authz.ProjectAccess, error)
}

// NewProjects returns the handlers over their services, the project access resolver and the
// page-token codec.
func NewProjects(access ProjectAccessResolver, projects *identity.Projects, store identity.ProjectStore, flows *flow.Creator, flowStore flow.Store, codec *page.Codec, clk clock.Clock) *Projects {
	return &Projects{access: access, projects: projects, store: store, flows: flows, flowStore: flowStore, codec: codec, clock: clk}
}

// Sort fields of the listings (the allowlists of the contract, without the minus).
var (
	projectSortFields = []string{"name", "updatedAt"}
	flowSortFields    = []string{"name", "updatedAt"}
)

// slugPattern is the form of a project slug the API accepts: lowercase letters, digits and
// single hyphens, so a handle reads well in a URL.
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// maxSlug is the length bound of a project slug (the project table).
const maxSlug = 100

// ListProjects lists the projects of the organization that the caller can read (project:read in
// the project or project:read-all, R-SH-PRJ-01), one page at a time, filtered by name prefix, environment and handle, and sorted by name or update time.
func (p *Projects) ListProjects(ctx context.Context, req apigen.ListProjectsRequestObject) (apigen.ListProjectsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, err := scopeOf(ctx)
	if err != nil {
		return nil, err
	}
	limit, err := pageLimit(req.Params.Limit)
	if err != nil {
		return nil, err
	}
	terms, err := page.ParseSort(derefString(sortParam(req.Params.Sort)), projectSortFields, "name")
	if err != nil {
		return nil, err
	}
	q := derefString(req.Params.Q)
	env := derefString(req.Params.Environment)
	handle := derefString(req.Params.Handle)
	filter := identity.ProjectFilter{NamePrefix: q, Environment: env, Handle: handle}
	binding := page.Binding("sort="+page.String(terms), "q="+q, "environment="+env, "handle="+handle)
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
	rows, err := authz.ReadableProjects(ctx, p.store, org, access, page.MaxLimit, maxListRows)
	if err != nil {
		return nil, err
	}
	var items []listItem
	for i := range rows {
		rec := &rows[i]
		if !filter.Match(*rec) {
			continue
		}
		items = append(items, listItem{
			keys: map[string]string{"name": rec.Name, "updatedAt": sortKeyTime(rec.UpdatedAt)},
			id:   id.UUID(rec.ID).String(), idx: i,
		})
	}
	window, more, last := listWindow(items, terms, limit, cursor)

	body := apigen.ProjectList{Items: []apigen.Project{}}
	for _, it := range window {
		item := projectBody(rows[it.idx])
		n, err := p.flowStore.CountFlows(ctx, org, rows[it.idx].ID)
		if err != nil {
			return nil, err
		}
		count := int(n)
		item.FlowCount = &count
		body.Items = append(body.Items, item)
	}
	if more {
		token, err := tokenFor(p.codec, org.String(), binding, last)
		if err != nil {
			return nil, err
		}
		body.NextPageToken = &token
	}
	return apigen.ListProjects200JSONResponse{Body: body}, nil
}

// CreateProject creates a project in the organization. The handle is the slug, or the slug
// prefixed with the organization slug when the slug is held (SPEC-004 R-ID-11); when that is
// held too the answer is the generic 409 conflict. The creator becomes Project Admin of the
// project in the same transaction.
func (p *Projects) CreateProject(ctx context.Context, req apigen.CreateProjectRequestObject) (apigen.CreateProjectResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, err := scopeOf(ctx)
	if err != nil {
		return nil, err
	}
	principal, ok := middleware.PrincipalFrom(ctx)
	if !ok || principal.UserID.IsZero() {
		return nil, identity.ErrUnauthenticated
	}
	if req.Body == nil {
		return nil, identity.ErrInvalidInput
	}
	var slug string
	if req.Body.Slug != nil {
		slug = *req.Body.Slug
	} else {
		slug = slugOf(req.Body.Name)
	}
	if slug == "" || len(slug) > maxSlug || !slugPattern.MatchString(slug) {
		return nil, fmt.Errorf("%w: slug must be 1 to %d lowercase letters, digits and single hyphens", identity.ErrInvalidInput, maxSlug)
	}
	access, err := p.access.ProjectAccess(ctx, principal.UserID, org)
	if err != nil {
		return nil, err
	}
	// A creator who already reads every project (Owner, Admin, project:read-all) gets no binding;
	// anyone else becomes Project Admin of the project they create (SPEC-016 Q3 = A, R-SH-PRJ-03).
	creator := principal.UserID
	if access.ReadsEveryProject() {
		creator = identity.UserID{}
	}
	// The request cannot create an unprotected project: a new project is always protected
	// (ADR-0059 section 5, SH-PRJ-1); a protected:false in the body is ignored.
	rec, err := p.projects.Create(ctx, org, identity.NewProject{
		Name: req.Body.Name, Slug: slug, Environment: derefString(req.Body.Environment),
		By: actorOf(principal), Creator: creator,
	})
	if err != nil {
		return nil, err
	}
	loc := projectLocation(rec.ID)
	return apigen.CreateProject201JSONResponse{
		Body: projectBody(rec), Headers: apigen.CreateProject201ResponseHeaders{Location: &loc},
	}, nil
}

// ListFlows lists the flows of a project, one page at a time, filtered by name prefix and
// activation state, and sorted by name or update time (newest first by default).
func (p *Projects) ListFlows(ctx context.Context, req apigen.ListFlowsRequestObject) (apigen.ListFlowsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, err := scopeOf(ctx)
	if err != nil {
		return nil, err
	}
	limit, err := pageLimit(req.Params.Limit)
	if err != nil {
		return nil, err
	}
	terms, err := page.ParseSort(derefString(flowSortParam(req.Params.Sort)), flowSortFields, "-updatedAt")
	if err != nil {
		return nil, err
	}
	q := derefString(req.Params.Q)
	state := derefString(flowStateParam(req.Params.ActivationState))
	project := identity.ProjectID(req.ProjectId)
	filter := flow.ListFilter{NamePrefix: q, ActivationState: flow.ActivationState(state)}
	binding := page.Binding("sort="+page.String(terms), "q="+q, "activationState="+state, "project="+project.String())
	cursor, err := cursorFor(p.codec, derefString(req.Params.PageToken), org.String(), binding, terms)
	if err != nil {
		return nil, err
	}

	rows, err := p.readActivity(ctx, org, project, p.clock.Now().Add(-activityWindow))
	if err != nil {
		return nil, err
	}
	var items []listItem
	for i := range rows {
		rec := &rows[i].Flow
		if !filter.Match(*rec) {
			continue
		}
		items = append(items, listItem{
			keys: map[string]string{"name": rec.Name, "updatedAt": sortKeyTime(rec.UpdatedAt)},
			id:   id.UUID(rec.ID).String(), idx: i,
		})
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
	return apigen.ListFlows200JSONResponse{Body: body}, nil
}

// CreateFlow creates an inactive flow with an empty draft in the project (ADR-0080 section 3).
// The creation and its audit entry are one transaction.
func (p *Projects) CreateFlow(ctx context.Context, req apigen.CreateFlowRequestObject) (apigen.CreateFlowResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, err := scopeOf(ctx)
	if err != nil {
		return nil, err
	}
	principal, ok := middleware.PrincipalFrom(ctx)
	if !ok || principal.UserID.IsZero() {
		return nil, identity.ErrUnauthenticated
	}
	if req.Body == nil {
		return nil, identity.ErrInvalidInput
	}
	rec, err := p.flows.Create(ctx, org, flow.CreateInput{
		ProjectID: identity.ProjectID(req.ProjectId), Name: req.Body.Name, By: actorOf(principal),
	})
	if err != nil {
		return nil, err
	}
	etag := flowETag(rec)
	loc := flowLocation(rec.ID)
	return apigen.CreateFlow201JSONResponse{
		Body: flowListBody(rec), Headers: apigen.CreateFlow201ResponseHeaders{ETag: &etag, Location: &loc},
	}, nil
}

// Rebuilds returns the rebuild of the two creations: a repeat that finds an in-progress record
// whose change committed answers the resource that the change created (Amendment 8 section 8-2).
func (p *Projects) Rebuilds() map[string]idempotency.Rebuild {
	return map[string]idempotency.Rebuild{
		"createProject": p.rebuildProject,
		"createFlow":    p.rebuildFlow,
	}
}

func (p *Projects) rebuildProject(ctx context.Context, rec identity.IdempotencyRecord) (identity.IdempotencyResponse, error) {
	org, err := scopeOf(ctx)
	if err != nil {
		return identity.IdempotencyResponse{}, err
	}
	raw, ok := strings.CutPrefix(rec.ResourceRef, "project:")
	if !ok {
		return identity.IdempotencyResponse{}, idempotency.ErrNoRebuild
	}
	pid, err := id.Parse(raw)
	if err != nil {
		return identity.IdempotencyResponse{}, idempotency.ErrNoRebuild
	}
	project, err := p.store.GetProject(ctx, org, identity.ProjectID(pid))
	if errors.Is(err, identity.ErrNotFound) {
		return identity.IdempotencyResponse{}, idempotency.ErrNoRebuild
	}
	if err != nil {
		return identity.IdempotencyResponse{}, err
	}
	loc := projectLocation(project.ID)
	return encodeResponse(201, projectBody(project), map[string]string{"Location": loc}, "")
}

func (p *Projects) rebuildFlow(ctx context.Context, rec identity.IdempotencyRecord) (identity.IdempotencyResponse, error) {
	org, err := scopeOf(ctx)
	if err != nil {
		return identity.IdempotencyResponse{}, err
	}
	raw, ok := strings.CutPrefix(rec.ResourceRef, "flow:")
	if !ok {
		return identity.IdempotencyResponse{}, idempotency.ErrNoRebuild
	}
	fid, err := id.Parse(raw)
	if err != nil {
		return identity.IdempotencyResponse{}, idempotency.ErrNoRebuild
	}
	f, err := p.flowStore.GetFlow(ctx, org, flow.FlowID(fid))
	if errors.Is(err, identity.ErrNotFound) {
		return identity.IdempotencyResponse{}, idempotency.ErrNoRebuild
	}
	if err != nil {
		return identity.IdempotencyResponse{}, err
	}
	return encodeResponse(201, flowListBody(f), map[string]string{"Location": flowLocation(f.ID)}, flowETag(f))
}

// readActivity reads the flows of the project with their latest execution and the executions of
// the day since the instant given, at most maxListRows flows.
func (p *Projects) readActivity(ctx context.Context, org orgscope.Org, project identity.ProjectID, since time.Time) ([]flow.Activity, error) {
	var out []flow.Activity
	var after flow.FlowID
	for len(out) < maxListRows {
		batch, err := p.flowStore.ListFlowActivity(ctx, org, project, since, after, page.MaxLimit)
		if err != nil {
			return nil, err
		}
		out = append(out, batch...)
		if len(batch) < page.MaxLimit {
			break
		}
		after = batch[len(batch)-1].Flow.ID
	}
	return out, nil
}

// scopeOf returns the organization the request was authorized in.
func scopeOf(ctx context.Context) (orgscope.Org, error) {
	org, ok := authz.OrgScopeFrom(ctx)
	if !ok {
		return orgscope.Org{}, identity.ErrUnauthenticated
	}
	return org, nil
}

// actorOf is the audit actor of a user principal.
func actorOf(p identity.Principal) audit.Actor {
	return audit.Actor{PrincipalID: id.UUID(p.ID), Kind: audit.ActorUser}
}

// pageLimit reads the limit parameter (PAGE-1).
func pageLimit(raw *int) (int, error) {
	if raw == nil {
		return page.Limit("")
	}
	return page.Limit(strconv.Itoa(*raw))
}

func sortParam(s *apigen.ListProjectsParamsSort) *string {
	if s == nil {
		return nil
	}
	v := string(*s)
	return &v
}

func flowSortParam(s *apigen.ListFlowsParamsSort) *string {
	if s == nil {
		return nil
	}
	v := string(*s)
	return &v
}

func flowStateParam(s *apigen.ListFlowsParamsActivationState) *string {
	if s == nil {
		return nil
	}
	v := string(*s)
	return &v
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// slugOf derives a slug from a project name: lowercase letters and digits, runs of other
// characters become one hyphen. It is empty when the name has none of them.
func slugOf(name string) string {
	var b strings.Builder
	dash := true
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.TrimSuffix(b.String(), "-")
	if len(out) > maxSlug {
		out = strings.TrimSuffix(out[:maxSlug], "-")
	}
	return out
}

func projectBody(rec identity.ProjectRecord) apigen.Project {
	pid := types.UUID(rec.ID)
	body := apigen.Project{
		Id: &pid, OrgId: types.UUID(rec.OrgID), Name: rec.Name, Slug: rec.Slug, Handle: rec.Handle,
		Protected: rec.IsProtected, CreatedAt: rec.CreatedAt, UpdatedAt: rec.UpdatedAt,
	}
	if rec.Environment != "" {
		env := rec.Environment
		body.Environment = &env
	}
	return body
}

// flowListBody is the flow as a list or a creation shows it. These reads do not compare content
// hashes, which would cost a version read and a parse per row: a flow that was never published has
// changes by definition, and a published one is taken to have them once its draft was saved at all
// (the reads of one flow, [flow.Manager.Get], compare the hashes and are exact).
func flowListBody(rec flow.Record) apigen.Flow {
	rec.HasUnpublishedChanges = rec.ActiveVersionID.IsZero() || rec.DraftRevision > 0
	return flowBody(rec)
}

// flowBody is the flow as the API shows it. A flow has a version number only once it is
// published; the publication operations arrive with the editor, so until then the active
// version is absent.
func flowBody(rec flow.Record) apigen.Flow {
	fid := types.UUID(rec.ID)
	return apigen.Flow{
		Id: &fid, ProjectId: types.UUID(rec.ProjectID), Name: rec.Name,
		ActivationState:       apigen.FlowActivationState(rec.ActivationState),
		DraftRevision:         int(rec.DraftRevision),
		ActiveVersion:         activeVersion(rec),
		HasUnpublishedChanges: rec.HasUnpublishedChanges,
		CreatedAt:             rec.CreatedAt, UpdatedAt: rec.UpdatedAt,
	}
}

// flowActivityBody is a flow of a list with its latest execution and its executions of the last day.
func flowActivityBody(a *flow.Activity) apigen.Flow {
	body := flowListBody(a.Flow)
	count := int(a.ExecutionCountLast24h)
	body.ExecutionCountLast24h = &count
	if a.LastExecution != nil {
		body.LastExecution = lastExecutionBody(*a.LastExecution)
	}
	return body
}

// lastExecutionBody is the latest execution of a flow as the list shows it; the finish instant is
// left out while the execution has not finished.
func lastExecutionBody(e flow.LastExecution) *apigen.FlowLastExecution {
	eid := types.UUID(e.ID)
	body := apigen.FlowLastExecution{
		Id: &eid, Status: apigen.FlowLastExecutionStatus(e.Status),
		HasUncertainOperation: e.HasUncertainOperation, CreatedAt: e.CreatedAt,
	}
	if !e.FinishedAt.IsZero() {
		finished := e.FinishedAt
		body.FinishedAt = &finished
	}
	return &body
}

// flowETag is the entity tag of the flow: the revision of its row, which activation, rename,
// rollback and delete carry in If-Match (Amendment 8 section 8-9).
func flowETag(rec flow.Record) string { return conditional.ETag(rec.Revision) }

// draftETag is the entity tag of the draft: its revision, which a save and a publication carry in
// If-Match (Amendment 8 section 8-9).
func draftETag(rec flow.Record) string { return conditional.ETag(rec.DraftRevision) }

func projectLocation(pid identity.ProjectID) string {
	return "/api/v1/projects/" + id.UUID(pid).String()
}

func flowLocation(fid flow.FlowID) string {
	return "/api/v1/flows/" + id.UUID(fid).String()
}

// encodeResponse is the stored form of a JSON response: the body encodes as the generated code
// encodes it, so a rebuilt answer is the answer the first request gave.
func encodeResponse(status int, body any, headers map[string]string, etag string) (identity.IdempotencyResponse, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return identity.IdempotencyResponse{}, fmt.Errorf("encode the rebuilt response: %w", err)
	}
	return identity.IdempotencyResponse{
		Status: status, ContentType: "application/json", Location: headers["Location"], ETag: etag, Body: buf.Bytes(),
	}, nil
}

// activeVersion is the number of the active version, absent until the first publication.
func activeVersion(rec flow.Record) *int {
	if rec.ActiveVersion < 1 {
		return nil
	}
	n := int(rec.ActiveVersion)
	return &n
}
