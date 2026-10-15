// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
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

// Create inserts a project in the organization of the scope. The new project is
// protected (ADR-0038 amendments, SPEC-005). Its handle is the slug; when
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
		err := p.store.CreateProject(ctx, org, rec)
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
