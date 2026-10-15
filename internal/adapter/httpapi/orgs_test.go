// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/middleware"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope/issue"
)

// fakeResolver answers the organizations of one user and counts the calls.
type fakeResolver struct {
	orgs  []identity.OrganizationChoice
	calls int
}

func (f *fakeResolver) ResolveUserOrganizations(_ context.Context, _ identity.UserID, limit int) ([]identity.OrganizationChoice, error) {
	f.calls++
	if limit != identity.MaxResolvedOrganizations {
		return nil, errors.New("the listing must read the contract's cap")
	}
	return append([]identity.OrganizationChoice(nil), f.orgs...), nil
}

// choice builds an organization of the test data with a fresh identity and its scope.
func choice(t *testing.T, slug, name string) identity.OrganizationChoice {
	t.Helper()
	oid := identity.OrgID(testIDs.New())
	scope, err := issue.New(id.UUID(oid))
	if err != nil {
		t.Fatalf("scope: %v", err)
	}
	return identity.OrganizationChoice{Scope: scope, ID: oid, Slug: slug, Name: name}
}

// listBody returns the body of a 200 answer of listOrgs, failing the test for any other answer.
func listBody(t *testing.T, resp apigen.ListOrgsResponseObject) apigen.OrganizationList {
	t.Helper()
	ok, isOK := resp.(apigen.ListOrgs200JSONResponse)
	if !isOK {
		t.Fatalf("answer = %T, want a 200", resp)
	}
	return ok.Body
}

// listCtx is a context with a signed-in user.
func listCtx(t *testing.T) context.Context {
	t.Helper()
	p := identity.Principal{Kind: identity.PrincipalUser, UserID: identity.UserID(testIDs.New()), CredentialType: identity.CredentialSession}
	return middleware.WithPrincipal(t.Context(), p)
}

func intPtr(n int) *int         { return &n }
func strVal(s string) *string   { return &s }
func tokenVal(s string) *string { return &s }
func pageOf(limit int) apigen.ListOrgsParams {
	return apigen.ListOrgsParams{Limit: intPtr(limit)}
}

// TestListOrgsPagesActiveMemberships (SPEC-016 R-SH-ORG-03): organizations come by name
// and then id, one page at a time, and the last page has no token.
func TestListOrgsPagesActiveMemberships(t *testing.T) {
	res := &fakeResolver{orgs: []identity.OrganizationChoice{
		choice(t, "wirvii", "Wirvii"),
		choice(t, "acme", "Acme"),
		choice(t, "zeta", "Zeta"),
	}}
	h := NewOrgs(res)
	first, err := h.ListOrgs(listCtx(t), apigen.ListOrgsRequestObject{Params: pageOf(2)})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	body := listBody(t, first)
	if len(body.Items) != 2 || body.Items[0].Name != "Acme" || body.Items[1].Name != "Wirvii" || body.NextPageToken == nil {
		t.Fatalf("first page = %+v; want Acme, Wirvii and a next token", body)
	}
	second, err := h.ListOrgs(listCtx(t), apigen.ListOrgsRequestObject{Params: apigen.ListOrgsParams{Limit: intPtr(2), PageToken: body.NextPageToken}})
	if err != nil {
		t.Fatalf("second page: %v", err)
	}
	last := listBody(t, second)
	if len(last.Items) != 1 || last.Items[0].Name != "Zeta" || last.NextPageToken != nil {
		t.Fatalf("second page = %+v; want Zeta and no token", last)
	}
}

// TestListOrgsSlugFilter: the slug filter is exact.
func TestListOrgsSlugFilter(t *testing.T) {
	res := &fakeResolver{orgs: []identity.OrganizationChoice{choice(t, "acme", "Acme"), choice(t, "acme-two", "Acme Two")}}
	out, err := NewOrgs(res).ListOrgs(listCtx(t), apigen.ListOrgsRequestObject{Params: apigen.ListOrgsParams{Slug: strVal("acme")}})
	if err != nil {
		t.Fatalf("ListOrgs: %v", err)
	}
	items := listBody(t, out).Items
	if len(items) != 1 || items[0].Slug != "acme" {
		t.Errorf("items = %+v; want only acme", items)
	}
}

// TestListOrgsRequiresSession: without a signed-in user there is no listing.
func TestListOrgsRequiresSession(t *testing.T) {
	res := &fakeResolver{}
	if _, err := NewOrgs(res).ListOrgs(t.Context(), apigen.ListOrgsRequestObject{}); !errors.Is(err, identity.ErrUnauthenticated) {
		t.Errorf("no principal: err = %v, want ErrUnauthenticated", err)
	}
	none := middleware.WithPrincipal(t.Context(), identity.Principal{Kind: identity.PrincipalServiceAccount})
	if _, err := NewOrgs(res).ListOrgs(none, apigen.ListOrgsRequestObject{}); !errors.Is(err, identity.ErrUnauthenticated) {
		t.Errorf("a principal without a user: err = %v, want ErrUnauthenticated", err)
	}
	if res.calls != 0 {
		t.Error("the resolver was asked without a user")
	}
}

// TestListOrgsRejectsForeignPageToken: a token issued for another filter is invalid.
func TestListOrgsRejectsForeignPageToken(t *testing.T) {
	res := &fakeResolver{orgs: []identity.OrganizationChoice{choice(t, "acme", "Acme"), choice(t, "beta", "Beta"), choice(t, "gamma", "Gamma")}}
	h := NewOrgs(res)
	first, err := h.ListOrgs(listCtx(t), apigen.ListOrgsRequestObject{Params: pageOf(1)})
	if err != nil {
		t.Fatalf("first page: %v", err)
	}
	token := listBody(t, first).NextPageToken
	if token == nil {
		t.Fatal("no next token")
	}
	_, err = h.ListOrgs(listCtx(t), apigen.ListOrgsRequestObject{Params: apigen.ListOrgsParams{Limit: intPtr(1), PageToken: token, Slug: strVal("beta")}})
	if !errors.Is(err, errInvalidPageToken) {
		t.Errorf("a token of another filter: err = %v, want errInvalidPageToken", err)
	}
	_, err = h.ListOrgs(listCtx(t), apigen.ListOrgsRequestObject{Params: apigen.ListOrgsParams{Limit: intPtr(1), PageToken: tokenVal("not-a-token")}})
	if !errors.Is(err, errInvalidPageToken) {
		t.Errorf("a malformed token: err = %v, want errInvalidPageToken", err)
	}
}

// TestListOrgsRefusesPageSizeOutOfRange: the page size is between 1 and 200.
func TestListOrgsRefusesPageSizeOutOfRange(t *testing.T) {
	h := NewOrgs(&fakeResolver{})
	for _, n := range []int{0, 201} {
		if _, err := h.ListOrgs(listCtx(t), apigen.ListOrgsRequestObject{Params: pageOf(n)}); !errors.Is(err, identity.ErrInvalidInput) {
			t.Errorf("limit %d: err = %v, want ErrInvalidInput", n, err)
		}
	}
}

// TestListOrgsAnswersJSON: the body carries the contract fields and nothing else.
func TestListOrgsAnswersJSON(t *testing.T) {
	res := &fakeResolver{orgs: []identity.OrganizationChoice{choice(t, "acme", "Acme")}}
	out, err := NewOrgs(res).ListOrgs(listCtx(t), apigen.ListOrgsRequestObject{})
	if err != nil {
		t.Fatalf("ListOrgs: %v", err)
	}
	rec := httptest.NewRecorder()
	if err := out.VisitListOrgsResponse(rec); err != nil {
		t.Fatalf("write: %v", err)
	}
	if rec.Code != http.StatusOK || rec.Body.String() == "" {
		t.Fatalf("status %d body %q", rec.Code, rec.Body.String())
	}
}
