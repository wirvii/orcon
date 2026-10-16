// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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
	etag := flowETag(rec)
	return apigen.GetDraft200JSONResponse{Body: body, Headers: apigen.GetDraft200ResponseHeaders{ETag: &etag}}, nil
}

// ReplaceDraft replaces the whole document of the draft when If-Match names the current revision.
func (d *Drafts) ReplaceDraft(ctx context.Context, req apigen.ReplaceDraftRequestObject) (apigen.ReplaceDraftResponseObject, error) { //nolint:ireturn // the generated interface requires it
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
	expected, err := conditional.Parse(req.Params.IfMatch)
	if err != nil {
		// A tag this API did not issue names no current revision: the client reads again.
		return nil, d.stale(ctx, org, req.FlowId)
	}
	document, err := json.Marshal(req.Body.Document)
	if err != nil {
		return nil, fmt.Errorf("%w: the document cannot be read", identity.ErrInvalidInput)
	}
	rec, err := d.drafts.Save(ctx, org, flow.SaveInput{
		FlowID: flow.FlowID(req.FlowId), Document: document, ExpectedRevision: expected, By: actorOf(principal),
	})
	switch {
	case errors.Is(err, flow.ErrDraftConflict):
		return nil, d.stale(ctx, org, req.FlowId)
	case err != nil:
		return nil, documentError(err)
	}
	body, err := draftBody(rec)
	if err != nil {
		return nil, err
	}
	etag := flowETag(rec)
	return apigen.ReplaceDraft200JSONResponse{Body: body, Headers: apigen.ReplaceDraft200ResponseHeaders{ETag: &etag}}, nil
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
