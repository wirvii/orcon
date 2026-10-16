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
	"github.com/wirvii/orcon/internal/adapter/httpapi/conditional"
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
	// listed is what ListMine returns; renamed records the name of the last Rename.
	listed  []identity.SessionRecord
	renamed string
	// renameErr is what Rename fails with, apart from err.
	renameErr error
	// expected is the update instant that Rename was given.
	expected time.Time
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

func (f *fakeSessions) ListMine(context.Context, identity.Principal) ([]identity.SessionRecord, error) {
	return f.listed, f.err
}

func (f *fakeSessions) Rename(_ context.Context, _ identity.Principal, name string, expected time.Time) (identity.UserRecord, error) {
	f.renamed, f.expected = name, expected
	if f.renameErr != nil {
		return identity.UserRecord{}, f.renameErr
	}
	f.current.User.DisplayName = name
	f.current.User.UpdatedAt = expected.Add(time.Second)
	return f.current.User, f.err
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

// TestRevokeOtherSessionKeepsTheCookies (SPEC-004 R-ID-08): ending another session of the user
// is a plain 204 that leaves the cookies of the request alone; the service decides ownership, so
// the session of another user is its not-found.
func TestRevokeOtherSessionKeepsTheCookies(t *testing.T) {
	current := sampleSession()
	other := sampleSession()
	svc := &fakeSessions{}
	principal := identity.Principal{ID: identity.PrincipalID(testIDs.New()), Kind: identity.PrincipalUser, UserID: current.UserID, CredentialType: identity.CredentialSession, CredentialID: current.ID.String()}
	ctx := middleware.WithPrincipal(t.Context(), principal)

	resp, err := newTestSessions(svc).RevokeSession(ctx, apigen.RevokeSessionRequestObject{SessionId: types.UUID(other.ID)})
	if err != nil {
		t.Fatalf("RevokeSession: %v", err)
	}
	out := httptest.NewRecorder()
	if err := resp.VisitRevokeSessionResponse(out); err != nil {
		t.Fatalf("write: %v", err)
	}
	if out.Code != http.StatusNoContent || len(svc.revoked) != 1 || svc.revoked[0] != other.ID {
		t.Fatalf("status %d, revoked %v; want 204 and the other session", out.Code, svc.revoked)
	}
	if len(out.Result().Cookies()) != 0 {
		t.Errorf("cookies = %v, want none touched", out.Result().Cookies())
	}

	svc = &fakeSessions{err: identity.ErrNotFound}
	if _, err := newTestSessions(svc).RevokeSession(ctx, apigen.RevokeSessionRequestObject{SessionId: types.UUID(other.ID)}); !errors.Is(err, identity.ErrNotFound) {
		t.Errorf("another user's session: err = %v, want ErrNotFound", err)
	}
	if _, err := newTestSessions(svc).RevokeSession(t.Context(), apigen.RevokeSessionRequestObject{}); !errors.Is(err, identity.ErrUnauthenticated) {
		t.Errorf("no session: err = %v, want ErrUnauthenticated", err)
	}
}

// sessionsAt returns n sessions of one user, newest first, one minute apart.
func sessionsAt(n int) []identity.SessionRecord {
	user := identity.UserID(testIDs.New())
	out := make([]identity.SessionRecord, n)
	for i := range out {
		rec := sampleSession()
		rec.UserID = user
		rec.CreatedAt = rec.CreatedAt.Add(-time.Duration(i) * time.Minute)
		out[i] = rec
	}
	return out
}

func userContext(t *testing.T, user identity.UserID) context.Context {
	t.Helper()
	principal := identity.Principal{ID: identity.PrincipalID(testIDs.New()), Kind: identity.PrincipalUser, UserID: user, CredentialType: identity.CredentialSession, CredentialID: testIDs.New().String()}
	return middleware.WithPrincipal(t.Context(), principal)
}

// sessionPage returns the body of a listSessions answer, failing the test on any other answer.
func sessionPage(t *testing.T, resp apigen.ListSessionsResponseObject) apigen.SessionList {
	t.Helper()
	ok, is := resp.(apigen.ListSessions200JSONResponse)
	if !is {
		t.Fatalf("answer = %T, want the 200 page", resp)
	}
	return ok.Body
}

// TestListSessionsPagesNewestFirst (SPEC-004 R-ID-08): the sessions come in the order the
// service gave them, one page at a time, and the token of a page leads to the rest; a token of
// another page size, a bad limit and a missing session are refused. The body carries no token.
func TestListSessionsPagesNewestFirst(t *testing.T) {
	all := sessionsAt(5)
	svc := &fakeSessions{listed: all}
	h := newTestSessions(svc)
	ctx := userContext(t, all[0].UserID)
	limit := 2

	var seen []types.UUID
	var token *string
	for pages := range 5 {
		resp, err := h.ListSessions(ctx, apigen.ListSessionsRequestObject{Params: apigen.ListSessionsParams{Limit: &limit, PageToken: token}})
		if err != nil {
			t.Fatalf("page %d: %v", pages, err)
		}
		body := sessionPage(t, resp)
		for i := range body.Items {
			seen = append(seen, *body.Items[i].Id)
		}
		if body.NextPageToken == nil {
			break
		}
		token = body.NextPageToken
	}
	if len(seen) != len(all) {
		t.Fatalf("saw %d sessions, want %d", len(seen), len(all))
	}
	for i := range all {
		if seen[i] != types.UUID(all[i].ID) {
			t.Errorf("session %d = %s, want %s", i, seen[i], all[i].ID)
		}
	}

	other := 3
	first, err := h.ListSessions(ctx, apigen.ListSessionsRequestObject{Params: apigen.ListSessionsParams{Limit: &limit}})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	next := sessionPage(t, first).NextPageToken
	if _, err := h.ListSessions(ctx, apigen.ListSessionsRequestObject{Params: apigen.ListSessionsParams{Limit: &other, PageToken: next}}); !errors.Is(err, errInvalidPageToken) {
		t.Errorf("token of another page size: err = %v, want invalid page token", err)
	}
	garbage := "not-a-token"
	if _, err := h.ListSessions(ctx, apigen.ListSessionsRequestObject{Params: apigen.ListSessionsParams{PageToken: &garbage}}); !errors.Is(err, errInvalidPageToken) {
		t.Errorf("garbage token: err = %v, want invalid page token", err)
	}
	zero := 0
	if _, err := h.ListSessions(ctx, apigen.ListSessionsRequestObject{Params: apigen.ListSessionsParams{Limit: &zero}}); !errors.Is(err, identity.ErrInvalidInput) {
		t.Errorf("limit 0: err = %v, want ErrInvalidInput", err)
	}
	if _, err := h.ListSessions(t.Context(), apigen.ListSessionsRequestObject{}); !errors.Is(err, identity.ErrUnauthenticated) {
		t.Errorf("no session: err = %v, want ErrUnauthenticated", err)
	}
	out := httptest.NewRecorder()
	if err := first.VisitListSessionsResponse(out); err != nil {
		t.Fatalf("write: %v", err)
	}
	if strings.Contains(out.Body.String(), "tokenHash") || !strings.Contains(out.Body.String(), `"assuranceLevel":"AAL1"`) {
		t.Errorf("body = %s, want the sessions and no token", out.Body.String())
	}
}

// TestUpdateCurrentUser (SPEC-019 F-04): the name changes when If-Match is the ETag of the user
// that getCurrentUser returned; a stale ETag answers with the current one, and nothing changes;
// a missing name is invalid; a conflict between the check and the write is a stale ETag too.
func TestUpdateCurrentUser(t *testing.T) {
	user := identity.UserRecord{
		ID: identity.UserID(testIDs.New()), EmailNormalized: "ada@example.test", DisplayName: "Owner", Status: identity.UserActive,
		UpdatedAt: time.Date(2026, 10, 14, 15, 4, 5, 0, time.UTC),
	}
	newSvc := func() *fakeSessions {
		return &fakeSessions{current: identity.CurrentUser{User: user, Session: sampleSession()}}
	}
	ctx := userContext(t, user.ID)
	name := "Ada Lovelace"
	etag := conditional.ETag(user.UpdatedAt.UnixMicro())

	svc := newSvc()
	resp, err := newTestSessions(svc).UpdateCurrentUser(ctx, apigen.UpdateCurrentUserRequestObject{
		Params: apigen.UpdateCurrentUserParams{IfMatch: etag}, Body: &apigen.UpdateCurrentUserRequest{DisplayName: &name},
	})
	if err != nil {
		t.Fatalf("UpdateCurrentUser: %v", err)
	}
	out := httptest.NewRecorder()
	if err := resp.VisitUpdateCurrentUserResponse(out); err != nil {
		t.Fatalf("write: %v", err)
	}
	if svc.renamed != name || !svc.expected.Equal(user.UpdatedAt) {
		t.Errorf("rename got %q at %v, want %q at %v", svc.renamed, svc.expected, name, user.UpdatedAt)
	}
	if !strings.Contains(out.Body.String(), `"displayName":"Ada Lovelace"`) || out.Header().Get("ETag") == "" || out.Header().Get("ETag") == etag {
		t.Errorf("body %s, ETag %q; want the new name and a new ETag", out.Body.String(), out.Header().Get("ETag"))
	}

	svc = newSvc()
	_, err = newTestSessions(svc).UpdateCurrentUser(ctx, apigen.UpdateCurrentUserRequestObject{
		Params: apigen.UpdateCurrentUserParams{IfMatch: `"1"`}, Body: &apigen.UpdateCurrentUserRequest{DisplayName: &name},
	})
	var stale staleRevisionError
	if !errors.As(err, &stale) || stale.etag != etag || svc.renamed != "" {
		t.Errorf("stale If-Match: err = %v, renamed %q; want a stale revision carrying %s and no rename", err, svc.renamed, etag)
	}
	for _, bad := range []string{"*", "W/\"1\"", "abc"} {
		_, err = newTestSessions(newSvc()).UpdateCurrentUser(ctx, apigen.UpdateCurrentUserRequestObject{
			Params: apigen.UpdateCurrentUserParams{IfMatch: bad}, Body: &apigen.UpdateCurrentUserRequest{DisplayName: &name},
		})
		if !errors.As(err, &stale) {
			t.Errorf("If-Match %q: err = %v, want a stale revision", bad, err)
		}
	}

	for name, body := range map[string]*apigen.UpdateCurrentUserRequest{"no body": nil, "no name": {}} {
		_, err = newTestSessions(newSvc()).UpdateCurrentUser(ctx, apigen.UpdateCurrentUserRequestObject{Params: apigen.UpdateCurrentUserParams{IfMatch: etag}, Body: body})
		if !errors.Is(err, identity.ErrInvalidInput) {
			t.Errorf("%s: err = %v, want ErrInvalidInput", name, err)
		}
	}

	svc = newSvc()
	svc.renameErr = identity.ErrConflict
	_, err = newTestSessions(svc).UpdateCurrentUser(ctx, apigen.UpdateCurrentUserRequestObject{
		Params: apigen.UpdateCurrentUserParams{IfMatch: etag}, Body: &apigen.UpdateCurrentUserRequest{DisplayName: &name},
	})
	if !errors.As(err, &stale) {
		t.Errorf("conflict at write: err = %v, want a stale revision", err)
	}
	svc = newSvc()
	svc.renameErr = identity.ErrInvalidInput
	_, err = newTestSessions(svc).UpdateCurrentUser(ctx, apigen.UpdateCurrentUserRequestObject{
		Params: apigen.UpdateCurrentUserParams{IfMatch: etag}, Body: &apigen.UpdateCurrentUserRequest{DisplayName: &name},
	})
	if !errors.Is(err, identity.ErrInvalidInput) {
		t.Errorf("invalid name: err = %v, want ErrInvalidInput", err)
	}
	if _, err := newTestSessions(newSvc()).UpdateCurrentUser(t.Context(), apigen.UpdateCurrentUserRequestObject{}); !errors.Is(err, identity.ErrUnauthenticated) {
		t.Errorf("no session: err = %v, want ErrUnauthenticated", err)
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
