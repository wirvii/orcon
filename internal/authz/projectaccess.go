// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"context"
	"fmt"
	"slices"

	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// Permissions that decide which projects of an organization a user reads.
const (
	permProjectRead    Permission = "project:read"
	permProjectReadAll Permission = "project:read-all"
)

// ProjectAccess says which projects of one organization a user can read: every one (through an
// organization binding that holds project:read or project:read-all) or only the ones named by
// project bindings that hold project:read. It is computed once from the user's grant, so deciding
// a list of projects costs one grant lookup, not one per project (SPEC-016 R-SH-PRJ-01).
type ProjectAccess struct {
	readAll   bool
	everyone  bool
	projects  map[string]struct{}
	allowNone bool
}

// ProjectAccessOf derives the project access of a grant.
func ProjectAccessOf(g Grant) ProjectAccess {
	a := ProjectAccess{projects: map[string]struct{}{}}
	a.readAll = slices.Contains(g.Everywhere, permProjectReadAll)
	a.everyone = a.readAll || slices.Contains(g.Everywhere, permProjectRead)
	for project, perms := range g.PerProject {
		if slices.Contains(perms, permProjectRead) {
			a.projects[project] = struct{}{}
		}
	}
	a.allowNone = !a.everyone && len(a.projects) == 0
	return a
}

// CanRead reports whether the user can read the project (a project identifier in canonical text).
func (a ProjectAccess) CanRead(project string) bool {
	if a.everyone {
		return true
	}
	_, ok := a.projects[project]
	return ok
}

// ReadsEveryProject reports whether the user holds project:read-all: access to every project
// of the organization without any project binding. Such a user needs no Project Admin binding
// on a project they create.
func (a ProjectAccess) ReadsEveryProject() bool { return a.readAll }

// ProjectAccess returns the project access of the user in the organization, from the cached
// grant (recomputed when the organization's authorization epoch moved).
func (e *Evaluator) ProjectAccess(ctx context.Context, user identity.UserID, org orgscope.Org) (ProjectAccess, error) {
	if user.IsZero() {
		return ProjectAccessOf(Grant{}), nil
	}
	g, err := e.grant(ctx, user, org)
	if err != nil {
		return ProjectAccess{}, err
	}
	return ProjectAccessOf(g), nil
}

// ProjectLister is the part of the project store that [ReadableProjects] reads.
type ProjectLister interface {
	// ListProjects returns at most limit projects of the organization with an identity greater
	// than after, in identity order.
	ListProjects(ctx context.Context, org orgscope.Org, after identity.ProjectID, limit int) ([]identity.ProjectRecord, error)
}

// ReadableProjects returns the projects of the organization that access lets the user read, in
// identity order, at most limit of them. It reads the projects in batches and keeps the readable
// ones, so a user who reads few projects is not shown fewer because other projects came first.
func ReadableProjects(ctx context.Context, store ProjectLister, org orgscope.Org, access ProjectAccess, batch, limit int) ([]identity.ProjectRecord, error) {
	var out []identity.ProjectRecord
	if access.allowNone {
		return out, nil
	}
	var after identity.ProjectID
	for len(out) < limit {
		rows, err := store.ListProjects(ctx, org, after, batch)
		if err != nil {
			return nil, fmt.Errorf("list projects: %w", err)
		}
		for i := range rows {
			if len(out) < limit && access.CanRead(rows[i].ID.String()) {
				out = append(out, rows[i])
			}
		}
		// A user limited to named projects has them all once each was found; no later row can add one.
		if len(rows) < batch || (!access.everyone && len(out) == len(access.projects)) {
			break
		}
		after = rows[len(rows)-1].ID
	}
	return out, nil
}
