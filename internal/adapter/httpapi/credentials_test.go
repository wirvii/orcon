// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/credential"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// fakeCredentials is an in-memory CredentialService for the handler tests: it keeps summaries by id and remembers
// what the handler asked for. Secret values reach it in the input and never leave it in a summary.
type fakeCredentials struct {
	byID     map[id.UUID]credential.Summary
	list     []credential.Summary
	lastSet  credential.Settings
	lastIn   credential.Input
	unshared []identity.ProjectID
	err      error
	// writeErr is what Update and Delete answer, after Get found the credential.
	writeErr error
	// lastExpected is the revision the last Update or Delete carried to the write.
	lastExpected time.Time
}

func (f *fakeCredentials) Create(_ context.Context, _ orgscope.Org, project identity.ProjectID, in credential.Input, _ audit.Actor) (credential.Summary, error) {
	if f.err != nil {
		return credential.Summary{}, f.err
	}
	f.lastIn = in
	s := credential.Summary{Record: credential.Record{
		ID: newTestID(1), ProjectID: project, Name: in.Name, ConnectorReference: in.ConnectorReference,
		Status: credential.StatusOK, AllowedHosts: in.AllowedHosts, CurrentVersion: 1,
		CreatedAt: testInstant, UpdatedAt: testInstant, SecretUpdatedAt: testInstant,
	}, Owned: true, Secret: credential.SecretState{Set: true, UpdatedAt: testInstant}, Public: in.Public}
	return s, nil
}

func (f *fakeCredentials) Get(_ context.Context, _ orgscope.Org, _ identity.ProjectID, cred id.UUID) (credential.Summary, error) {
	if f.err != nil {
		return credential.Summary{}, f.err
	}
	s, ok := f.byID[cred]
	if !ok {
		return credential.Summary{}, credential.ErrNotFound
	}
	return s, nil
}

func (f *fakeCredentials) List(context.Context, orgscope.Org, identity.ProjectID) ([]credential.Summary, error) {
	return append([]credential.Summary(nil), f.list...), f.err
}

func (f *fakeCredentials) Replace(ctx context.Context, _ orgscope.Org, _ identity.ProjectID, cred id.UUID, in credential.Input, _ audit.Actor) (credential.Summary, error) {
	f.lastIn = in
	return f.Get(ctx, orgscope.Org{}, identity.ProjectID{}, cred)
}

func (f *fakeCredentials) Update(ctx context.Context, _ orgscope.Org, _ identity.ProjectID, cred id.UUID, set credential.Settings, expected time.Time, _ audit.Actor) (credential.Summary, error) {
	f.lastSet, f.lastExpected = set, expected
	if f.writeErr != nil {
		return credential.Summary{}, f.writeErr
	}
	return f.Get(ctx, orgscope.Org{}, identity.ProjectID{}, cred)
}

func (f *fakeCredentials) Delete(_ context.Context, _ orgscope.Org, _ identity.ProjectID, _ id.UUID, expected time.Time, _ audit.Actor) error {
	f.lastExpected = expected
	if f.writeErr != nil {
		return f.writeErr
	}
	return f.err
}

func (f *fakeCredentials) Share(context.Context, orgscope.Org, identity.ProjectID, id.UUID, identity.ProjectID, audit.Actor) error {
	return f.err
}

func (f *fakeCredentials) Unshare(_ context.Context, _ orgscope.Org, _ identity.ProjectID, _ id.UUID, target identity.ProjectID, _ audit.Actor) error {
	f.unshared = append(f.unshared, target)
	return f.err
}

func (f *fakeCredentials) OwnerProject(_ context.Context, _ orgscope.Org, cred string) (identity.ProjectID, error) {
	u, err := id.Parse(cred)
	if err != nil {
		return identity.ProjectID{}, identity.ErrNotFound
	}
	s, ok := f.byID[u]
	if !ok {
		return identity.ProjectID{}, identity.ErrNotFound
	}
	return s.ProjectID, nil
}

var testInstant = time.Date(2026, 10, 12, 15, 4, 5, 0, time.UTC)

// newTestID returns a fixed, valid identifier for the n-th fixture.
func newTestID(n byte) id.UUID {
	var u id.UUID
	u[0], u[15] = 0x01, n
	u[6], u[8] = 0x70, 0x80
	return u
}

// callerCtx returns a context carrying a signed-in user of organization orgID.
func callerCtx(t *testing.T) context.Context {
	t.Helper()
	org := newTestID(9)
	return middleware.WithPrincipal(t.Context(), identity.Principal{
		ID: identity.PrincipalID(newTestID(8)), Kind: identity.PrincipalUser, UserID: identity.UserID(newTestID(7)),
		OrgID: identity.OrgID(org), CredentialType: identity.CredentialSession,
	})
}

// TestCreateCredentialNeverReturnsSecrets (SPEC-004 R-CRED-01): the created credential goes out without the secret
// value, and the response names its ETag and its location.
func TestCreateCredentialNeverReturnsSecrets(t *testing.T) {
	const secret = "probe-secret-value-4c1d"
	svc := &fakeCredentials{byID: map[id.UUID]credential.Summary{}}
	h := NewCredentials(svc)
	project := apigen.ProjectId(newTestID(2))
	resp, err := h.CreateCredential(callerCtx(t), apigen.CreateCredentialRequestObject{
		ProjectId: project,
		Body: &apigen.CreateCredentialRequest{
			Name: "Slack", Type: "orcon.http@1", SecretFields: map[string]string{"token": secret},
			PublicFields: &map[string]string{"header": "Authorization"},
		},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	created, ok := resp.(apigen.CreateCredential201JSONResponse)
	if !ok {
		t.Fatalf("response is %T, want 201", resp)
	}
	raw, err := json.Marshal(created.Body)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), secret) {
		t.Errorf("the response carries the secret value: %s", raw)
	}
	if created.Headers.ETag == nil || created.Headers.Location == nil {
		t.Errorf("the response lacks ETag or Location: %+v", created.Headers)
	}
	if svc.lastIn.Secret["token"] != secret {
		t.Errorf("the secret did not reach the service")
	}
}

// TestUpdateCredentialRefusesStaleRevision (IDEM-5): a change based on an old revision is refused with the current
// ETag, and a change based on the current revision keeps the fields the request leaves out.
func TestUpdateCredentialRefusesStaleRevision(t *testing.T) {
	cur := credential.Summary{Record: credential.Record{
		ID: newTestID(3), ProjectID: identity.ProjectID(newTestID(2)), Name: "Slack", AllowedHosts: []string{"hooks.slack.com"},
		Status: credential.StatusOK, CurrentVersion: 2, UpdatedAt: testInstant,
	}, Owned: true}
	svc := &fakeCredentials{byID: map[id.UUID]credential.Summary{cur.ID: cur}}
	h := NewCredentials(svc)
	ctx := callerCtx(t)
	stale := apigen.UpdateCredentialRequestObject{
		CredentialId: apigen.CredentialId(newTestID(3)),
		Params:       apigen.UpdateCredentialParams{IfMatch: `"1"`},
		Body:         &apigen.UpdateCredentialRequest{},
	}
	_, err := h.UpdateCredential(ctx, stale)
	var refused staleRevisionError
	if !errors.As(err, &refused) || refused.etag != `"`+strconv.FormatInt(testInstant.UnixMicro(), 10)+`"` {
		t.Fatalf("stale If-Match = %v, want a staleRevisionError with the current ETag", err)
	}

	fresh := stale
	fresh.Params.IfMatch = `"` + strconv.FormatInt(testInstant.UnixMicro(), 10) + `"`
	name := "Slack alerts"
	fresh.Body = &apigen.UpdateCredentialRequest{Name: &name}
	if _, err := h.UpdateCredential(ctx, fresh); err != nil {
		t.Fatalf("current If-Match: %v", err)
	}
	if svc.lastSet.Name != name || len(svc.lastSet.AllowedHosts) != 1 || svc.lastSet.AllowedHosts[0] != "hooks.slack.com" {
		t.Errorf("settings = %+v; the omitted hosts must keep their value", svc.lastSet)
	}
}

// TestCredentialWriteRaceAnswers412 (IDEM-5): the revision that passed the If-Match check travels to the write, and a
// write that loses the race to another change answers 412 with the current ETag, for the settings and the delete.
func TestCredentialWriteRaceAnswers412(t *testing.T) {
	cur := credential.Summary{Record: credential.Record{
		ID: newTestID(3), ProjectID: identity.ProjectID(newTestID(2)), Name: "Slack", AllowedHosts: []string{"hooks.slack.com"},
		Status: credential.StatusOK, CurrentVersion: 2, UpdatedAt: testInstant,
	}, Owned: true}
	svc := &fakeCredentials{byID: map[id.UUID]credential.Summary{cur.ID: cur}, writeErr: credential.ErrConflict}
	h := NewCredentials(svc)
	ctx := callerCtx(t)
	etag := `"` + strconv.FormatInt(testInstant.UnixMicro(), 10) + `"`
	wantStale := func(err error) {
		t.Helper()
		var refused staleRevisionError
		if !errors.As(err, &refused) || refused.etag != etag {
			t.Fatalf("lost race = %v, want a staleRevisionError with the current ETag", err)
		}
		if !svc.lastExpected.Equal(testInstant) {
			t.Fatalf("the write carried revision %v, want %v", svc.lastExpected, testInstant)
		}
	}
	_, err := h.UpdateCredential(ctx, apigen.UpdateCredentialRequestObject{
		CredentialId: apigen.CredentialId(cur.ID), Params: apigen.UpdateCredentialParams{IfMatch: etag}, Body: &apigen.UpdateCredentialRequest{},
	})
	wantStale(err)
	_, err = h.DeleteCredential(ctx, apigen.DeleteCredentialRequestObject{
		CredentialId: apigen.CredentialId(cur.ID), Params: apigen.DeleteCredentialParams{IfMatch: etag},
	})
	wantStale(err)
}

// TestListCredentialsPages (PAGE): a page carries a token for the next one, the last page carries none, and a token
// issued for another page size is refused.
func TestListCredentialsPages(t *testing.T) {
	mk := func(n byte, name string) credential.Summary {
		return credential.Summary{Record: credential.Record{ID: newTestID(n), ProjectID: identity.ProjectID(newTestID(2)), Name: name, Status: credential.StatusOK}}
	}
	svc := &fakeCredentials{list: []credential.Summary{mk(12, "c"), mk(10, "a"), mk(11, "b")}}
	h := NewCredentials(svc)
	ctx := callerCtx(t)
	limit := 2
	req := apigen.ListCredentialsRequestObject{ProjectId: apigen.ProjectId(newTestID(2)), Params: apigen.ListCredentialsParams{Limit: &limit}}
	first, err := h.ListCredentials(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	first200, ok := first.(apigen.ListCredentials200JSONResponse)
	if !ok {
		t.Fatalf("first page is %T", first)
	}
	page1 := first200.Body
	if len(page1.Items) != 2 || page1.Items[0].Name != "a" || page1.Items[1].Name != "b" || page1.NextPageToken == nil {
		t.Fatalf("first page = %+v", page1)
	}
	if page1.Items[0].PublicFields != nil {
		t.Errorf("list items carry public fields")
	}
	req.Params.PageToken = page1.NextPageToken
	second, err := h.ListCredentials(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	second200, ok := second.(apigen.ListCredentials200JSONResponse)
	if !ok {
		t.Fatalf("second page is %T", second)
	}
	page2 := second200.Body
	if len(page2.Items) != 1 || page2.Items[0].Name != "c" || page2.NextPageToken != nil {
		t.Fatalf("second page = %+v", page2)
	}
	other := 3
	req.Params.Limit = &other
	if _, err := h.ListCredentials(ctx, req); !errors.Is(err, errInvalidPageToken) {
		t.Errorf("a token of another page size = %v, want invalid page token", err)
	}
}

// TestCallerNeedsAPrincipalInAnOrganization: without a principal the handlers refuse before they touch the service.
func TestCallerNeedsAPrincipalInAnOrganization(t *testing.T) {
	h := NewCredentials(&fakeCredentials{byID: map[id.UUID]credential.Summary{}})
	_, err := h.ListCredentials(t.Context(), apigen.ListCredentialsRequestObject{ProjectId: apigen.ProjectId(newTestID(2))})
	if !errors.Is(err, identity.ErrUnauthenticated) {
		t.Errorf("no principal = %v, want unauthenticated", err)
	}
}

// TestCredentialErrorsAreProblems: the errors of the credential store reach the caller as the problem codes of the
// contract, and nothing else leaks.
func TestCredentialErrorsAreProblems(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
	}{
		{"not found", credential.ErrNotFound, http.StatusNotFound},
		{"invalid", credential.ErrInvalid, http.StatusBadRequest},
		{"conflict", credential.ErrConflict, http.StatusConflict},
		{"not owner", credential.ErrNotOwner, http.StatusForbidden},
		{"not usable", credential.ErrNotUsable, http.StatusForbidden},
		{"stale revision", staleRevisionError{etag: `"5"`}, http.StatusPreconditionFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/credentials/x", nil)
			h := errorHandler{logger: nopLogger()}
			h.response(rec, req, tc.err)
			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d", rec.Code, tc.status)
			}
			if tc.status == http.StatusPreconditionFailed && rec.Header().Get("ETag") != `"5"` {
				t.Errorf("412 lacks the current ETag: %q", rec.Header().Get("ETag"))
			}
		})
	}
}

// nopLogger returns a logger that discards its records.
func nopLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }
