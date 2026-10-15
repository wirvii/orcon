// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// Length bounds of the project fields. They mirror the CHECK constraints of the
// project table; no Accepted document fixes the character set of a slug, so the
// format beyond these bounds is for the API layer to decide (SPEC-004 R-ID-11,
// owner ruling on SPEC-001 C2a).
const (
	maxProjectName        = 200
	maxProjectSlug        = 100
	maxProjectEnvironment = 100
)

// NewProject is what a caller supplies to create a project.
type NewProject struct {
	Name string
	// Slug is unique within the organization and the default project handle.
	Slug string
	// Environment is an optional free label.
	Environment string
	// By is the principal that creates the project; it is the actor of the audit entry.
	By audit.Actor
	// Creator is the user who creates the project, and who receives the Project Admin role in
	// the project in the same transaction (SPEC-016 Q3 = A). The zero value grants nothing: the
	// caller leaves it zero for a user who already reads every project (an Owner, an Admin or
	// any holder of project:read-all).
	Creator UserID
}

// Projects creates projects and assigns their handles.
type Projects struct {
	store ProjectStore
	orgs  OrgStore
	clock clock.Clock
	ids   id.Generator
}

// NewProjects returns the project service over its stores.
func NewProjects(store ProjectStore, orgs OrgStore, clk clock.Clock, ids id.Generator) *Projects {
	return &Projects{store: store, orgs: orgs, clock: clk, ids: ids}
}

// Create inserts a project in the organization of the scope. The new project is always
// protected (ADR-0059 section 5, SPEC-016 SH-PRJ-1): no input can create it unprotected. Its handle is the slug; when
// another project of the instance holds it, the handle is "<orgSlug>-<slug>"; when
// that is taken too, Create fails with [ErrHandleUnavailable], whose text names
// nothing of anyone else (SPEC-004 R-ID-11).
func (p *Projects) Create(ctx context.Context, org orgscope.Org, in NewProject) (ProjectRecord, error) {
	if err := validateNewProject(in); err != nil {
		return ProjectRecord{}, err
	}
	orgRecord, err := p.orgs.GetOrganization(ctx, org)
	if err != nil {
		return ProjectRecord{}, fmt.Errorf("create project: %w", err)
	}
	now := p.clock.Now()
	rec := ProjectRecord{
		ID: ProjectID(p.ids.New()), OrgID: OrgIDOf(org), Name: in.Name, Slug: in.Slug,
		Environment: in.Environment, IsProtected: true, CreatedAt: now, UpdatedAt: now,
	}
	for _, handle := range handleCandidates(orgRecord.Slug, in.Slug) {
		rec.Handle = handle
		entry, err := audit.NewEntry(audit.EntryParams{
			ID: p.ids.New(), Chain: audit.OrganizationChain(id.UUID(rec.OrgID)), OccurredAt: now,
			Actor: in.By, Action: "project.created", Outcome: audit.OutcomeSuccess,
			Details: projectDetails(rec),
		})
		if err != nil {
			return ProjectRecord{}, fmt.Errorf("create project: audit: %w", err)
		}
		err = p.createIn(ctx, org, rec, in.Creator, entry, in.By, now)
		if err == nil {
			return rec, nil
		}
		if !errors.Is(err, ErrHandleTaken) {
			return ProjectRecord{}, fmt.Errorf("create project: %w", err)
		}
	}
	// The cause is dropped on purpose: a driver error would quote the handle.
	return ProjectRecord{}, ErrHandleUnavailable
}

// createIn stores the project. With a creator, the Project Admin binding of the creator and its
// role_binding.created entry are written in the same transaction as the project and its entry.
func (p *Projects) createIn(ctx context.Context, org orgscope.Org, rec ProjectRecord, creator UserID, entry audit.Entry, by audit.Actor, now time.Time) error {
	if creator.IsZero() {
		return p.store.CreateProject(ctx, org, rec, entry)
	}
	grant := RoleBindingRecord{
		ID: RoleBindingID(p.ids.New()), OrgID: rec.OrgID, ScopeKind: RoleScopeProject, ProjectID: rec.ID,
		UserID: creator, RoleName: RoleProjectAdmin, CreatedAt: now, UpdatedAt: now,
	}
	bound, err := audit.NewEntry(audit.EntryParams{
		ID: p.ids.New(), Chain: audit.OrganizationChain(id.UUID(rec.OrgID)), OccurredAt: now,
		Actor: by, Action: "role_binding.created", Outcome: audit.OutcomeSuccess,
		Details: grantDetails(grant),
	})
	if err != nil {
		return fmt.Errorf("audit: %w", err)
	}
	return p.store.CreateProjectWithGrant(ctx, org, rec, grant, []audit.Entry{entry, bound})
}

// grantDetails are the fields of a role_binding.created entry: who holds which role where.
func grantDetails(b RoleBindingRecord) jsonvalue.Value {
	members := []jsonvalue.Member{
		{Key: "userId", Value: jsonvalue.String(b.UserID.String())},
		{Key: "roleName", Value: jsonvalue.String(b.RoleName)},
		{Key: "scopeKind", Value: jsonvalue.String(b.ScopeKind)},
	}
	if b.ScopeKind == RoleScopeProject {
		members = append(members, jsonvalue.Member{Key: "projectId", Value: jsonvalue.String(b.ProjectID.String())})
	}
	v, err := jsonvalue.Object(members...)
	if err != nil {
		return jsonvalue.Value{}
	}
	return v
}

// projectDetails are the fields of the project.created entry: the identity and the
// handle of the project, never anything the operator typed as free text (R-AUD-08).
func projectDetails(p ProjectRecord) jsonvalue.Value {
	v, err := jsonvalue.Object(
		jsonvalue.Member{Key: "projectId", Value: jsonvalue.String(p.ID.String())},
		jsonvalue.Member{Key: "handle", Value: jsonvalue.String(p.Handle)},
	)
	if err != nil {
		return jsonvalue.Value{}
	}
	return v
}

// handleCandidates lists the handles to try, in order: the slug, then the slug
// prefixed with the organization slug.
func handleCandidates(orgSlug, slug string) []string {
	return []string{slug, orgSlug + "-" + slug}
}

func validateNewProject(in NewProject) error {
	switch {
	case in.Name == "" || utf8.RuneCountInString(in.Name) > maxProjectName:
		return fmt.Errorf("%w: project name must have 1 to %d characters", ErrInvalidInput, maxProjectName)
	case in.Slug == "" || utf8.RuneCountInString(in.Slug) > maxProjectSlug:
		return fmt.Errorf("%w: project slug must have 1 to %d characters", ErrInvalidInput, maxProjectSlug)
	case utf8.RuneCountInString(in.Environment) > maxProjectEnvironment:
		return fmt.Errorf("%w: project environment must have at most %d characters", ErrInvalidInput, maxProjectEnvironment)
	}
	return nil
}

// ProjectFilter selects projects of a listing (SPEC-005 listProjects). A zero field matches everything.
type ProjectFilter struct {
	// NamePrefix keeps projects whose name starts with it, ignoring case.
	NamePrefix string
	// Environment keeps projects with exactly this environment label.
	Environment string
	// Handle keeps the project with exactly this handle.
	Handle string
}

// Match reports whether the project passes the filter.
func (f ProjectFilter) Match(p ProjectRecord) bool {
	return strings.HasPrefix(strings.ToLower(p.Name), strings.ToLower(f.NamePrefix)) &&
		(f.Environment == "" || p.Environment == f.Environment) &&
		(f.Handle == "" || p.Handle == f.Handle)
}
