// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/oapi-codegen/runtime/types"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/conditional"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/adapter/httpapi/page"
	"github.com/wirvii/orcon/internal/adapter/httpapi/problem"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
	"github.com/wirvii/orcon/internal/trigger"
)

// maxNote is the length bound of a release note, in characters (SPEC-014 A7).
const maxNote = 500

// versionSortFields is the allowlist of the sort of the version listing.
var versionSortFields = []string{"version"}

// FlowLifecycle is what the flow operations need of the lifecycle of a flow: publication,
// rollback, activation, deletion and the validation of a draft (the trigger service implements
// it). Each change is one transaction with its audit entry.
type FlowLifecycle interface {
	Publish(ctx context.Context, cmd trigger.PublishCommand) (trigger.Published, error)
	Rollback(ctx context.Context, cmd trigger.RollbackCommand) (trigger.Published, error)
	ActivateFlow(ctx context.Context, cmd *trigger.ChangeCommand) error
	DeactivateFlow(ctx context.Context, cmd *trigger.ChangeCommand) error
	DeleteFlow(ctx context.Context, cmd *trigger.ChangeCommand) error
	Validate(ctx context.Context, org orgscope.Org, flowID flow.FlowID) (flow.Report, error)
}

// FlowVersions reads the published versions of a flow.
type FlowVersions interface {
	GetVersionByNumber(ctx context.Context, org orgscope.Org, flowID flow.FlowID, number int64) (flow.VersionRecord, error)
	ListVersions(ctx context.Context, org orgscope.Org, flowID flow.FlowID, after int64, limit int) ([]flow.VersionRecord, error)
}

// Flows serves the operations on one flow (SPEC-005 section 3.5, Amendment 8 sections 8 and 9):
// read, rename, delete, publish, activate, deactivate, roll back, the version history and the
// validation of the draft. Each one runs after authorization, so the organization is the one the
// authorization decided (authz.OrgScopeFrom).
type Flows struct {
	manager   *flow.Manager
	lifecycle FlowLifecycle
	versions  FlowVersions
	drafts    *flow.Drafts
	codec     *page.Codec
}

// NewFlows returns the flow handlers over their services. A nil lifecycle leaves the operations that
// change the lifecycle of a flow, or validate it, answering 503 service-unavailable: a process
// without an engine has no trigger service.
func NewFlows(manager *flow.Manager, lifecycle FlowLifecycle, versions FlowVersions, drafts *flow.Drafts, codec *page.Codec) *Flows {
	return &Flows{manager: manager, lifecycle: lifecycle, versions: versions, drafts: drafts, codec: codec}
}

// principalOf returns the user principal of the request.
func principalOf(ctx context.Context) (identity.Principal, error) {
	principal, ok := middleware.PrincipalFrom(ctx)
	if !ok || principal.UserID.IsZero() {
		return identity.Principal{}, identity.ErrUnauthenticated
	}
	return principal, nil
}

// revisionOf reads If-Match as a revision. A tag this API did not issue names no current revision.
func revisionOf(ifMatch string) (int64, error) {
	if strings.TrimSpace(ifMatch) == "" {
		return 0, conditional.ErrRequired
	}
	rev, err := conditional.Parse(ifMatch)
	if err != nil {
		return 0, conditional.ErrMismatch
	}
	return rev, nil
}

// GetFlow returns the flow with the revision of its row as the ETag.
func (f *Flows) GetFlow(ctx context.Context, req apigen.GetFlowRequestObject) (apigen.GetFlowResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, err := scopeOf(ctx)
	if err != nil {
		return nil, err
	}
	rec, err := f.manager.Get(ctx, org, flow.FlowID(req.FlowId))
	if err != nil {
		return nil, err
	}
	etag := flowETag(rec)
	return apigen.GetFlow200JSONResponse{Body: flowBody(rec), Headers: apigen.GetFlow200ResponseHeaders{ETag: &etag}}, nil
}

// UpdateFlow changes the name of the flow when If-Match names the current revision of the flow.
func (f *Flows) UpdateFlow(ctx context.Context, req apigen.UpdateFlowRequestObject) (apigen.UpdateFlowResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, err := scopeOf(ctx)
	if err != nil {
		return nil, err
	}
	principal, err := principalOf(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, identity.ErrInvalidInput
	}
	expected, err := revisionOf(req.Params.IfMatch)
	if err != nil {
		return nil, err
	}
	var rec flow.Record
	if req.Body.Name == nil {
		// Nothing to change: the patch still has to be based on the current representation.
		if rec, err = f.manager.Get(ctx, org, flow.FlowID(req.FlowId)); err != nil {
			return nil, err
		}
		if rec.Revision != expected {
			return nil, conditional.ErrMismatch
		}
	} else if rec, err = f.manager.Rename(ctx, org, flow.RenameInput{
		FlowID: flow.FlowID(req.FlowId), Name: *req.Body.Name, ExpectedRevision: expected, By: actorOf(principal),
	}); err != nil {
		return nil, nameError(err)
	}
	etag := flowETag(rec)
	return apigen.UpdateFlow200JSONResponse{Body: flowBody(rec), Headers: apigen.UpdateFlow200ResponseHeaders{ETag: &etag}}, nil
}

// nameError turns a refused name into a field error under /name.
func nameError(err error) error {
	if errors.Is(err, identity.ErrInvalidInput) {
		return &documentIssuesError{fields: []problem.FieldError{{
			Pointer: "/name", Code: "out-of-range", Message: "The name has 1 to 200 characters.",
		}}}
	}
	return err
}

// DeleteFlow deletes the flow and switches its automatic triggers off when If-Match names the
// current revision of the flow.
func (f *Flows) DeleteFlow(ctx context.Context, req apigen.DeleteFlowRequestObject) (apigen.DeleteFlowResponseObject, error) { //nolint:ireturn // the generated interface requires it
	cmd, err := f.change(ctx, req.FlowId, req.Params.IfMatch)
	if err != nil {
		return nil, err
	}
	if err := f.lifecycle.DeleteFlow(ctx, cmd); err != nil {
		return nil, err
	}
	return apigen.DeleteFlow204Response{}, nil
}

// ActivateFlow switches the automatic triggers of the flow on when If-Match names the current
// revision of the flow. A flow that was never published is a conflict.
func (f *Flows) ActivateFlow(ctx context.Context, req apigen.ActivateFlowRequestObject) (apigen.ActivateFlowResponseObject, error) { //nolint:ireturn // the generated interface requires it
	cmd, err := f.change(ctx, req.FlowId, req.Params.IfMatch)
	if err != nil {
		return nil, err
	}
	if err := f.lifecycle.ActivateFlow(ctx, cmd); err != nil {
		return nil, err
	}
	rec, err := f.manager.Get(ctx, cmd.Org, cmd.Flow)
	if err != nil {
		return nil, err
	}
	etag := flowETag(rec)
	return apigen.ActivateFlow200JSONResponse{Body: flowBody(rec), Headers: apigen.ActivateFlow200ResponseHeaders{ETag: &etag}}, nil
}

// DeactivateFlow switches the automatic triggers of the flow off when If-Match names the current
// revision of the flow.
func (f *Flows) DeactivateFlow(ctx context.Context, req apigen.DeactivateFlowRequestObject) (apigen.DeactivateFlowResponseObject, error) { //nolint:ireturn // the generated interface requires it
	cmd, err := f.change(ctx, req.FlowId, req.Params.IfMatch)
	if err != nil {
		return nil, err
	}
	if err := f.lifecycle.DeactivateFlow(ctx, cmd); err != nil {
		return nil, err
	}
	rec, err := f.manager.Get(ctx, cmd.Org, cmd.Flow)
	if err != nil {
		return nil, err
	}
	etag := flowETag(rec)
	return apigen.DeactivateFlow200JSONResponse{Body: flowBody(rec), Headers: apigen.DeactivateFlow200ResponseHeaders{ETag: &etag}}, nil
}

// change builds the command of a change that carries the revision of the flow in If-Match.
func (f *Flows) change(ctx context.Context, flowID types.UUID, ifMatch string) (*trigger.ChangeCommand, error) {
	if f.lifecycle == nil {
		return nil, errUnavailable
	}
	org, err := scopeOf(ctx)
	if err != nil {
		return nil, err
	}
	principal, err := principalOf(ctx)
	if err != nil {
		return nil, err
	}
	expected, err := revisionOf(ifMatch)
	if err != nil {
		return nil, err
	}
	return &trigger.ChangeCommand{Org: org, Flow: flow.FlowID(flowID), Actor: actorOf(principal), ExpectedRevision: &expected}, nil
}

// PublishFlow publishes the draft of the flow as the next version when If-Match names the current
// revision of the draft. A draft with errors is refused with the findings as field errors.
func (f *Flows) PublishFlow(ctx context.Context, req apigen.PublishFlowRequestObject) (apigen.PublishFlowResponseObject, error) { //nolint:ireturn // the generated interface requires it
	if f.lifecycle == nil {
		return nil, errUnavailable
	}
	org, err := scopeOf(ctx)
	if err != nil {
		return nil, err
	}
	principal, err := principalOf(ctx)
	if err != nil {
		return nil, err
	}
	var note *string
	if req.Body != nil {
		note = req.Body.Note
	}
	text, err := noteOf(note)
	if err != nil {
		return nil, err
	}
	expected, err := revisionOf(req.Params.IfMatch)
	if err != nil {
		return nil, f.staleOr(ctx, org, req.FlowId, err)
	}
	published, err := f.lifecycle.Publish(ctx, trigger.PublishCommand{
		Org: org, Flow: flow.FlowID(req.FlowId), By: principal.ID, Actor: actorOf(principal),
		ExpectedDraftRevision: &expected, Note: text,
	})
	if err != nil {
		return nil, f.publishError(ctx, org, req.FlowId, err)
	}
	body, err := versionBody(&published.Version, false)
	if err != nil {
		return nil, err
	}
	return apigen.PublishFlow200JSONResponse{Body: body}, nil
}

// RollbackFlow publishes the content of an earlier version again as the next version when
// If-Match names the current revision of the flow.
func (f *Flows) RollbackFlow(ctx context.Context, req apigen.RollbackFlowRequestObject) (apigen.RollbackFlowResponseObject, error) { //nolint:ireturn // the generated interface requires it
	if f.lifecycle == nil {
		return nil, errUnavailable
	}
	org, err := scopeOf(ctx)
	if err != nil {
		return nil, err
	}
	principal, err := principalOf(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, identity.ErrInvalidInput
	}
	text, err := noteOf(req.Body.Note)
	if err != nil {
		return nil, err
	}
	expected, err := revisionOf(req.Params.IfMatch)
	if err != nil {
		return nil, err
	}
	published, err := f.lifecycle.Rollback(ctx, trigger.RollbackCommand{
		Org: org, Flow: flow.FlowID(req.FlowId), By: principal.ID, Actor: actorOf(principal),
		ToVersion: int64(req.Body.Version), ExpectedRevision: &expected, Note: text,
	})
	if err != nil {
		return nil, f.publishError(ctx, org, req.FlowId, err)
	}
	body, err := versionBody(&published.Version, false)
	if err != nil {
		return nil, err
	}
	return apigen.RollbackFlow200JSONResponse{Body: body}, nil
}

// noteOf checks the release note of a publication.
func noteOf(note *string) (string, error) {
	if note == nil {
		return "", nil
	}
	if utf8.RuneCountInString(*note) > maxNote {
		return "", &documentIssuesError{fields: []problem.FieldError{{
			Pointer: "/note", Code: "out-of-range", Message: fmt.Sprintf("The note has at most %d characters.", maxNote),
		}}}
	}
	return *note, nil
}

// staleOr answers a precondition that does not name a revision this API issued: 412 with the
// current revision of the draft when the flow exists, the error of the read otherwise.
func (f *Flows) staleOr(ctx context.Context, org orgscope.Org, flowID types.UUID, cause error) error {
	rec, err := f.drafts.Get(ctx, org, flow.FlowID(flowID))
	if err != nil {
		return err
	}
	if errors.Is(cause, conditional.ErrMismatch) {
		return &staleDraftError{revision: rec.DraftRevision}
	}
	return cause
}

// publishError turns the failures of a publication or rollback into answers: a draft that moved is
// 412 with its current revision, and the findings of the validation are field errors.
func (f *Flows) publishError(ctx context.Context, org orgscope.Org, flowID types.UUID, err error) error {
	if errors.Is(err, flow.ErrDraftConflict) {
		return f.staleOr(ctx, org, flowID, conditional.ErrMismatch)
	}
	return documentError(err)
}

// ListFlowVersions lists the published versions of the flow without their documents, newest
// first by default.
func (f *Flows) ListFlowVersions(ctx context.Context, req apigen.ListFlowVersionsRequestObject) (apigen.ListFlowVersionsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, err := scopeOf(ctx)
	if err != nil {
		return nil, err
	}
	limit, err := pageLimit(req.Params.Limit)
	if err != nil {
		return nil, err
	}
	sortExpr := ""
	if req.Params.Sort != nil {
		sortExpr = string(*req.Params.Sort)
	}
	terms, err := page.ParseSort(sortExpr, versionSortFields, "-version")
	if err != nil {
		return nil, err
	}
	flowID := flow.FlowID(req.FlowId)
	if _, err := f.manager.Get(ctx, org, flowID); err != nil {
		return nil, err
	}
	binding := page.Binding("sort="+page.String(terms), "flow="+id.UUID(flowID).String())
	cursor, err := cursorFor(f.codec, derefString(req.Params.PageToken), org.String(), binding, terms)
	if err != nil {
		return nil, err
	}
	rows, err := f.readVersions(ctx, org, flowID)
	if err != nil {
		return nil, err
	}
	items := make([]listItem, len(rows))
	for i := range rows {
		items[i] = listItem{keys: map[string]string{"version": fmt.Sprintf("%020d", rows[i].Number)}, id: id.UUID(rows[i].ID).String(), idx: i}
	}
	window, more, last := listWindow(items, terms, limit, cursor)
	body := apigen.FlowVersionList{Items: []apigen.FlowVersion{}}
	for _, it := range window {
		item, err := versionBody(&rows[it.idx], false)
		if err != nil {
			return nil, err
		}
		body.Items = append(body.Items, item)
	}
	if more {
		token, err := tokenFor(f.codec, org.String(), binding, last)
		if err != nil {
			return nil, err
		}
		body.NextPageToken = &token
	}
	return apigen.ListFlowVersions200JSONResponse{Body: body}, nil
}

// readVersions reads the versions of the flow, at most maxListRows of them, in number order.
func (f *Flows) readVersions(ctx context.Context, org orgscope.Org, flowID flow.FlowID) ([]flow.VersionRecord, error) {
	var out []flow.VersionRecord
	var after int64
	for len(out) < maxListRows {
		batch, err := f.versions.ListVersions(ctx, org, flowID, after, page.MaxLimit)
		if err != nil {
			return nil, err
		}
		out = append(out, batch...)
		if len(batch) < page.MaxLimit {
			break
		}
		after = batch[len(batch)-1].Number
	}
	return out, nil
}

// GetFlowVersion returns one published version with its document.
func (f *Flows) GetFlowVersion(ctx context.Context, req apigen.GetFlowVersionRequestObject) (apigen.GetFlowVersionResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, err := scopeOf(ctx)
	if err != nil {
		return nil, err
	}
	flowID := flow.FlowID(req.FlowId)
	if _, err := f.manager.Get(ctx, org, flowID); err != nil {
		return nil, err
	}
	rec, err := f.versions.GetVersionByNumber(ctx, org, flowID, int64(req.Version))
	if err != nil {
		return nil, err
	}
	body, err := versionBody(&rec, true)
	if err != nil {
		return nil, err
	}
	return apigen.GetFlowVersion200JSONResponse{Body: body}, nil
}

// ValidateDraft runs on the stored draft every check that publication would run and returns the
// findings. It changes nothing.
func (f *Flows) ValidateDraft(ctx context.Context, req apigen.ValidateDraftRequestObject) (apigen.ValidateDraftResponseObject, error) { //nolint:ireturn // the generated interface requires it
	if f.lifecycle == nil {
		return nil, errUnavailable
	}
	org, err := scopeOf(ctx)
	if err != nil {
		return nil, err
	}
	flowID := flow.FlowID(req.FlowId)
	rec, err := f.drafts.Get(ctx, org, flowID)
	if err != nil {
		return nil, err
	}
	report, err := f.lifecycle.Validate(ctx, org, flowID)
	if err != nil {
		return nil, err
	}
	issues := make([]apigen.ValidationIssue, len(report.Issues))
	for i, is := range report.Issues {
		issues[i] = issueBody(is)
	}
	return apigen.ValidateDraft200JSONResponse{Body: apigen.ValidationResult{
		Valid: !report.HasErrors(), DraftRevision: int(rec.DraftRevision), Issues: issues,
	}}, nil
}

func issueBody(is flow.Issue) apigen.ValidationIssue {
	return apigen.ValidationIssue{
		Severity: apigen.ValidationIssueSeverity(is.Severity), Code: is.Code, Path: is.Path, Message: is.Message,
	}
}

// versionBody is a version as the API shows it; the document is only in the answer that reads one version.
func versionBody(rec *flow.VersionRecord, withDocument bool) (apigen.FlowVersion, error) {
	out := apigen.FlowVersion{
		FlowId: types.UUID(rec.FlowID), Version: int(rec.Number), ContentHash: rec.ContentHash,
		PublishedAt: rec.PublishedAt, Warnings: []apigen.ValidationIssue{},
	}
	if !rec.PublishedByPrincipalID.IsZero() {
		by := types.UUID(rec.PublishedByPrincipalID)
		out.PublishedBy = &by
	}
	if rec.Note != "" {
		note := rec.Note
		out.Note = &note
	}
	if rec.RolledBackFromVersion > 0 {
		from := int(rec.RolledBackFromVersion)
		out.RolledBackFromVersion = &from
	}
	if len(rec.Warnings) > 0 {
		if err := json.Unmarshal(rec.Warnings, &out.Warnings); err != nil {
			return apigen.FlowVersion{}, fmt.Errorf("read the warnings of version %d of flow %s: %w", rec.Number, id.UUID(rec.FlowID), err)
		}
	}
	if withDocument {
		var doc apigen.FlowDocument
		if err := json.Unmarshal(rec.Document, &doc); err != nil {
			return apigen.FlowVersion{}, fmt.Errorf("read version %d of flow %s: %w", rec.Number, id.UUID(rec.FlowID), err)
		}
		out.Document = &doc
	}
	return out, nil
}
