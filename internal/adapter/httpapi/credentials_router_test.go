// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/adapter/httpapi/apigen"
	"github.com/wirvii/orcon/internal/adapter/httpapi/idempotency"
	"github.com/wirvii/orcon/internal/authz"
	"github.com/wirvii/orcon/internal/credential"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/config"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// credServer serves the credential operations of the contract and refuses the others, as the composition root does.
type credServer struct {
	apigen.StrictServerInterface
	c *Credentials
}

func (s credServer) ListCredentials(ctx context.Context, r apigen.ListCredentialsRequestObject) (apigen.ListCredentialsResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.c.ListCredentials(ctx, r)
}

func (s credServer) CreateCredential(ctx context.Context, r apigen.CreateCredentialRequestObject) (apigen.CreateCredentialResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.c.CreateCredential(ctx, r)
}

func (s credServer) GetCredential(ctx context.Context, r apigen.GetCredentialRequestObject) (apigen.GetCredentialResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.c.GetCredential(ctx, r)
}

func (s credServer) UpdateCredential(ctx context.Context, r apigen.UpdateCredentialRequestObject) (apigen.UpdateCredentialResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.c.UpdateCredential(ctx, r)
}

func (s credServer) ReplaceCredentialSecret(ctx context.Context, r apigen.ReplaceCredentialSecretRequestObject) (apigen.ReplaceCredentialSecretResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.c.ReplaceCredentialSecret(ctx, r)
}

func (s credServer) DeleteCredential(ctx context.Context, r apigen.DeleteCredentialRequestObject) (apigen.DeleteCredentialResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.c.DeleteCredential(ctx, r)
}

func (s credServer) CreateCredentialShare(ctx context.Context, r apigen.CreateCredentialShareRequestObject) (apigen.CreateCredentialShareResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.c.CreateCredentialShare(ctx, r)
}

func (s credServer) DeleteCredentialShare(ctx context.Context, r apigen.DeleteCredentialShareRequestObject) (apigen.DeleteCredentialShareResponseObject, error) { //nolint:ireturn // the generated interface requires it
	return s.c.DeleteCredentialShare(ctx, r)
}

// routerAuthzStore is the authorization store of the router tests: one organization, two projects, and the bindings
// and custom roles each test sets.
type routerAuthzStore struct {
	bindings []authz.Binding
	custom   map[string][]authz.Permission
	projects map[string]identity.OrgID
	// epoch moves with every change of bindings, as the stores move the authorization epoch of an organization
	// (SPEC-004 R-AZ-06); the evaluator keeps a grant while the epoch does not move.
	epoch int64
}

// setBindings replaces the bindings and moves the epoch, so the next request sees the new grant.
func (s *routerAuthzStore) setBindings(b ...authz.Binding) {
	s.bindings = b
	s.epoch++
}

func (s *routerAuthzStore) Epoch(context.Context, orgscope.Org) (int64, error) {
	return s.epoch + 1, nil
}

func (s *routerAuthzStore) ActiveMember(context.Context, identity.UserID, orgscope.Org) (bool, error) {
	return true, nil
}

func (s *routerAuthzStore) Bindings(context.Context, identity.UserID, orgscope.Org) ([]authz.Binding, error) {
	return s.bindings, nil
}

func (s *routerAuthzStore) CustomRoles(context.Context, orgscope.Org) (map[string][]authz.Permission, error) {
	return s.custom, nil
}

func (s *routerAuthzStore) ProjectOrganization(_ context.Context, project string) (identity.OrgID, error) {
	org, ok := s.projects[project]
	if !ok {
		return identity.OrgID{}, identity.ErrNotFound
	}
	return org, nil
}

// routerFixture is one router over the real contract, the real evaluator and the credential handlers.
type routerFixture struct {
	t      *testing.T
	router *Router
	svc    *fakeCredentials
	store  *routerAuthzStore
	org    identity.OrgID
	projA  string
	projB  string
}

// principalTokens maps a bearer token to a principal of the organization.
func (f *routerFixture) principalTokens() fakeAuthn {
	user := func(n byte, org identity.OrgID) identity.Principal {
		return identity.Principal{
			ID: identity.PrincipalID(newTestID(n)), Kind: identity.PrincipalUser, UserID: identity.UserID(newTestID(n)),
			OrgID: org, CredentialType: identity.CredentialSession,
		}
	}
	return fakeAuthn{
		"editor": user(20, f.org), "viewer": user(21, f.org), "outsider": user(22, f.org),
		"no-context": user(23, identity.OrgID{}),
	}
}

// newRouterFixture builds the fixture. Project A owns credential 3; project B is another project of the organization.
func newRouterFixture(t *testing.T) *routerFixture {
	t.Helper()
	f := &routerFixture{t: t, org: identity.OrgID(newTestID(9))}
	f.projA = newTestID(2).String()
	f.projB = newTestID(4).String()
	f.store = &routerAuthzStore{
		custom: map[string][]authz.Permission{
			"editor": {"project:read", "credential:read", "credential:create", "credential:update", "credential:delete", "credential:share"},
			"viewer": {"project:read", "credential:read"},
		},
		projects: map[string]identity.OrgID{f.projA: f.org, f.projB: f.org},
	}
	catalog, err := authz.NewCatalog(map[authz.Scope][]authz.Permission{
		authz.ScopeOrganization: {"org:read"},
		authz.ScopeProject:      {"project:read", "credential:read", "credential:create", "credential:update", "credential:delete", "credential:share"},
	})
	if err != nil {
		t.Fatal(err)
	}
	f.svc = &fakeCredentials{byID: map[id.UUID]credential.Summary{}}
	f.svc.byID[newTestID(3)] = credential.Summary{Record: credential.Record{
		ID: newTestID(3), ProjectID: identity.ProjectID(newTestID(2)), Name: "Slack", ConnectorReference: "orcon.http@1",
		AllowedHosts: []string{"hooks.slack.com"}, Status: credential.StatusOK, CurrentVersion: 1,
		CreatedAt: testInstant, UpdatedAt: testInstant,
	}, Owned: true, Secret: credential.SecretState{Set: true, UpdatedAt: testInstant}, Public: map[string]string{"header": "Authorization"}}
	deps := &Deps{
		Role:          config.RoleAPI,
		Logger:        slog.New(slog.DiscardHandler),
		Clock:         clock.NewFake(testInstant),
		IDs:           id.NewSequence(),
		Authenticator: f.principalTokens(),
		Authorizer:    authz.NewEvaluator(f.store, catalog).WithOwner("credential", f.svc),
		Idempotency:   idempotency.NewMemoryStore(clock.NewFake(testInstant)),
		Server:        credServer{StrictServerInterface: Unavailable{}, c: NewCredentials(f.svc)},
	}
	rt, err := New(deps)
	if err != nil {
		t.Fatalf("build the router: %v", err)
	}
	f.router = rt
	return f
}

// send sends one request as the principal of token and returns the recorded response.
func (f *routerFixture) send(method, path, token, body string, headers map[string]string) *httptest.ResponseRecorder {
	f.t.Helper()
	req := httptest.NewRequestWithContext(f.t.Context(), method, path, strings.NewReader(body))
	req.RemoteAddr = "192.0.2.10:4000"
	req.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec
}

// TestCredentialRoutesAreDecidedByTheEvaluator (SPEC-004 R-AZ-07, R-AZ-10, SE6-http): the owner's editor reads the
// credential; a viewer of the owner reads it but cannot change it; a user of another project does not see it; an
// identifier the organization does not hold is not found; a principal without an organization context is refused.
func TestCredentialRoutesAreDecidedByTheEvaluator(t *testing.T) {
	f := newRouterFixture(t)
	f.store.setBindings(authz.Binding{Role: "editor", Project: f.projA})
	cred := "/api/v1/credentials/" + newTestID(3).String()

	get := f.send(http.MethodGet, cred, "editor", "", nil)
	if get.Code != http.StatusOK || get.Header().Get("ETag") == "" {
		t.Fatalf("editor reads the credential: %d %s", get.Code, get.Body.String())
	}
	if strings.Contains(get.Body.String(), "secretFields") || strings.Contains(get.Body.String(), "token") {
		t.Errorf("the body names secret material: %s", get.Body.String())
	}

	f.store.setBindings(authz.Binding{Role: "viewer", Project: f.projA})
	if rec := f.send(http.MethodGet, cred, "viewer", "", nil); rec.Code != http.StatusOK {
		t.Errorf("viewer of the owner reads it: %d", rec.Code)
	}
	patch := `{"name":"Renamed"}`
	if rec := f.send(http.MethodPatch, cred, "viewer", patch, map[string]string{"If-Match": get.Header().Get("ETag"), "Content-Type": "application/merge-patch+json"}); rec.Code != http.StatusForbidden {
		t.Errorf("viewer of the owner updates it: %d, want 403", rec.Code)
	}

	f.store.setBindings(authz.Binding{Role: "editor", Project: f.projB})
	if rec := f.send(http.MethodGet, cred, "editor", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("a user of another project reads it: %d, want 404", rec.Code)
	}

	f.store.setBindings(authz.Binding{Role: "editor", Project: f.projA})
	if rec := f.send(http.MethodGet, "/api/v1/credentials/"+newTestID(77).String(), "editor", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("an unknown credential: %d, want 404", rec.Code)
	}
	if rec := f.send(http.MethodGet, cred, "no-context", "", nil); rec.Code != http.StatusForbidden {
		t.Errorf("a principal without an organization context: %d, want 403", rec.Code)
	}
}

// TestCredentialChangesNeedAFreshRevision (IDEM-5): an edit without If-Match is refused with 428; one based on an old
// revision with 412 and the current ETag; one based on the current revision goes through.
func TestCredentialChangesNeedAFreshRevision(t *testing.T) {
	f := newRouterFixture(t)
	f.store.setBindings(authz.Binding{Role: "editor", Project: f.projA})
	cred := "/api/v1/credentials/" + newTestID(3).String()
	patch := map[string]string{"Content-Type": "application/merge-patch+json"}
	withTag := func(tag string) map[string]string {
		h := map[string]string{"If-Match": tag}
		for k, v := range patch {
			h[k] = v
		}
		return h
	}

	if rec := f.send(http.MethodPatch, cred, "editor", `{"name":"Renamed"}`, patch); rec.Code != http.StatusPreconditionRequired {
		t.Errorf("edit without If-Match: %d, want 428", rec.Code)
	}
	stale := f.send(http.MethodPatch, cred, "editor", `{"name":"Renamed"}`, withTag(`"1"`))
	if stale.Code != http.StatusPreconditionFailed || stale.Header().Get("ETag") == "" {
		t.Errorf("stale If-Match: %d with ETag %q, want 412 with the current ETag", stale.Code, stale.Header().Get("ETag"))
	}
	current := f.send(http.MethodGet, cred, "editor", "", nil).Header().Get("ETag")
	if rec := f.send(http.MethodPatch, cred, "editor", `{"name":"Renamed"}`, withTag(current)); rec.Code != http.StatusOK {
		t.Errorf("edit on the current revision: %d %s", rec.Code, rec.Body.String())
	}
}

// TestCreateCredentialAnswersWithoutTheSecret: the creation goes through the project route, and the answer holds
// the new credential without its secret value.
func TestCreateCredentialAnswersWithoutTheSecret(t *testing.T) {
	f := newRouterFixture(t)
	f.store.setBindings(authz.Binding{Role: "editor", Project: f.projA})
	const secret = "probe-secret-value-router"
	body := `{"name":"Slack alerts","type":"orcon.http@1","allowedHosts":["hooks.slack.com"],"secretFields":{"token":"` + secret + `"}}`
	rec := f.send(http.MethodPost, "/api/v1/projects/"+f.projA+"/credentials", "editor", body, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), secret) {
		t.Errorf("the answer carries the secret value: %s", rec.Body.String())
	}
	if rec.Header().Get("Location") == "" || rec.Header().Get("ETag") == "" {
		t.Errorf("the answer lacks Location or ETag")
	}
	var doc map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&doc); err != nil {
		t.Fatalf("answer is not JSON: %v", err)
	}
	if doc["type"] != "orcon.http@1" {
		t.Errorf("type = %v", doc["type"])
	}
}

func (s *routerAuthzStore) ResourceProject(context.Context, string, string) (identity.OrgID, string, error) {
	return identity.OrgID{}, "", identity.ErrNotFound
}
