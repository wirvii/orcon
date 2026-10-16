// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/oapi-codegen/runtime/types"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/conditional"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/adapter/httpapi/problem"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// Drafts serves the draft of a flow (SPEC-005 Amendment 8 sections 8 and 9): getDraft and
// replaceDraft. Each one runs after authorization, so the organization is the one the
// authorization decided (authz.OrgScopeFrom).
type Drafts struct {
	drafts *flow.Drafts
}

// NewDrafts returns the draft handlers over the draft service.
func NewDrafts(drafts *flow.Drafts) *Drafts { return &Drafts{drafts: drafts} }

// staleDraftError is the failure of a save whose If-Match names a revision that is no longer
// current. The error handler answers it as 412 precondition-failed with the current revision.
type staleDraftError struct {
	revision int64
}

func (e *staleDraftError) Error() string {
	return fmt.Sprintf("the draft is at revision %d, not the revision of If-Match", e.revision)
}

// documentIssuesError carries the findings of a document that does not have the shape of the
// format, as per-field errors with their pointer into the request.
type documentIssuesError struct {
	fields []problem.FieldError
}

func (e *documentIssuesError) Error() string {
	if len(e.fields) == 0 {
		return "the document is invalid"
	}
	return fmt.Sprintf("the document is invalid (%d findings, first at %s: %s)", len(e.fields), e.fields[0].Pointer, e.fields[0].Message)
}

// GetDraft returns the draft of the flow with its revision as the ETag.
func (d *Drafts) GetDraft(ctx context.Context, req apigen.GetDraftRequestObject) (apigen.GetDraftResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, err := scopeOf(ctx)
	if err != nil {
		return nil, err
	}
	rec, err := d.drafts.Get(ctx, org, flow.FlowID(req.FlowId))
	if err != nil {
		return nil, err
	}
	body, err := draftBody(rec)
	if err != nil {
		return nil, err
	}
	etag := draftETag(rec)
	return apigen.GetDraft200JSONResponse{Body: body, Headers: apigen.GetDraft200ResponseHeaders{ETag: &etag}}, nil
}

// ReplaceDraft replaces the whole document of the draft when If-Match names the current revision.
func (d *Drafts) ReplaceDraft(ctx context.Context, req apigen.ReplaceDraftRequestObject) (apigen.ReplaceDraftResponseObject, error) { //nolint:ireturn // the generated interface requires it
	if req.Body == nil {
		return nil, identity.ErrInvalidInput
	}
	document, err := json.Marshal(req.Body.Document)
	if err != nil {
		return nil, fmt.Errorf("%w: the document cannot be read", identity.ErrInvalidInput)
	}
	rec, err := d.save(ctx, req.FlowId, req.Params.IfMatch, func(flow.Record) ([]byte, error) { return document, nil })
	if err != nil {
		return nil, err
	}
	body, err := draftBody(rec)
	if err != nil {
		return nil, err
	}
	etag := draftETag(rec)
	return apigen.ReplaceDraft200JSONResponse{Body: body, Headers: apigen.ReplaceDraft200ResponseHeaders{ETag: &etag}}, nil
}

// UpdateDraft applies a JSON Patch (RFC 6902) to the document of the draft when If-Match names the
// current revision. The patch is all or nothing: the document it produces is saved by the same
// checks as a replacement, so a patch cannot leave a draft that is not a flow document.
func (d *Drafts) UpdateDraft(ctx context.Context, req apigen.UpdateDraftRequestObject) (apigen.UpdateDraftResponseObject, error) { //nolint:ireturn // the generated interface requires it
	if req.Body == nil {
		return nil, identity.ErrInvalidInput
	}
	rec, err := d.save(ctx, req.FlowId, req.Params.IfMatch, func(current flow.Record) ([]byte, error) {
		return applyJSONPatch(current.Draft, *req.Body)
	})
	if err != nil {
		return nil, err
	}
	body, err := draftBody(rec)
	if err != nil {
		return nil, err
	}
	etag := draftETag(rec)
	return apigen.UpdateDraft200JSONResponse{Body: body, Headers: apigen.UpdateDraft200ResponseHeaders{ETag: &etag}}, nil
}

// save is the save both writes share: it checks the revision of If-Match, builds the new document
// from the draft as it is, and saves it when the revision is still current at the moment of the
// write.
func (d *Drafts) save(ctx context.Context, flowID types.UUID, ifMatch string, build func(current flow.Record) ([]byte, error)) (flow.Record, error) {
	org, err := scopeOf(ctx)
	if err != nil {
		return flow.Record{}, err
	}
	principal, ok := middleware.PrincipalFrom(ctx)
	if !ok || principal.UserID.IsZero() {
		return flow.Record{}, identity.ErrUnauthenticated
	}
	if strings.TrimSpace(ifMatch) == "" {
		return flow.Record{}, conditional.ErrRequired
	}
	expected, err := conditional.Parse(ifMatch)
	if err != nil {
		// A tag this API did not issue names no current revision: the client reads again.
		return flow.Record{}, d.stale(ctx, org, flowID)
	}
	current, err := d.drafts.Get(ctx, org, flow.FlowID(flowID))
	if err != nil {
		return flow.Record{}, err
	}
	if current.DraftRevision != expected {
		return flow.Record{}, &staleDraftError{revision: current.DraftRevision}
	}
	document, err := build(current)
	if err != nil {
		return flow.Record{}, err
	}
	rec, err := d.drafts.Save(ctx, org, flow.SaveInput{
		FlowID: flow.FlowID(flowID), Document: document, ExpectedRevision: expected, By: actorOf(principal),
	})
	switch {
	case errors.Is(err, flow.ErrDraftConflict):
		return flow.Record{}, d.stale(ctx, org, flowID)
	case err != nil:
		return flow.Record{}, documentError(err)
	}
	return rec, nil
}

// stale builds the error of a save based on an old revision: it reads the revision stored now,
// or the error of the read (a flow that vanished is 404).
func (d *Drafts) stale(ctx context.Context, org orgscope.Org, flowID types.UUID) error {
	rec, err := d.drafts.Get(ctx, org, flow.FlowID(flowID))
	if err != nil {
		return err
	}
	return &staleDraftError{revision: rec.DraftRevision}
}

// documentError turns the shape errors of a document into per-field errors under /document.
func documentError(err error) error {
	var invalid *flow.ValidationError
	if errors.As(err, &invalid) {
		out := &documentIssuesError{}
		for _, issue := range invalid.Report.Errors() {
			out.fields = append(out.fields, problem.FieldError{Pointer: "/document" + issue.Path, Code: issue.Code, Message: issue.Message})
		}
		return out
	}
	var unknown *flow.UnknownFormatError
	if errors.As(err, &unknown) {
		return &documentIssuesError{fields: []problem.FieldError{{
			Pointer: "/document/format", Code: "unknown-format", Message: unknown.Error(),
		}}}
	}
	return err
}

// draftBody is the draft of a flow as the API shows it.
func draftBody(rec flow.Record) (apigen.FlowDraft, error) {
	var doc apigen.FlowDocument
	if err := json.Unmarshal(rec.Draft, &doc); err != nil {
		return apigen.FlowDraft{}, fmt.Errorf("read the stored draft of flow %s: %w", id.UUID(rec.ID), err)
	}
	return apigen.FlowDraft{
		FlowId: types.UUID(rec.ID), Revision: int(rec.DraftRevision), Document: doc, UpdatedAt: rec.UpdatedAt,
	}, nil
}
