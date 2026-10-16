// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/oapi-codegen/runtime/types"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

const (
	testOrigin   = "https://orcon.test"
	testPassword = "correct horse battery staple"
)

// fakeSessions is a session service that records its inputs and answers what the test sets.
type fakeSessions struct {
	in      identity.SignIn
	signed  identity.SignedIn
	err     error
	revoked []identity.SessionID
	current identity.CurrentUser
}

func (f *fakeSessions) SignIn(_ context.Context, in identity.SignIn) (identity.SignedIn, error) {
	f.in = in
	return f.signed, f.err
}

func (f *fakeSessions) Current(context.Context, identity.Principal) (identity.CurrentUser, error) {
	return f.current, f.err
}

func (f *fakeSessions) Revoke(_ context.Context, _ identity.Principal, session identity.SessionID) error {
	f.revoked = append(f.revoked, session)
	return f.err
}

// testIDs hands out distinct identities to the tests of this file.
var testIDs = id.NewSequence()

// sampleSession is a session that started at a fixed instant and lasts seven days.
func sampleSession() identity.SessionRecord {
	start := time.Date(2026, 10, 14, 15, 4, 5, 0, time.UTC)
	return identity.SessionRecord{
		ID:             identity.SessionID(testIDs.New()),
		UserID:         identity.UserID(testIDs.New()),
		AssuranceLevel: identity.AssuranceAAL1,
		CreatedAt:      start,
		ExpiresAt:      start.Add(7 * 24 * time.Hour),
		IdleExpiresAt:  start.Add(8 * time.Hour),
		LastSeenAt:     start,
	}
}

// signInRequest is a createSession request from the installation's origin.
func signInRequest(t *testing.T, origin, prior string) context.Context {
	t.Helper()
	return middleware.WithCredentials(t.Context(), identity.Credentials{Origin: origin, Session: prior})
}

func newTestSessions(svc SessionService) *Sessions {
	return NewSessions(svc, middleware.DefaultAuthConfig(), testOrigin, &fakeResolver{}, noOrgRoles{})
}

// noOrgRoles is an organization-role reader of a user with no organization role.
type noOrgRoles struct{}

// OrganizationRole returns no role.
func (noOrgRoles) OrganizationRole(context.Context, identity.UserID, orgscope.Org) (string, error) {
	return "", nil
}

// cookieOf returns the Set-Cookie header that names cookie, or nil.
func cookieOf(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// TestCreateSessionSetsSessionAndCsrfCookies (SPEC-016 R-SH-SES-01): the session
// cookie is HttpOnly and the CSRF cookie is readable; neither body nor header shows the token.
func TestCreateSessionSetsSessionAndCsrfCookies(t *testing.T) {
	rec := sampleSession()
	svc := &fakeSessions{signed: identity.SignedIn{Token: "token-value", CSRFToken: "csrf-value", Session: rec}} //nolint:gosec // a fixture token, not a credential
	req := apigen.CreateSessionRequestObject{Body: &apigen.SessionCredentials{Email: types.Email("ada@example.test"), Password: strPtr(testPassword)}}

	resp, err := newTestSessions(svc).CreateSession(signInRequest(t, testOrigin, ""), req)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	out := httptest.NewRecorder()
	if err := resp.VisitCreateSessionResponse(out); err != nil {
		t.Fatalf("write: %v", err)
	}
	if out.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", out.Code)
	}
	session := cookieOf(out, "__Host-orcon_session")
	if session == nil || session.Value != "token-value" || !session.HttpOnly || !session.Secure || session.Path != "/" || session.SameSite != http.SameSiteLaxMode {
		t.Errorf("session cookie = %+v, want HttpOnly, Secure, SameSite=Lax, Path=/", session)
	}
	csrf := cookieOf(out, "__Host-orcon_csrf")
	if csrf == nil || csrf.Value != "csrf-value" || csrf.HttpOnly || !csrf.Secure || csrf.Path != "/" || csrf.SameSite != http.SameSiteStrictMode {
		t.Errorf("csrf cookie = %+v, want readable, Secure, SameSite=Strict, Path=/", csrf)
	}
	if strings.Contains(out.Body.String(), "token-value") {
		t.Error("the session token is in the response body")
	}
	if !strings.Contains(out.Body.String(), `"assuranceLevel":"AAL1"`) {
		t.Errorf("body = %s, want the session fields", out.Body.String())
	}
}

// TestCreateSessionChecksOrigin (SPEC-016 R-SH-SES-03): a sign-in from another
// origin, or with no origin, is refused before the password is looked at.
func TestCreateSessionChecksOrigin(t *testing.T) {
	svc := &fakeSessions{}
	req := apigen.CreateSessionRequestObject{Body: &apigen.SessionCredentials{Email: "ada@example.test", Password: strPtr(testPassword)}}
	for _, origin := range []string{"", "https://evil.test"} {
		_, err := newTestSessions(svc).CreateSession(signInRequest(t, origin, ""), req)
		if !errors.Is(err, identity.ErrCSRFCheckFailed) {
			t.Errorf("origin %q: err = %v, want ErrCSRFCheckFailed", origin, err)
		}
	}
	if svc.in.Email != "" {
		t.Error("the service was asked to sign in a request from another origin")
	}
}

// TestCreateSessionRotatesPreviousSession (SPEC-016 R-SH-SES-01): the session the
// request already carries is handed to the service to be revoked.
func TestCreateSessionRotatesPreviousSession(t *testing.T) {
	svc := &fakeSessions{signed: identity.SignedIn{Token: "new", Session: sampleSession()}}
	req := apigen.CreateSessionRequestObject{Body: &apigen.SessionCredentials{Email: "ada@example.test", Password: strPtr(testPassword)}}
	if _, err := newTestSessions(svc).CreateSession(signInRequest(t, testOrigin, "previous-token"), req); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if svc.in.PriorToken != "previous-token" {
		t.Errorf("PriorToken = %q, want the carried session", svc.in.PriorToken)
	}
}

// TestCreateSessionRefusesAlikeAndThrottles: a refusal is 401 for every reason, and
// a throttled identifier is 429 with the wait (SPEC-004 R-AUTH-02).
func TestCreateSessionRefusesAlikeAndThrottles(t *testing.T) {
	cases := map[string]struct {
		err    error
		status int
	}{
		"wrong password":  {&identity.SignInError{Reason: identity.ReasonWrongPassword}, http.StatusUnauthorized},
		"unknown account": {&identity.SignInError{Reason: identity.ReasonUnknownAccount}, http.StatusUnauthorized},
		"throttled":       {&identity.SignInError{Reason: identity.ReasonThrottled, RetryAfter: 30 * time.Second}, http.StatusTooManyRequests},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			out := serveError(t, c.err)
			if out.Code != c.status {
				t.Errorf("status = %d, want %d", out.Code, c.status)
			}
			if c.status == http.StatusTooManyRequests && out.Header().Get("Retry-After") != "30" {
				t.Errorf("Retry-After = %q, want 30", out.Header().Get("Retry-After"))
			}
			if strings.Contains(out.Body.String(), "WRONG_PASSWORD") || strings.Contains(out.Body.String(), "UNKNOWN_ACCOUNT") {
				t.Error("the reason of a refusal reaches the client")
			}
		})
	}
}

// TestRevokeCurrentSessionSignsOut (SPEC-016 R-SH-SES-04): the current session is
// revoked and both cookies expire.
func TestRevokeCurrentSessionSignsOut(t *testing.T) {
	rec := sampleSession()
	svc := &fakeSessions{}
	principal := identity.Principal{ID: identity.PrincipalID(testIDs.New()), Kind: identity.PrincipalUser, UserID: rec.UserID, CredentialType: identity.CredentialSession, CredentialID: rec.ID.String()}
	ctx := middleware.WithPrincipal(t.Context(), principal)
	req := apigen.RevokeSessionRequestObject{SessionId: types.UUID(rec.ID)}

	resp, err := newTestSessions(svc).RevokeSession(ctx, req)
	if err != nil {
		t.Fatalf("RevokeSession: %v", err)
	}
	out := httptest.NewRecorder()
	if err := resp.VisitRevokeSessionResponse(out); err != nil {
		t.Fatalf("write: %v", err)
	}
	if out.Code != http.StatusNoContent || len(svc.revoked) != 1 || svc.revoked[0] != rec.ID {
		t.Fatalf("status %d, revoked %v; want 204 and the current session", out.Code, svc.revoked)
	}
	if c := cookieOf(out, "__Host-orcon_session"); c == nil || c.MaxAge >= 0 {
		t.Errorf("session cookie not expired: %+v", c)
	}
}

// TestRevokeOtherSessionIsNotFound (SPEC-016 R-SH-SES-04): another session id is
// not found, and the service is never asked to revoke it.
func TestRevokeOtherSessionIsNotFound(t *testing.T) {
	current := sampleSession()
	other := sampleSession()
	svc := &fakeSessions{}
	principal := identity.Principal{ID: identity.PrincipalID(testIDs.New()), Kind: identity.PrincipalUser, UserID: current.UserID, CredentialType: identity.CredentialSession, CredentialID: current.ID.String()}
	ctx := middleware.WithPrincipal(t.Context(), principal)

	_, err := newTestSessions(svc).RevokeSession(ctx, apigen.RevokeSessionRequestObject{SessionId: types.UUID(other.ID)})
	if !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if len(svc.revoked) != 0 {
		t.Error("another session was revoked")
	}
}

// TestGetCurrentUserRequiresSession (SPEC-016 R-SH-SES-05): without a session the
// operation is unauthenticated; with one it returns the user and no secret.
func TestGetCurrentUserRequiresSession(t *testing.T) {
	svc := &fakeSessions{current: identity.CurrentUser{
		User:    identity.UserRecord{ID: identity.UserID(testIDs.New()), EmailNormalized: "ada@example.test", DisplayName: "Ada", Status: identity.UserActive},
		Session: sampleSession(),
	}}
	if _, err := newTestSessions(svc).GetCurrentUser(t.Context(), apigen.GetCurrentUserRequestObject{}); !errors.Is(err, identity.ErrUnauthenticated) {
		t.Errorf("no session: err = %v, want ErrUnauthenticated", err)
	}
	principal := identity.Principal{ID: identity.PrincipalID(testIDs.New()), Kind: identity.PrincipalUser, CredentialType: identity.CredentialSession}
	resp, err := newTestSessions(svc).GetCurrentUser(middleware.WithPrincipal(t.Context(), principal), apigen.GetCurrentUserRequestObject{})
	if err != nil {
		t.Fatalf("GetCurrentUser: %v", err)
	}
	out := httptest.NewRecorder()
	if err := resp.VisitGetCurrentUserResponse(out); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !strings.Contains(out.Body.String(), `"email":"ada@example.test"`) || strings.Contains(out.Body.String(), "password") {
		t.Errorf("body = %s, want the user and no password", out.Body.String())
	}
}

// serveError runs a refused sign-in through the error table and returns the answer.
func serveError(t *testing.T, err error) *httptest.ResponseRecorder {
	t.Helper()
	out := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/users/me/sessions", nil)
	errorHandler{logger: slog.New(slog.DiscardHandler)}.response(out, req, err)
	return out
}

func strPtr(s string) *string { return &s }
