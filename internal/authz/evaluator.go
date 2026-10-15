// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
	"github.com/wirvii/orcon/internal/platform/orgscope/issue"
)

// Store is what the evaluator reads. The adapters implement it; the organization's
// bindings are read in the organization's scope, and a project is resolved across
// organizations only to refuse a project of another organization (ADR-0041 section 6).
type Store interface {
	// Epoch returns the authorization epoch of the organization: a counter that every
	// change of roles, bindings, groups or memberships increments in the same transaction
	// (SPEC-004 R-AZ-06).
	Epoch(ctx context.Context, org orgscope.Org) (int64, error)
	// ActiveMember reports whether the user has an ACTIVE membership of the organization.
	ActiveMember(ctx context.Context, user identity.UserID, org orgscope.Org) (bool, error)
	// Bindings returns the role bindings of the user in the organization.
	Bindings(ctx context.Context, user identity.UserID, org orgscope.Org) ([]Binding, error)
	// CustomRoles returns the permissions of each custom role of the organization, by identifier.
	CustomRoles(ctx context.Context, org orgscope.Org) (map[string][]Permission, error)
	// ProjectOrganization returns the organization a project belongs to, or
	// [identity.ErrNotFound] when no project has that identifier.
	ProjectOrganization(ctx context.Context, project string) (identity.OrgID, error)
}

// Evaluator decides the requests of the API (ADR-0041 section 6, SPEC-004 R-AZ-05, R-AZ-06,
// R-AZ-10). It implements [Authorizer]. Grants are cached per (user, organization) and
// kept while the organization's epoch does not move, so a change of roles takes effect on
// the next request.
type Evaluator struct {
	store  Store
	cat    *Catalog
	owners map[string]OwnerResolver
	mu     sync.Mutex
	cache  map[string]cachedGrant
}

// OwnerResolver names the project that owns a resource of one kind, so the evaluator can decide a request
// about that resource with the grant of the project (SPEC-004 R-AZ-07, SE6-http). It returns
// [identity.ErrNotFound] when the organization holds no resource with that identifier; the resolver never
// answers for another organization, because the scope it receives is the principal's.
type OwnerResolver interface {
	OwnerProject(ctx context.Context, org orgscope.Org, id string) (identity.ProjectID, error)
}

// WithOwner returns the evaluator with the owner resolver of kind (the first path parameter names it, for
// example credential for {credentialId}). Kinds without a resolver are denied, as before.
func (e *Evaluator) WithOwner(kind string, r OwnerResolver) *Evaluator {
	next := make(map[string]OwnerResolver, len(e.owners)+1)
	for k, v := range e.owners {
		next[k] = v
	}
	next[kind] = r
	e.owners = next
	return e
}

type cachedGrant struct {
	epoch int64
	grant Grant
}

// maxCached bounds the grant cache; past it the cache starts again.
const maxCached = 10000

// NewEvaluator returns the evaluator over store and the permission catalog.
func NewEvaluator(store Store, cat *Catalog) *Evaluator {
	return &Evaluator{store: store, cat: cat, cache: map[string]cachedGrant{}}
}

var _ Authorizer = (*Evaluator)(nil)

// Authorize implements [Authorizer]. The visibility of a resource is the permission to
// read it (project:read, org:read): without it the resource is hidden (404); with it, a
// missing action is forbidden (403). A project of another organization is hidden.
// Resource kinds other than project are forbidden until their resolvers are wired.
func (e *Evaluator) Authorize(ctx context.Context, req Request) (Decision, error) {
	if !req.Requirement.NeedsDecision() {
		return DecisionAllow, nil
	}
	user := req.Principal.UserID
	org := req.Principal.OrgID
	if user.IsZero() || org.IsZero() {
		return DecisionForbidden, nil
	}
	scope, err := issue.New(id.UUID(org))
	if err != nil {
		return DecisionForbidden, fmt.Errorf("authorize: organization scope: %w", err)
	}
	project := ""
	visible := Permission("org:read")
	switch req.Resource.Kind {
	case "":
	case "project":
		owner, err := e.store.ProjectOrganization(ctx, req.Resource.ID)
		if errors.Is(err, identity.ErrNotFound) || (err == nil && owner != org) {
			return DecisionHidden, nil
		}
		if err != nil {
			return DecisionForbidden, fmt.Errorf("authorize: project: %w", err)
		}
		project = req.Resource.ID
		visible = "project:read"
	default:
		r, ok := e.owners[req.Resource.Kind]
		if !ok {
			return DecisionForbidden, nil
		}
		owner, err := r.OwnerProject(ctx, scope, req.Resource.ID)
		if errors.Is(err, identity.ErrNotFound) {
			return DecisionHidden, nil
		}
		if err != nil {
			return DecisionForbidden, fmt.Errorf("authorize: %s owner: %w", req.Resource.Kind, err)
		}
		project = owner.String()
		visible = Permission(req.Resource.Kind + ":read")
	}
	active, err := e.store.ActiveMember(ctx, user, scope)
	if err != nil {
		return DecisionForbidden, fmt.Errorf("authorize: membership: %w", err)
	}
	if !active {
		return DecisionHidden, nil
	}
	grant, err := e.grant(ctx, user, scope)
	if err != nil {
		return DecisionForbidden, err
	}
	have := grant.For(project)
	if satisfies(req.Requirement, have) {
		return DecisionAllow, nil
	}
	if !slices.Contains(have, visible) {
		return DecisionHidden, nil
	}
	return DecisionForbidden, nil
}

// grant returns the grant of the user in the organization, recomputed when the epoch moved.
func (e *Evaluator) grant(ctx context.Context, user identity.UserID, org orgscope.Org) (Grant, error) {
	epoch, err := e.store.Epoch(ctx, org)
	if err != nil {
		return Grant{}, fmt.Errorf("authorize: epoch: %w", err)
	}
	key := user.String() + "|" + org.String()
	e.mu.Lock()
	c, ok := e.cache[key]
	e.mu.Unlock()
	if ok && c.epoch == epoch {
		return c.grant, nil
	}
	bindings, err := e.store.Bindings(ctx, user, org)
	if err != nil {
		return Grant{}, fmt.Errorf("authorize: bindings: %w", err)
	}
	custom, err := e.store.CustomRoles(ctx, org)
	if err != nil {
		return Grant{}, fmt.Errorf("authorize: custom roles: %w", err)
	}
	g := Resolve(e.cat, bindings, custom)
	e.mu.Lock()
	if len(e.cache) >= maxCached {
		e.cache = map[string]cachedGrant{}
	}
	e.cache[key] = cachedGrant{epoch: epoch, grant: g}
	e.mu.Unlock()
	return g, nil
}

// satisfies reports whether have meets the requirement: every listed permission, or at least one.
func satisfies(req Requirement, have []Permission) bool {
	switch req.Mode {
	case ModeAll:
		return len(Missing(req.Permissions, have)) == 0
	case ModeAny:
		for _, p := range req.Permissions {
			if slices.Contains(have, p) {
				return true
			}
		}
	case ModeAuthenticated, ModeAnonymous:
		return false
	}
	return false
}
