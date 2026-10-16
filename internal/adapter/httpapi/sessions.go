// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/oapi-codegen/runtime/types"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/conditional"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// SessionService is the part of the session service that the operations of SPEC-016
// section 3.2 need. The identity session service implements it.
type SessionService interface {
	SignIn(ctx context.Context, in identity.SignIn) (identity.SignedIn, error)
	Current(ctx context.Context, principal identity.Principal) (identity.CurrentUser, error)
	Revoke(ctx context.Context, actor identity.Principal, session identity.SessionID) error
	ListMine(ctx context.Context, actor identity.Principal) ([]identity.SessionRecord, error)
	Rename(ctx context.Context, actor identity.Principal, displayName string, expected time.Time) (identity.UserRecord, error)
}

// Sessions serves createSession, revokeSession, getCurrentUser (SPEC-016 section 3.2),
// listSessions and updateCurrentUser (SPEC-004 R-ID-08, SPEC-019 F-03 and F-04).
type Sessions struct {
	svc    SessionService
	cookie middleware.AuthConfig
	origin string
	orgs   identity.OrganizationResolver
	roles  OrganizationRoleReader
}

// OrganizationRoleReader says which organization-level role a user holds in an organization (the
// authorization evaluator implements it).
type OrganizationRoleReader interface {
	OrganizationRole(ctx context.Context, user identity.UserID, org orgscope.Org) (string, error)
}

// NewSessions returns the handlers over svc. The public origin is the origin of the
// installation (ORCON_PUBLIC_URL). The cookie names come from the same configuration
// as the authentication pipeline. The orgs and roles arguments give the organization roles that
// getCurrentUser lists for the organization menu (SPEC-019 R-UX-SHL-03).
func NewSessions(svc SessionService, cookie middleware.AuthConfig, publicOrigin string, orgs identity.OrganizationResolver, roles OrganizationRoleReader) *Sessions {
	return &Sessions{svc: svc, cookie: cookie, origin: publicOrigin, orgs: orgs, roles: roles}
}

// CreateSession signs in with an email and a password (SPEC-016 R-SH-SES-01..03).
// The request must come from the origin of the installation; a session the request
// carries is revoked and replaced.
func (s *Sessions) CreateSession(ctx context.Context, req apigen.CreateSessionRequestObject) (apigen.CreateSessionResponseObject, error) { //nolint:ireturn // the generated interface requires it
	creds, _ := middleware.CredentialsFrom(ctx)
	if s.origin == "" || creds.Origin != s.origin {
		return nil, identity.ErrCSRFCheckFailed
	}
	if req.Body == nil || req.Body.Password == nil {
		return nil, identity.ErrInvalidInput
	}
	signed, err := s.svc.SignIn(ctx, identity.SignIn{
		Email: string(req.Body.Email), Password: *req.Body.Password, PriorToken: creds.Session,
	})
	if err != nil {
		return nil, err
	}
	return createdSession{
		body:    sessionBody(signed.Session),
		token:   signed.Token,
		csrf:    signed.CSRFToken,
		maxAge:  maxAge(signed.Session),
		cookies: s.cookie,
	}, nil
}

// RevokeSession ends one live session of the signed-in user (SPEC-016 R-SH-SES-04, SPEC-004
// R-ID-08). Ending the session the request carries clears the cookies; ending another one of the
// same user leaves them. The session of another user is not found, so the answer never reveals it.
func (s *Sessions) RevokeSession(ctx context.Context, req apigen.RevokeSessionRequestObject) (apigen.RevokeSessionResponseObject, error) { //nolint:ireturn // the generated interface requires it
	principal, ok := middleware.PrincipalFrom(ctx)
	if !ok {
		return nil, identity.ErrUnauthenticated
	}
	session := identity.SessionID(req.SessionId)
	if err := s.svc.Revoke(ctx, principal, session); err != nil {
		return nil, err
	}
	if session.String() != principal.CredentialID {
		return apigen.RevokeSession204Response{}, nil
	}
	return clearedSession{cookies: s.cookie}, nil
}

// ListSessions lists the live sessions of the signed-in user, newest first, one page at a time
// (SPEC-004 R-ID-08). It reads at most identity.MaxListedSessions sessions.
func (s *Sessions) ListSessions(ctx context.Context, req apigen.ListSessionsRequestObject) (apigen.ListSessionsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	principal, ok := middleware.PrincipalFrom(ctx)
	if !ok || principal.UserID.IsZero() {
		return nil, identity.ErrUnauthenticated
	}
	limit := defaultPageSize
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	if limit < 1 || limit > maxPageSize {
		return nil, identity.ErrInvalidInput
	}
	query := sessionQuery{Limit: limit}
	var after *sessionCursor
	if req.Params.PageToken != nil {
		c, err := decodeSessionCursor(*req.Params.PageToken, query)
		if err != nil {
			return nil, err
		}
		after = &c
	}
	all, err := s.svc.ListMine(ctx, principal)
	if err != nil {
		return nil, err
	}
	var page []identity.SessionRecord
	for i := range all {
		if after != nil && !after.before(all[i]) {
			continue
		}
		page = append(page, all[i])
		if len(page) > limit {
			break
		}
	}
	body := apigen.SessionList{Items: []apigen.Session{}}
	more := len(page) > limit
	if more {
		page = page[:limit]
	}
	for i := range page {
		body.Items = append(body.Items, sessionBody(page[i]))
	}
	if more {
		token, err := encodeSessionCursor(page[len(page)-1], query)
		if err != nil {
			return nil, err
		}
		body.NextPageToken = &token
	}
	return apigen.ListSessions200JSONResponse{Body: body}, nil
}

// UpdateCurrentUser changes the display name of the signed-in user (SPEC-019 F-04). The If-Match
// ETag must be the one getCurrentUser returned; a stale one answers 412 with the current ETag.
func (s *Sessions) UpdateCurrentUser(ctx context.Context, req apigen.UpdateCurrentUserRequestObject) (apigen.UpdateCurrentUserResponseObject, error) { //nolint:ireturn // the generated interface requires it
	principal, ok := middleware.PrincipalFrom(ctx)
	if !ok {
		return nil, identity.ErrUnauthenticated
	}
	cur, err := s.svc.Current(ctx, principal)
	if err != nil {
		return nil, err
	}
	rev, parseErr := conditional.Parse(req.Params.IfMatch)
	if parseErr != nil || rev != userRevision(cur.User) {
		return nil, staleRevisionError{etag: conditional.ETag(userRevision(cur.User))}
	}
	if req.Body == nil || req.Body.DisplayName == nil {
		return nil, identity.ErrInvalidInput
	}
	if _, err := s.svc.Rename(ctx, principal, *req.Body.DisplayName, cur.User.UpdatedAt); err != nil {
		if errors.Is(err, identity.ErrConflict) {
			if fresh, getErr := s.svc.Current(ctx, principal); getErr == nil {
				return nil, staleRevisionError{etag: conditional.ETag(userRevision(fresh.User))}
			}
		}
		return nil, err
	}
	body, etag, err := s.currentUserBody(ctx, principal)
	if err != nil {
		return nil, err
	}
	return apigen.UpdateCurrentUser200JSONResponse{Body: body, Headers: apigen.UpdateCurrentUser200ResponseHeaders{ETag: &etag}}, nil
}

// userRevision is the revision of a user for optimistic concurrency: its last change, in
// microseconds (the precision the stores keep). Renaming and changing the password move it.
func userRevision(u identity.UserRecord) int64 { return u.UpdatedAt.UnixMicro() }

// GetCurrentUser returns the user of the session the request carries (SPEC-016
// R-SH-SES-05). It never returns the password hash or the token.
func (s *Sessions) GetCurrentUser(ctx context.Context, _ apigen.GetCurrentUserRequestObject) (apigen.GetCurrentUserResponseObject, error) { //nolint:ireturn // the generated interface requires it
	principal, ok := middleware.PrincipalFrom(ctx)
	if !ok {
		return nil, identity.ErrUnauthenticated
	}
	body, etag, err := s.currentUserBody(ctx, principal)
	if err != nil {
		return nil, err
	}
	return apigen.GetCurrentUser200JSONResponse{Body: body, Headers: apigen.GetCurrentUser200ResponseHeaders{ETag: &etag}}, nil
}

// currentUserBody reads the user and the session of the principal and returns the CurrentUser
// body with the ETag of the user.
func (s *Sessions) currentUserBody(ctx context.Context, principal identity.Principal) (apigen.CurrentUser, string, error) {
	current, err := s.svc.Current(ctx, principal)
	if err != nil {
		return apigen.CurrentUser{}, "", err
	}
	uid := types.UUID(current.User.ID)
	roles, err := s.orgRoles(ctx, current.User.ID)
	if err != nil {
		return apigen.CurrentUser{}, "", err
	}
	return apigen.CurrentUser{
		Id:             &uid,
		Email:          types.Email(current.User.EmailNormalized),
		DisplayName:    current.User.DisplayName,
		CurrentSession: sessionBody(current.Session),
		OrgRoles:       &roles,
	}, conditional.ETag(userRevision(current.User)), nil
}

// orgRoles lists the organization-level role of the user in each organization it belongs to; an
// organization where the user holds no such role is left out.
func (s *Sessions) orgRoles(ctx context.Context, user identity.UserID) ([]apigen.OrgRole, error) {
	choices, err := s.orgs.ResolveUserOrganizations(ctx, user, identity.MaxResolvedOrganizations)
	if err != nil {
		return nil, err
	}
	out := []apigen.OrgRole{}
	for i := range choices {
		role, err := s.roles.OrganizationRole(ctx, user, choices[i].Scope)
		if err != nil {
			return nil, err
		}
		if role == "" {
			continue
		}
		out = append(out, apigen.OrgRole{OrgId: types.UUID(choices[i].ID), Role: role})
	}
	return out, nil
}

// createdSession is the 201 answer of createSession: the session body and the two
// cookies, the session cookie HttpOnly and the CSRF cookie readable by the page.
type createdSession struct {
	body    apigen.Session
	token   string
	csrf    string
	maxAge  int
	cookies middleware.AuthConfig
}

// VisitCreateSessionResponse writes the cookies and the body.
func (c createdSession) VisitCreateSessionResponse(w http.ResponseWriter) error {
	http.SetCookie(w, &http.Cookie{
		Name: c.cookies.SessionCookie, Value: c.token, Path: "/", MaxAge: c.maxAge,
		Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	// The CSRF cookie is readable by the page on purpose: the page echoes it in a header (SPEC-016 R-SH-SES-01).
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // HttpOnly is off on purpose, see above
		Name: c.cookies.CSRFCookie, Value: c.csrf, Path: "/", MaxAge: c.maxAge,
		Secure: true, SameSite: http.SameSiteStrictMode,
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	return json.NewEncoder(w).Encode(c.body)
}

// clearedSession is the 204 answer of revokeSession: both cookies expire now.
type clearedSession struct {
	cookies middleware.AuthConfig
}

// VisitRevokeSessionResponse expires both cookies.
func (c clearedSession) VisitRevokeSessionResponse(w http.ResponseWriter) error {
	http.SetCookie(w, &http.Cookie{
		Name: c.cookies.SessionCookie, Value: "", Path: "/", MaxAge: -1,
		Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // clears the readable CSRF cookie, see above
		Name: c.cookies.CSRFCookie, Value: "", Path: "/", MaxAge: -1,
		Secure: true, SameSite: http.SameSiteStrictMode,
	})
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// sessionBody maps a session to the Session schema of the contract.
func sessionBody(rec identity.SessionRecord) apigen.Session {
	sid := types.UUID(rec.ID)
	return apigen.Session{
		Id:             &sid,
		AssuranceLevel: apigen.SessionAssuranceLevel(rec.AssuranceLevel),
		CreatedAt:      rec.CreatedAt,
		ExpiresAt:      rec.ExpiresAt,
		IdleExpiresAt:  rec.IdleExpiresAt,
	}
}

// maxAge is the lifetime of the cookies: the maximum age of the session, counted from
// its start. A cookie outliving the session is harmless; the server checks both.
func maxAge(rec identity.SessionRecord) int {
	return int(rec.ExpiresAt.Sub(rec.CreatedAt) / time.Second)
}

// sessionQuery is what a page token is bound to: a token issued for another page size is refused.
type sessionQuery struct {
	Limit int `json:"l"`
}

// sessionCursor is the position of the last session of a page: (start instant, id).
type sessionCursor struct {
	Created int64        `json:"c"`
	ID      string       `json:"i"`
	Query   sessionQuery `json:"q"`
}

// before reports whether rec comes after the cursor in the newest-first (start, id) order.
func (c sessionCursor) before(rec identity.SessionRecord) bool {
	created := rec.CreatedAt.UnixMicro()
	if created != c.Created {
		return created < c.Created
	}
	return rec.ID.String() < c.ID
}

// encodeSessionCursor returns the opaque token of the position after rec.
func encodeSessionCursor(rec identity.SessionRecord, q sessionQuery) (string, error) {
	raw, err := json.Marshal(sessionCursor{Created: rec.CreatedAt.UnixMicro(), ID: rec.ID.String(), Query: q})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// decodeSessionCursor reads a page token. A token that does not decode, or that was issued for
// another page size, is errInvalidPageToken.
func decodeSessionCursor(token string, q sessionQuery) (sessionCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return sessionCursor{}, errInvalidPageToken
	}
	var c sessionCursor
	if err := json.Unmarshal(raw, &c); err != nil || c.Query != q || c.ID == "" {
		return sessionCursor{}, errInvalidPageToken
	}
	return c, nil
}
