// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"slices"
	"strings"
)

// Predefined roles (ADR-0041 section 3, SPEC-004 R-AZ-02). They are immutable and are
// defined here, not stored: a binding names one by its name.
const (
	RoleOwner        = "Owner"
	RoleAdmin        = "Admin"
	RoleMember       = "Member"
	RoleProjectAdmin = "Project Admin"
	RoleEditor       = "Editor"
	RoleOperator     = "Operator"
	RoleViewer       = "Viewer"
)

// BuiltinRoles lists the predefined role names.
var BuiltinRoles = []string{RoleOwner, RoleAdmin, RoleMember, RoleProjectAdmin, RoleEditor, RoleOperator, RoleViewer}

// IsBuiltin reports whether name is a predefined role.
func IsBuiltin(name string) bool { return slices.Contains(BuiltinRoles, name) }

// Scope kinds of a binding, as the database stores them.
const (
	ScopeKindOrg     = "ORG"
	ScopeKindProject = "PROJECT"
)

// BuiltinPermissions returns the permissions of a predefined role, from the catalog.
// The rule of each role is ADR-0041 section 3; the wildcards expand over the catalog
// scope of the permission family.
func BuiltinPermissions(cat *Catalog, role string) []Permission {
	all := cat.Names()
	orgScoped := func(p Permission) bool { s, ok := cat.ScopeOf(p); return ok && s == ScopeOrganization }
	projScoped := func(p Permission) bool { s, ok := cat.ScopeOf(p); return ok && s == ScopeProject }
	family := func(prefix string, skip ...string) []Permission {
		var out []Permission
		for _, p := range all {
			if strings.HasPrefix(string(p), prefix+":") && !slices.Contains(skip, string(p)) {
				out = append(out, p)
			}
		}
		return out
	}
	switch role {
	case RoleOwner:
		return filter(all, func(p Permission) bool { return orgScoped(p) || projScoped(p) })
	case RoleAdmin:
		return filter(all, func(p Permission) bool {
			return (orgScoped(p) || projScoped(p)) && p != "org:delete"
		})
	case RoleMember:
		return names("org:read", "member:read", "role:read", "project:create", "secret-store:reference")
	case RoleProjectAdmin:
		return filter(all, projScoped)
	case RoleEditor:
		out := names("project:read", "human-task:resolve", "credential:read", "credential:create",
			"credential:update", "credential:delete", "credential:use", "credential:share")
		out = append(out, family("flow", "flow:move")...)
		out = append(out, family("execution", "execution:delete")...)
		out = append(out, family("variable")...)
		return out
	case RoleOperator:
		return names("project:read", "flow:read", "flow:activate", "execution:read",
			"execution:retry", "execution:cancel", "human-task:resolve")
	case RoleViewer:
		return names("project:read", "flow:read", "execution:read", "credential:read", "variable:read")
	default:
		return nil
	}
}

// filter keeps the permissions of list for which keep holds.
func filter(list []Permission, keep func(Permission) bool) []Permission {
	var out []Permission
	for _, p := range list {
		if keep(p) {
			out = append(out, p)
		}
	}
	return out
}

// names converts literal permission names.
func names(list ...string) []Permission {
	out := make([]Permission, 0, len(list))
	for _, n := range list {
		out = append(out, Permission(n))
	}
	return out
}

// Binding is one role given to a user: at the organization (applies to every project)
// or in one project.
type Binding struct {
	// Role is a predefined role name or the identifier of a custom role.
	Role string
	// Project is the project of a project binding; empty for an organization binding.
	Project string
}

// Grant is what an organization makes of its bindings for one user: the permissions
// that apply to every project (organization bindings) and those of each project.
type Grant struct {
	// Everywhere holds the permissions of organization bindings: the organization
	// scope ones, and the project scope ones that apply to every project.
	Everywhere []Permission
	// PerProject holds the project scope permissions of project bindings, by project.
	PerProject map[string][]Permission
}

// Resolve computes the grant of bindings. The custom argument maps the identifier of
// each custom role of the organization to its permissions; an unknown role grants nothing (deny by
// default, ADR-0041 section 6).
func Resolve(cat *Catalog, bindings []Binding, custom map[string][]Permission) Grant {
	g := Grant{PerProject: map[string][]Permission{}}
	for _, b := range bindings {
		perms := rolePermissions(cat, b.Role, custom)
		if b.Project == "" {
			g.Everywhere = append(g.Everywhere, perms...)
			continue
		}
		for _, p := range perms {
			if s, ok := cat.ScopeOf(p); ok && s == ScopeProject {
				g.PerProject[b.Project] = append(g.PerProject[b.Project], p)
			}
		}
	}
	return g
}

// For returns the permissions that hold in a project (empty for the organization as a
// whole, when project is empty): the organization bindings plus the project bindings of
// that project.
func (g Grant) For(project string) []Permission {
	out := slices.Clone(g.Everywhere)
	if project != "" {
		out = append(out, g.PerProject[project]...)
	}
	return out
}

// rolePermissions returns what a role grants, predefined or custom; unknown grants nothing.
func rolePermissions(cat *Catalog, role string, custom map[string][]Permission) []Permission {
	if IsBuiltin(role) {
		return BuiltinPermissions(cat, role)
	}
	return custom[role]
}

// Missing returns the permissions of want that are not in have.
func Missing(want, have []Permission) []Permission {
	var out []Permission
	for _, p := range want {
		if !slices.Contains(have, p) {
			out = append(out, p)
		}
	}
	return out
}

// MayGrant reports whether an actor holding actorHas may give a role that includes
// requested in the same scope: no escalation (SPEC-004 R-AZ-03).
func MayGrant(actorHas, requested []Permission) bool {
	return len(Missing(requested, actorHas)) == 0
}
