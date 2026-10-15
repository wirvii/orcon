// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
)

// stubSelector answers the membership question and counts its calls.
type stubSelector struct {
	member bool
	err    error
	calls  int
}

func (s *stubSelector) SelectOrganization(context.Context, identity.UserID, identity.OrgID) (bool, error) {
	s.calls++
	return s.member, s.err
}

var _ authz.OrgSelector = (*stubSelector)(nil)

// selectedOrg records the organization the principal carries inside the handler.
type selectedOrg struct{ got identity.OrgID }

func (s *selectedOrg) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p, found := middleware.PrincipalFrom(r.Context()); found {
		s.got = p.OrgID
	}
	ok(w, r)
}

// TestSelectOrganizationBindsAMemberOrganization (ADR-0038 section 6): a session user with an
// ACTIVE membership in the named organization carries that organization on the principal.
func TestSelectOrganizationBindsAMemberOrganization(t *testing.T) {
	org := id.UUID{15: 9}
	uid := identity.UserID(id.UUID{15: 7})
	sel := &stubSelector{member: true}
	next := &selectedOrg{}
	h := middleware.SelectOrganization(sel, quiet, middleware.DefaultAuthConfig())(next)
	rec := serve(t, h, func(r *http.Request) {
		r.Header.Set("Orcon-Org", org.String())
		*r = *r.WithContext(middleware.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalUser, UserID: uid}))
	})
	if rec.Code != http.StatusNoContent {
		t.Fatalf("member: status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if next.got != identity.OrgID(org) || sel.calls != 1 {
		t.Errorf("member: organization = %v after %d lookups, want %v after one lookup", next.got, sel.calls, org)
	}
}

// TestSelectOrganizationHidesEveryRefusal: a non-member, an identifier that is not an
// organization, and a principal fixed to another organization all answer 404, the same
// answer a missing organization gets.
func TestSelectOrganizationHidesEveryRefusal(t *testing.T) {
	org := id.UUID{15: 9}
	other := id.UUID{15: 10}
	user := func(r *http.Request) {
		*r = *r.WithContext(middleware.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalUser, UserID: identity.UserID(id.UUID{15: 7})}))
	}
	fixed := func(r *http.Request) {
		*r = *r.WithContext(middleware.WithPrincipal(r.Context(), identity.Principal{
			Kind: identity.PrincipalServiceAccount, OrgID: identity.OrgID(other),
		}))
	}
	tests := []struct {
		name   string
		header string
		sel    *stubSelector
		mut    func(*http.Request)
		calls  int
	}{
		{"non-member", org.String(), &stubSelector{member: false}, user, 1},
		{"selector failure answers 500", org.String(), &stubSelector{err: errors.New("down")}, user, 1},
		{"not an identifier", "not-a-uuid", &stubSelector{member: true}, user, 0},
		{"non-user without fixed organization", org.String(), &stubSelector{member: true}, func(r *http.Request) {
			*r = *r.WithContext(middleware.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalServiceAccount}))
		}, 0},
		{"fixed principal, other organization", org.String(), &stubSelector{member: true}, fixed, 0},
	}
	for _, tt := range tests {
		h := middleware.SelectOrganization(tt.sel, quiet, middleware.DefaultAuthConfig())(http.HandlerFunc(ok))
		rec := serve(t, h, func(r *http.Request) {
			r.Header.Set("Orcon-Org", tt.header)
		}, tt.mut)
		want := http.StatusNotFound
		if tt.name == "selector failure answers 500" {
			want = http.StatusInternalServerError
		}
		if rec.Code != want {
			t.Errorf("%s: status = %d, want %d", tt.name, rec.Code, want)
		}
		if tt.sel.calls != tt.calls {
			t.Errorf("%s: %d membership lookups, want %d", tt.name, tt.sel.calls, tt.calls)
		}
	}
}

// TestSelectOrganizationPassesWhatNeedsNoChoice: without the header, or without a principal
// (a public route), the request continues unchanged, and a fixed principal naming its own
// organization continues too.
func TestSelectOrganizationPassesWhatNeedsNoChoice(t *testing.T) {
	org := id.UUID{15: 9}
	sel := &stubSelector{member: false}
	h := middleware.SelectOrganization(sel, quiet, middleware.DefaultAuthConfig())(http.HandlerFunc(ok))
	if rec := serve(t, h); rec.Code != http.StatusNoContent {
		t.Errorf("no header: status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	rec := serve(t, h, func(r *http.Request) {
		r.Header.Set("Orcon-Org", org.String())
	})
	if rec.Code != http.StatusNoContent {
		t.Errorf("public route with a header: status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	fixed := serve(t, h, func(r *http.Request) {
		r.Header.Set("Orcon-Org", org.String())
		*r = *r.WithContext(middleware.WithPrincipal(r.Context(), identity.Principal{Kind: identity.PrincipalUser, OrgID: identity.OrgID(org)}))
	})
	if fixed.Code != http.StatusNoContent {
		t.Errorf("fixed principal, same organization: status = %d, want %d", fixed.Code, http.StatusNoContent)
	}
	if sel.calls != 0 {
		t.Errorf("no lookup is needed here, got %d", sel.calls)
	}
}
