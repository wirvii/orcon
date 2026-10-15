// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
)

// Bounds of the organization fields the operator sets (SPEC-016 SH4, the orgs create command).
// The slug is lowercase letters, digits and single hyphens, so it reads well in a URL.
const (
	maxOrgName = 200
	maxOrgSlug = 100
)

var orgSlugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// OrgCreateStore is what the organization service needs from storage.
type OrgCreateStore interface {
	OrgStore
	UserStore
	PrincipalStore
}

// NewOrganization is what an operator supplies to create an organization.
type NewOrganization struct {
	Name string
	Slug string
	// AdminEmail is the address of an existing account; the account becomes the Admin of the
	// organization, in the same transaction that creates it.
	AdminEmail string
}

// Organizations creates organizations for the operator's command line. The creation, the
// administrator's membership and role binding, and the audit entries are one transaction; the
// installation acts as the actor (SYSTEM principal).
type Organizations struct {
	store OrgCreateStore
	clock clock.Clock
	ids   id.Generator
}

// NewOrganizations returns the organization service over its ports.
func NewOrganizations(store OrgCreateStore, clk clock.Clock, ids id.Generator) *Organizations {
	return &Organizations{store: store, clock: clk, ids: ids}
}

// Create creates the organization and makes the account of AdminEmail its Admin. A repeated
// slug fails with [ErrConflict]; an unknown address with [ErrNotFound].
func (o *Organizations) Create(ctx context.Context, in NewOrganization) (OrgRecord, error) {
	if err := validateNewOrganization(in); err != nil {
		return OrgRecord{}, err
	}
	admin, err := o.store.GetUserByEmail(ctx, NormalizeEmail(in.AdminEmail))
	if err != nil {
		return OrgRecord{}, fmt.Errorf("create organization: administrator: %w", err)
	}
	actor, err := installationActor(ctx, o.store, o.clock, o.ids)
	if err != nil {
		return OrgRecord{}, fmt.Errorf("create organization: %w", err)
	}
	now := clock.Micros(o.clock.Now())
	org := OrgRecord{ID: OrgID(o.ids.New()), Name: in.Name, Slug: in.Slug, Status: OrgActive, CreatedAt: now, UpdatedAt: now}
	membership := MembershipRecord{
		ID: MembershipID(o.ids.New()), OrgID: org.ID, UserID: admin.ID, Status: MembershipActive, CreatedAt: now, UpdatedAt: now,
	}
	grant := RoleBindingRecord{
		ID: RoleBindingID(o.ids.New()), OrgID: org.ID, ScopeKind: RoleScopeOrg, UserID: admin.ID, RoleName: RoleAdmin,
		CreatedAt: now, UpdatedAt: now,
	}
	created, err := orgEntry(actor, org.ID, "org.created", now, o.ids,
		jsonvalue.Member{Key: "orgId", Value: jsonvalue.String(org.ID.String())},
		jsonvalue.Member{Key: "slug", Value: jsonvalue.String(org.Slug)})
	if err != nil {
		return OrgRecord{}, err
	}
	joined, err := orgEntry(actor, org.ID, "member.joined", now, o.ids,
		jsonvalue.Member{Key: "userId", Value: jsonvalue.String(admin.ID.String())})
	if err != nil {
		return OrgRecord{}, err
	}
	bound, err := orgEntry(actor, org.ID, "role_binding.created", now, o.ids,
		jsonvalue.Member{Key: "userId", Value: jsonvalue.String(admin.ID.String())},
		jsonvalue.Member{Key: "roleName", Value: jsonvalue.String(RoleAdmin)},
		jsonvalue.Member{Key: "scopeKind", Value: jsonvalue.String(RoleScopeOrg)})
	if err != nil {
		return OrgRecord{}, err
	}
	if err := o.store.CreateOrganizationWithAdmin(ctx, org, membership, grant, []audit.Entry{created, joined, bound}); err != nil {
		return OrgRecord{}, fmt.Errorf("create organization: %w", err)
	}
	return org, nil
}

// orgEntry builds an audit entry of the organization chain, stamped with the given instant.
func orgEntry(actor audit.Actor, org OrgID, action string, at time.Time, ids id.Generator, members ...jsonvalue.Member) (audit.Entry, error) {
	details, err := jsonvalue.Object(members...)
	if err != nil {
		return audit.Entry{}, fmt.Errorf("audit %s: %w", action, err)
	}
	e, err := audit.NewEntry(audit.EntryParams{
		ID: ids.New(), Chain: audit.OrganizationChain(id.UUID(org)), OccurredAt: at, Actor: actor,
		Action: action, Outcome: audit.OutcomeSuccess, Details: details,
	})
	if err != nil {
		return audit.Entry{}, fmt.Errorf("audit %s: %w", action, err)
	}
	return e, nil
}

func validateNewOrganization(in NewOrganization) error {
	switch {
	case in.Name == "" || utf8.RuneCountInString(in.Name) > maxOrgName:
		return fmt.Errorf("%w: organization name must have 1 to %d characters", ErrInvalidInput, maxOrgName)
	case len(in.Slug) == 0 || len(in.Slug) > maxOrgSlug || !orgSlugPattern.MatchString(in.Slug):
		return fmt.Errorf("%w: organization slug must be 1 to %d lowercase letters, digits and single hyphens", ErrInvalidInput, maxOrgSlug)
	case NormalizeEmail(in.AdminEmail) == "" || !strings.Contains(NormalizeEmail(in.AdminEmail), "@"):
		return fmt.Errorf("%w: the administrator must be an email address", ErrInvalidInput)
	}
	return nil
}
