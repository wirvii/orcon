// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/oapi-codegen/runtime/types"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/conditional"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/credential"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// CredentialService is what the credential handlers need; [credential.Service] implements it. The handlers call it
// only after the request pipeline has decided the permission of the operation (SPEC-004 SE6-http).
//
//nolint:interfacebloat // the handlers serve every credential operation of the contract, so they need each method
type CredentialService interface {
	Create(ctx context.Context, org orgscope.Org, project identity.ProjectID, in credential.Input, actor audit.Actor) (credential.Summary, error)
	Get(ctx context.Context, org orgscope.Org, project identity.ProjectID, cred id.UUID) (credential.Summary, error)
	List(ctx context.Context, org orgscope.Org, project identity.ProjectID) ([]credential.Summary, error)
	Replace(ctx context.Context, org orgscope.Org, project identity.ProjectID, cred id.UUID, in credential.Input, actor audit.Actor) (credential.Summary, error)
	Update(ctx context.Context, org orgscope.Org, project identity.ProjectID, cred id.UUID, set credential.Settings, expected time.Time, actor audit.Actor) (credential.Summary, error)
	Delete(ctx context.Context, org orgscope.Org, project identity.ProjectID, cred id.UUID, expected time.Time, actor audit.Actor) error
	Share(ctx context.Context, org orgscope.Org, project identity.ProjectID, cred id.UUID, target identity.ProjectID, actor audit.Actor) error
	Unshare(ctx context.Context, org orgscope.Org, project identity.ProjectID, cred id.UUID, target identity.ProjectID, actor audit.Actor) error
	OwnerProject(ctx context.Context, org orgscope.Org, cred string) (identity.ProjectID, error)
}

// Credentials serves the credential operations of the contract (SPEC-005 section 3, SPEC-004 SE7). Secret values go
// in through the requests and never come out: every response is built from [credential.Summary], which holds none.
type Credentials struct {
	svc CredentialService
}

// NewCredentials returns the handler over the credential service.
func NewCredentials(svc CredentialService) *Credentials {
	return &Credentials{svc: svc}
}

// credentialsPageSize bounds the list of credentials; it uses the same limits as the other listings.
const (
	credentialsDefaultPage = 50
	credentialsMaxPage     = 200
)

// staleRevisionError means If-Match names a revision that is no longer current. The response carries the current ETag so
// the caller can read again; the error handler turns it into 412 precondition-failed.
type staleRevisionError struct{ etag string }

func (staleRevisionError) Error() string { return "the credential changed since it was read" }

// ListCredentials lists the credentials a project owns or may use, by name and then id, one page at a time.
func (c *Credentials) ListCredentials(ctx context.Context, req apigen.ListCredentialsRequestObject) (apigen.ListCredentialsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, _, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	limit := credentialsDefaultPage
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	if limit < 1 || limit > credentialsMaxPage {
		return nil, identity.ErrInvalidInput
	}
	var after *credentialCursor
	if req.Params.PageToken != nil {
		cur, err := decodeCredentialCursor(*req.Params.PageToken, limit)
		if err != nil {
			return nil, err
		}
		after = &cur
	}
	all, err := c.svc.List(ctx, org, identity.ProjectID(id.UUID(req.ProjectId)))
	if err != nil {
		return nil, err
	}
	slices.SortFunc(all, func(a, b credential.Summary) int {
		if n := strings.Compare(a.Name, b.Name); n != 0 {
			return n
		}
		return strings.Compare(a.ID.String(), b.ID.String())
	})
	page := make([]credential.Summary, 0, limit+1)
	for i := range all {
		if after != nil && !after.before(&all[i]) {
			continue
		}
		page = append(page, all[i])
		if len(page) > limit {
			break
		}
	}
	body := apigen.CredentialList{Items: []apigen.Credential{}}
	more := len(page) > limit
	if more {
		page = page[:limit]
	}
	for i := range page {
		body.Items = append(body.Items, credentialBody(&page[i], false))
	}
	if more {
		token, err := encodeCredentialCursor(&page[len(page)-1], limit)
		if err != nil {
			return nil, err
		}
		body.NextPageToken = &token
	}
	return apigen.ListCredentials200JSONResponse{Body: body}, nil
}

// CreateCredential stores a new credential in a project, with its secret fields encrypted.
func (c *Credentials) CreateCredential(ctx context.Context, req apigen.CreateCredentialRequestObject) (apigen.CreateCredentialResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, actor, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, identity.ErrInvalidInput
	}
	in := credential.Input{
		Name:               req.Body.Name,
		ConnectorReference: req.Body.Type,
		Secret:             req.Body.SecretFields,
	}
	if req.Body.AllowedHosts != nil {
		in.AllowedHosts = *req.Body.AllowedHosts
	}
	if req.Body.PublicFields != nil {
		in.Public = *req.Body.PublicFields
	}
	s, err := c.svc.Create(ctx, org, identity.ProjectID(id.UUID(req.ProjectId)), in, actor)
	if err != nil {
		return nil, err
	}
	etag := conditional.ETag(credentialRevision(&s))
	location := "/api/v1/credentials/" + s.ID.String()
	return apigen.CreateCredential201JSONResponse{
		Body:    credentialBody(&s, true),
		Headers: apigen.CreateCredential201ResponseHeaders{ETag: &etag, Location: &location},
	}, nil
}

// GetCredential returns one credential with its public fields and the state of its secret.
func (c *Credentials) GetCredential(ctx context.Context, req apigen.GetCredentialRequestObject) (apigen.GetCredentialResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, _, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	owner, cid, err := c.ownerOf(ctx, org, req.CredentialId)
	if err != nil {
		return nil, err
	}
	s, err := c.svc.Get(ctx, org, owner, cid)
	if err != nil {
		return nil, err
	}
	etag := conditional.ETag(credentialRevision(&s))
	return apigen.GetCredential200JSONResponse{Body: credentialBody(&s, true), Headers: apigen.GetCredential200ResponseHeaders{ETag: &etag}}, nil
}

// UpdateCredential changes the name and the allowed hosts. Fields that the request leaves out keep their value; the
// If-Match revision must be the current one.
func (c *Credentials) UpdateCredential(ctx context.Context, req apigen.UpdateCredentialRequestObject) (apigen.UpdateCredentialResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, actor, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	owner, cid, err := c.ownerOf(ctx, org, req.CredentialId)
	if err != nil {
		return nil, err
	}
	cur, err := c.svc.Get(ctx, org, owner, cid)
	if err != nil {
		return nil, err
	}
	if err := checkRevision(req.Params.IfMatch, &cur); err != nil {
		return nil, err
	}
	set := credential.Settings{Name: cur.Name, AllowedHosts: cur.AllowedHosts}
	if req.Body != nil {
		if req.Body.Name != nil {
			set.Name = *req.Body.Name
		}
		if req.Body.AllowedHosts != nil {
			set.AllowedHosts = *req.Body.AllowedHosts
		}
	}
	s, err := c.svc.Update(ctx, org, owner, cid, set, cur.UpdatedAt, actor)
	if err != nil {
		return nil, c.staleOnConflict(ctx, org, owner, cid, err)
	}
	etag := conditional.ETag(credentialRevision(&s))
	return apigen.UpdateCredential200JSONResponse{Body: credentialBody(&s, true), Headers: apigen.UpdateCredential200ResponseHeaders{ETag: &etag}}, nil
}

// ReplaceCredentialSecret stores a new version of the secret. When the request leaves out the public fields, the
// public fields of the current version are kept.
func (c *Credentials) ReplaceCredentialSecret(ctx context.Context, req apigen.ReplaceCredentialSecretRequestObject) (apigen.ReplaceCredentialSecretResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, actor, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	owner, cid, err := c.ownerOf(ctx, org, req.CredentialId)
	if err != nil {
		return nil, err
	}
	cur, err := c.svc.Get(ctx, org, owner, cid)
	if err != nil {
		return nil, err
	}
	if err := checkRevision(req.Params.IfMatch, &cur); err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, identity.ErrInvalidInput
	}
	in := credential.Input{Secret: req.Body.SecretFields, Public: cur.Public}
	if req.Body.PublicFields != nil {
		in.Public = *req.Body.PublicFields
	}
	s, err := c.svc.Replace(ctx, org, owner, cid, in, actor)
	if err != nil {
		return nil, err
	}
	etag := conditional.ETag(credentialRevision(&s))
	return apigen.ReplaceCredentialSecret200JSONResponse{Body: credentialBody(&s, true), Headers: apigen.ReplaceCredentialSecret200ResponseHeaders{ETag: &etag}}, nil
}

// DeleteCredential removes a credential with its versions and its shares.
func (c *Credentials) DeleteCredential(ctx context.Context, req apigen.DeleteCredentialRequestObject) (apigen.DeleteCredentialResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, actor, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	owner, cid, err := c.ownerOf(ctx, org, req.CredentialId)
	if err != nil {
		return nil, err
	}
	cur, err := c.svc.Get(ctx, org, owner, cid)
	if err != nil {
		return nil, err
	}
	if err := checkRevision(req.Params.IfMatch, &cur); err != nil {
		return nil, err
	}
	if err := c.svc.Delete(ctx, org, owner, cid, cur.UpdatedAt, actor); err != nil {
		return nil, c.staleOnConflict(ctx, org, owner, cid, err)
	}
	return apigen.DeleteCredential204Response{}, nil
}

// CreateCredentialShare lets another project of the organization use the credential.
func (c *Credentials) CreateCredentialShare(ctx context.Context, req apigen.CreateCredentialShareRequestObject) (apigen.CreateCredentialShareResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, actor, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	if req.Body == nil {
		return nil, identity.ErrInvalidInput
	}
	owner, cid, err := c.ownerOf(ctx, org, req.CredentialId)
	if err != nil {
		return nil, err
	}
	if err := c.svc.Share(ctx, org, owner, cid, identity.ProjectID(id.UUID(req.Body.ProjectId)), actor); err != nil {
		return nil, err
	}
	return apigen.CreateCredentialShare204Response{}, nil
}

// DeleteCredentialShare stops a project from using the credential. The If-Match revision is the credential's.
func (c *Credentials) DeleteCredentialShare(ctx context.Context, req apigen.DeleteCredentialShareRequestObject) (apigen.DeleteCredentialShareResponseObject, error) { //nolint:ireturn // the generated interface requires it
	org, actor, err := callerOf(ctx)
	if err != nil {
		return nil, err
	}
	owner, cid, err := c.ownerOf(ctx, org, req.CredentialId)
	if err != nil {
		return nil, err
	}
	cur, err := c.svc.Get(ctx, org, owner, cid)
	if err != nil {
		return nil, err
	}
	if err := checkRevision(req.Params.IfMatch, &cur); err != nil {
		return nil, err
	}
	if err := c.svc.Unshare(ctx, org, owner, cid, identity.ProjectID(id.UUID(req.ProjectId)), actor); err != nil {
		return nil, err
	}
	return apigen.DeleteCredentialShare204Response{}, nil
}

// staleOnConflict turns the refusal of a write that carried the revision the caller had checked into the
// 412 answer with the current ETag: another change won the race between the check and the write. Any other
// error is returned as it is. When the credential is gone by then, the answer is the lookup's (not found).
func (c *Credentials) staleOnConflict(ctx context.Context, org orgscope.Org, owner identity.ProjectID, cid id.UUID, err error) error {
	if !errors.Is(err, credential.ErrConflict) {
		return err
	}
	cur, getErr := c.svc.Get(ctx, org, owner, cid)
	if getErr != nil {
		return getErr
	}
	return staleRevisionError{etag: conditional.ETag(credentialRevision(&cur))}
}

// ownerOf returns the project that owns the credential named in the URL, inside the caller's organization. A credential
// of another organization is not found, as the evaluator already decided.
func (c *Credentials) ownerOf(ctx context.Context, org orgscope.Org, cred apigen.CredentialId) (identity.ProjectID, id.UUID, error) {
	cid := id.UUID(cred)
	owner, err := c.svc.OwnerProject(ctx, org, cid.String())
	if err != nil {
		return identity.ProjectID{}, id.UUID{}, err
	}
	return owner, cid, nil
}

// callerOf returns the organization scope the request acts in and the audit actor of the caller.
func callerOf(ctx context.Context) (orgscope.Org, audit.Actor, error) {
	p, ok := middleware.PrincipalFrom(ctx)
	if !ok || p.IsZero() {
		return orgscope.Org{}, audit.Actor{}, identity.ErrUnauthenticated
	}
	org, err := authz.OrganizationOf(p)
	if err != nil {
		return orgscope.Org{}, audit.Actor{}, identity.ErrNotFound
	}
	var kind audit.ActorKind
	switch p.Kind {
	case identity.PrincipalUser:
		kind = audit.ActorUser
	case identity.PrincipalServiceAccount:
		kind = audit.ActorServiceAccount
	case identity.PrincipalSystem:
		return orgscope.Org{}, audit.Actor{}, identity.ErrUnauthenticated
	default:
		return orgscope.Org{}, audit.Actor{}, identity.ErrUnauthenticated
	}
	return org, audit.Actor{PrincipalID: id.UUID(p.ID), Kind: kind}, nil
}

// credentialRevision is the revision of a credential for optimistic concurrency: its last change, in microseconds
// (the precision the stores keep). Every change that a client can make moves it.
func credentialRevision(s *credential.Summary) int64 {
	return s.UpdatedAt.UnixMicro()
}

// checkRevision compares the If-Match of a change with the current revision. A missing If-Match is refused by the
// generated code before this point; a stale or malformed one is a staleRevisionError carrying the current ETag.
func checkRevision(ifMatch string, cur *credential.Summary) error {
	rev, err := conditional.Parse(ifMatch)
	if err != nil || rev != credentialRevision(cur) {
		return staleRevisionError{etag: conditional.ETag(credentialRevision(cur))}
	}
	return nil
}

// credentialBody is the wire form of a summary. The public fields travel only when the caller asks for them: list
// items omit them. The secret never does; the body says only whether one is set and when and by whom it was written.
func credentialBody(s *credential.Summary, withPublic bool) apigen.Credential {
	cid := types.UUID(s.ID)
	out := apigen.Credential{
		Id:           &cid,
		ProjectId:    types.UUID(s.ProjectID),
		Owned:        s.Owned,
		Name:         s.Name,
		Type:         s.ConnectorReference,
		Status:       apigen.CredentialStatus(s.Status),
		AllowedHosts: append([]string{}, s.AllowedHosts...),
		VersionCount: s.CurrentVersion,
		Secret:       apigen.CredentialSecret{Set: s.Secret.Set},
		CreatedAt:    timePtr(s.CreatedAt),
		UpdatedAt:    timePtr(s.UpdatedAt),
	}
	if s.Secret.Set {
		at := s.Secret.UpdatedAt
		out.Secret.UpdatedAt = &at
		by := types.UUID(s.Secret.UpdatedBy)
		out.Secret.UpdatedBy = &by
	}
	if withPublic {
		pub := make(map[string]string, len(s.Public))
		for k, v := range s.Public {
			pub[k] = v
		}
		out.PublicFields = &pub
	}
	return out
}

// timePtr returns a pointer to t, for the optional instants of the wire form.
func timePtr(t time.Time) *time.Time { return &t }

// credentialCursor is the position of the last credential of a page: its name and id. A token is bound to the page
// size it was issued for, as the other listings bind theirs.
type credentialCursor struct {
	Name  string `json:"n"`
	ID    string `json:"i"`
	Limit int    `json:"l"`
}

// before reports whether s comes after the cursor in the (name, id) order.
func (c credentialCursor) before(s *credential.Summary) bool {
	if s.Name != c.Name {
		return s.Name > c.Name
	}
	return s.ID.String() > c.ID
}

// encodeCredentialCursor returns the opaque token of the position after s.
func encodeCredentialCursor(s *credential.Summary, limit int) (string, error) {
	raw, err := json.Marshal(credentialCursor{Name: s.Name, ID: s.ID.String(), Limit: limit})
	if err != nil {
		return "", fmt.Errorf("encode page token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// decodeCredentialCursor reads a page token. A token that does not decode, or that was issued for another page size,
// is errInvalidPageToken.
func decodeCredentialCursor(token string, limit int) (credentialCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return credentialCursor{}, errInvalidPageToken
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var c credentialCursor
	if err := dec.Decode(&c); err != nil || c.ID == "" || c.Limit != limit {
		return credentialCursor{}, errInvalidPageToken
	}
	return c, nil
}
