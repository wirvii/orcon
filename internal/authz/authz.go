// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package authz is the business area of permissions, roles and their
// evaluation (ADR-0041). It is an area package: it imports no adapter.
//
// Today it holds the permission catalog, the [Authorizer] port that the request
// pipeline of the public API consumes (ADR-0052 section 4, step 6) and the
// implementation that denies everything, [DenyAll]. The real evaluation arrives
// with the authorization cut of SPEC-004 and implements the same port without
// changing its shape.
package authz

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// Permission is a catalog name in the form resource:action, for example
// flow:read. Names are identifiers and keep this form (ADR-0041 section 2).
type Permission string

// Scope is where a permission applies.
type Scope string

// The scopes of the catalog.
const (
	// ScopeOrganization permissions apply to an organization and its projects.
	ScopeOrganization Scope = "organization"
	// ScopeProject permissions apply inside one project.
	ScopeProject Scope = "project"
	// ScopeInstance permissions apply to the installation (ADR-0038 section 4).
	ScopeInstance Scope = "instance"
)

// ErrInvalidCatalog means a catalog definition is malformed.
var ErrInvalidCatalog = errors.New("invalid permission catalog")

// Catalog is the set of permissions the product knows.
type Catalog struct {
	scopes map[Permission]Scope
}

// NewCatalog builds a catalog from the permissions of each scope. A name must
// be of the form resource:action and may appear only once in the whole catalog.
func NewCatalog(byScope map[Scope][]Permission) (*Catalog, error) {
	c := &Catalog{scopes: map[Permission]Scope{}}
	for scope, names := range byScope {
		switch scope {
		case ScopeOrganization, ScopeProject, ScopeInstance:
		default:
			return nil, fmt.Errorf("%w: unknown scope %q", ErrInvalidCatalog, scope)
		}
		for _, name := range names {
			if !validName(string(name)) {
				return nil, fmt.Errorf("%w: %q is not of the form resource:action", ErrInvalidCatalog, name)
			}
			if prev, dup := c.scopes[name]; dup {
				return nil, fmt.Errorf("%w: %q is in both %s and %s", ErrInvalidCatalog, name, prev, scope)
			}
			c.scopes[name] = scope
		}
	}
	return c, nil
}

// Has reports whether name is in the catalog.
func (c *Catalog) Has(name Permission) bool {
	_, ok := c.scopes[name]
	return ok
}

// ScopeOf returns the scope of name.
func (c *Catalog) ScopeOf(name Permission) (Scope, bool) {
	s, ok := c.scopes[name]
	return s, ok
}

// Names returns every permission name in order.
func (c *Catalog) Names() []Permission {
	out := make([]Permission, 0, len(c.scopes))
	for n := range c.scopes {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// validName reports whether s is resource:action with lowercase letters,
// digits and hyphens in both parts.
func validName(s string) bool {
	resource, action, ok := strings.Cut(s, ":")
	return ok && validPart(resource) && validPart(action)
}

func validPart(p string) bool {
	if p == "" || p[0] < 'a' || p[0] > 'z' {
		return false
	}
	for i := range len(p) {
		ch := p[i]
		if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '-' {
			return false
		}
	}
	return true
}

// Mode says how the permissions of a [Requirement] combine.
type Mode int

// The modes of x-orcon-permission (CHAN-7).
const (
	// ModeAll requires every listed permission.
	ModeAll Mode = iota
	// ModeAny requires at least one listed permission.
	ModeAny
	// ModeAuthenticated requires only an authenticated principal.
	ModeAuthenticated
	// ModeAnonymous requires nothing; only protocol operations use it.
	ModeAnonymous
)

// Requirement is what an operation needs, from its x-orcon-permission.
type Requirement struct {
	// Mode says how Permissions combine.
	Mode Mode
	// Permissions are the listed names; empty for authenticated and anonymous.
	Permissions []Permission
}

// NeedsDecision reports whether the authorizer must be asked: operations that
// need only a principal, or nothing, are decided by the pipeline itself.
func (r Requirement) NeedsDecision() bool {
	return r.Mode == ModeAll || r.Mode == ModeAny
}

// ResourceRef names the resource a request addresses, as the identifiers of
// its URL. The identifier authorizes; the handler then operates on the row it
// loads within the organization scope (R-API-04, R-ISO-07).
type ResourceRef struct {
	// Kind is the resource kind taken from the path parameter, for example flow.
	Kind string
	// ID is the identifier as sent in the URL; empty for collections.
	ID string
}

// Request is one authorization question.
type Request struct {
	// Principal is who acts.
	Principal identity.Principal
	// Requirement is what the operation needs.
	Requirement Requirement
	// Resource is what the request addresses.
	Resource ResourceRef
}

// Decision is the answer of the authorizer.
type Decision int

// The decisions. Visibility follows ADR-0041 section 6: another organization or
// no read permission is hidden (404), read without the action is forbidden (403).
const (
	// DecisionForbidden denies the action; the caller can see the resource.
	DecisionForbidden Decision = iota
	// DecisionHidden denies and hides that the resource exists.
	DecisionHidden
	// DecisionAllow permits the action.
	DecisionAllow
)

// Authorizer decides whether a principal may do what a request asks. When the decision is
// [DecisionAllow] it also returns the organization the request was authorized in: that scope
// is the only one a handler may use (ADR-0046 section 2), so the scope is never built from a
// request value. Otherwise the scope is the zero value.
type Authorizer interface {
	Authorize(ctx context.Context, req Request) (Decision, orgscope.Org, error)
}

// DenyAll is the authorizer that forbids every question. The composition root
// wires it until the real evaluator exists, so nothing is ever allowed by
// accident.
type DenyAll struct{}

// Authorize implements [Authorizer].
func (DenyAll) Authorize(context.Context, Request) (Decision, orgscope.Org, error) {
	return DecisionForbidden, orgscope.Org{}, nil
}

type scopeKey struct{}

// WithOrgScope returns a context that carries the organization a request was authorized in.
// The authorization middleware sets it after an allow decision; only the authorizer can
// issue the scope it carries.
func WithOrgScope(ctx context.Context, org orgscope.Org) context.Context {
	return context.WithValue(ctx, scopeKey{}, org)
}

// OrgScopeFrom returns the organization scope the request was authorized in, when the
// authorization step allowed it.
func OrgScopeFrom(ctx context.Context) (orgscope.Org, bool) {
	org, ok := ctx.Value(scopeKey{}).(orgscope.Org)
	return org, ok
}
