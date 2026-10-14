// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/oapi-codegen/runtime/types"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/identity"
)

// SessionService is the part of the session service that the operations of SPEC-016
// section 3.2 need. The identity session service implements it.
type SessionService interface {
	SignIn(ctx context.Context, in identity.SignIn) (identity.SignedIn, error)
	Current(ctx context.Context, principal identity.Principal) (identity.CurrentUser, error)
	Revoke(ctx context.Context, user identity.UserID, session identity.SessionID) error
}

// Sessions serves createSession, revokeSession and getCurrentUser (SPEC-016
// section 3.2). It is not routed yet. Until SPEC-004 SE8 (the audit of sign-in and
// sign-out) is on main, the two state-changing operations keep answering 503 from
// Unavailable (SPEC-016 rule C-6). The getCurrentUser operation joins the routes with them.
type Sessions struct {
	svc    SessionService
	cookie middleware.AuthConfig
	origin string
}

// NewSessions returns the handlers over svc. The public origin is the origin of the
// installation (ORCON_PUBLIC_URL). The cookie names come from the same configuration
// as the authentication pipeline.
func NewSessions(svc SessionService, cookie middleware.AuthConfig, publicOrigin string) *Sessions {
	return &Sessions{svc: svc, cookie: cookie, origin: publicOrigin}
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

// RevokeSession ends the session the request carries (SPEC-016 R-SH-SES-04). Any
// other session id is not found, so the answer never reveals another session.
func (s *Sessions) RevokeSession(ctx context.Context, req apigen.RevokeSessionRequestObject) (apigen.RevokeSessionResponseObject, error) { //nolint:ireturn // the generated interface requires it
	principal, ok := middleware.PrincipalFrom(ctx)
	if !ok {
		return nil, identity.ErrUnauthenticated
	}
	session := identity.SessionID(req.SessionId)
	if session.String() != principal.CredentialID {
		return nil, identity.ErrNotFound
	}
	if err := s.svc.Revoke(ctx, principal.UserID, session); err != nil {
		return nil, err
	}
	return clearedSession{cookies: s.cookie}, nil
}

// GetCurrentUser returns the user of the session the request carries (SPEC-016
// R-SH-SES-05). It never returns the password hash or the token.
func (s *Sessions) GetCurrentUser(ctx context.Context, _ apigen.GetCurrentUserRequestObject) (apigen.GetCurrentUserResponseObject, error) { //nolint:ireturn // the generated interface requires it
	principal, ok := middleware.PrincipalFrom(ctx)
	if !ok {
		return nil, identity.ErrUnauthenticated
	}
	current, err := s.svc.Current(ctx, principal)
	if err != nil {
		return nil, err
	}
	uid := types.UUID(current.User.ID)
	return apigen.GetCurrentUser200JSONResponse{Body: apigen.CurrentUser{
		Id:             &uid,
		Email:          types.Email(current.User.EmailNormalized),
		DisplayName:    current.User.DisplayName,
		CurrentSession: sessionBody(current.Session),
	}}, nil
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
