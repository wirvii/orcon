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
	store Store
	cat   *Catalog
	mu    sync.Mutex
	cache map[string]cachedGrant
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

// OrgSelector decides whether a session principal may work in the organization it named
// with the Orcon-Org header (ADR-0038 section 6, SPEC-004 SE6-http).
type OrgSelector interface {
	// SelectOrganization reports whether user holds an ACTIVE membership of org. It returns
	// false, not an error, for a membership that does not exist.
	SelectOrganization(ctx context.Context, user identity.UserID, org identity.OrgID) (bool, error)
}

var _ OrgSelector = (*Evaluator)(nil)

// SelectOrganization implements [OrgSelector]: one lookup of (organization, user) in the
// request plane. A zero user or organization is never a member.
func (e *Evaluator) SelectOrganization(ctx context.Context, user identity.UserID, org identity.OrgID) (bool, error) {
	if user.IsZero() || org.IsZero() {
		return false, nil
	}
	scope, err := issue.New(id.UUID(org))
	if err != nil {
		return false, fmt.Errorf("select organization: scope: %w", err)
	}
	active, err := e.store.ActiveMember(ctx, user, scope)
	if err != nil {
		return false, fmt.Errorf("select organization: membership: %w", err)
	}
	return active, nil
}

// Authorize implements [Authorizer]. The visibility of a resource is the permission to
// read it (project:read, org:read): without it the resource is hidden (404); with it, a
// missing action is forbidden (403). A resource of another organization is hidden.
//
// The organization a request addresses comes from the resource: an organization resource
// is its own organization, a project resource belongs to the organization of the project.
// A session principal carries no organization, so the addressed one is the organization of
// the request, and it must have an active membership there. A key or token principal is
// fixed to its organization and sees nothing of another. Resource kinds other than org and
// project are forbidden until their resolvers are wired.
func (e *Evaluator) Authorize(ctx context.Context, req Request) (Decision, orgscope.Org, error) {
	if !req.Requirement.NeedsDecision() {
		return DecisionAllow, orgscope.Org{}, nil
	}
	user := req.Principal.UserID
	if user.IsZero() {
		return DecisionForbidden, orgscope.Org{}, nil
	}
	target, project, visible, decision, err := e.target(ctx, req)
	if decision != DecisionAllow || err != nil {
		return decision, orgscope.Org{}, err
	}
	scope, err := issue.New(id.UUID(target))
	if err != nil {
		return DecisionForbidden, orgscope.Org{}, fmt.Errorf("authorize: organization scope: %w", err)
	}
	active, err := e.store.ActiveMember(ctx, user, scope)
	if err != nil {
		return DecisionForbidden, orgscope.Org{}, fmt.Errorf("authorize: membership: %w", err)
	}
	if !active {
		return DecisionHidden, orgscope.Org{}, nil
	}
	grant, err := e.grant(ctx, user, scope)
	if err != nil {
		return DecisionForbidden, orgscope.Org{}, err
	}
	have := grant.For(project)
	if satisfies(req.Requirement, have) {
		return DecisionAllow, scope, nil
	}
	if !slices.Contains(have, visible) {
		return DecisionHidden, orgscope.Org{}, nil
	}
	return DecisionForbidden, orgscope.Org{}, nil
}

// target resolves the organization a request addresses, the project inside it (empty for
// an organization resource), and the permission that makes the resource visible. A decision
// other than allow ends the question: the request is hidden or forbidden before any grant
// is read.
func (e *Evaluator) target(ctx context.Context, req Request) (identity.OrgID, string, Permission, Decision, error) {
	own := req.Principal.OrgID
	switch req.Resource.Kind {
	case "":
		if own.IsZero() {
			return identity.OrgID{}, "", "", DecisionForbidden, nil
		}
		return own, "", Permission("org:read"), DecisionAllow, nil
	case "org":
		parsed, err := id.Parse(req.Resource.ID)
		if err != nil || (!own.IsZero() && identity.OrgID(parsed) != own) {
			// An identifier that is not an organization names none: the answer is hidden, not an error.
			return identity.OrgID{}, "", "", DecisionHidden, nil //nolint:nilerr // see the comment above
		}
		return identity.OrgID(parsed), "", Permission("org:read"), DecisionAllow, nil
	case "project":
		owner, err := e.store.ProjectOrganization(ctx, req.Resource.ID)
		if errors.Is(err, identity.ErrNotFound) || (err == nil && !own.IsZero() && owner != own) {
			return identity.OrgID{}, "", "", DecisionHidden, nil
		}
		if err != nil {
			return identity.OrgID{}, "", "", DecisionForbidden, fmt.Errorf("authorize: project: %w", err)
		}
		return owner, req.Resource.ID, Permission("project:read"), DecisionAllow, nil
	default:
		return identity.OrgID{}, "", "", DecisionForbidden, nil
	}
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
