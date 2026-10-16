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
	// OwnerEmail is the address of an existing account; the account becomes the Owner of the
	// organization, in the same transaction that creates it.
	OwnerEmail string
}

// OrgKeyIssuer prepares the first key of a new organization, wrapped by the master key, without
// storing it: the store writes it in the transaction that creates the organization.
type OrgKeyIssuer interface {
	NewOrgKey(ctx context.Context, org OrgID) (OrgKeyRecord, error)
}

// Organizations creates organizations for the operator's command line. The creation, the
// organization's key, the administrator's membership and role binding, and the audit entries
// are one transaction; the installation acts as the actor (SYSTEM principal).
type Organizations struct {
	store OrgCreateStore
	keys  OrgKeyIssuer
	clock clock.Clock
	ids   id.Generator
}

// NewOrganizations returns the organization service over its ports.
func NewOrganizations(store OrgCreateStore, keys OrgKeyIssuer, clk clock.Clock, ids id.Generator) *Organizations {
	return &Organizations{store: store, keys: keys, clock: clk, ids: ids}
}

// Create creates the organization and makes the account of OwnerEmail its Owner, the role every
// organization creator holds (ADR-0038: the first user of an organization is its Owner). A repeated
// slug fails with [ErrConflict]; an unknown address with [ErrNotFound].
func (o *Organizations) Create(ctx context.Context, in NewOrganization) (OrgRecord, error) {
	if err := validateNewOrganization(in); err != nil {
		return OrgRecord{}, err
	}
	owner, err := o.store.GetUserByEmail(ctx, NormalizeEmail(in.OwnerEmail))
	if err != nil {
		return OrgRecord{}, fmt.Errorf("create organization: owner: %w", err)
	}
	actor, err := installationActor(ctx, o.store, o.clock, o.ids)
	if err != nil {
		return OrgRecord{}, fmt.Errorf("create organization: %w", err)
	}
	now := clock.Micros(o.clock.Now())
	org := OrgRecord{ID: OrgID(o.ids.New()), Name: in.Name, Slug: in.Slug, Status: OrgActive, CreatedAt: now, UpdatedAt: now}
	membership := MembershipRecord{
		ID: MembershipID(o.ids.New()), OrgID: org.ID, UserID: owner.ID, Status: MembershipActive, CreatedAt: now, UpdatedAt: now,
	}
	grant := RoleBindingRecord{
		ID: RoleBindingID(o.ids.New()), OrgID: org.ID, ScopeKind: RoleScopeOrg, UserID: owner.ID, RoleName: RoleOwner,
		CreatedAt: now, UpdatedAt: now,
	}
	key, err := o.keys.NewOrgKey(ctx, org.ID)
	if err != nil {
		return OrgRecord{}, fmt.Errorf("create organization: organization key: %w", err)
	}
	created, err := orgEntry(actor, org.ID, "org.created", now, o.ids,
		jsonvalue.Member{Key: "orgId", Value: jsonvalue.String(org.ID.String())},
		jsonvalue.Member{Key: "slug", Value: jsonvalue.String(org.Slug)})
	if err != nil {
		return OrgRecord{}, err
	}
	joined, err := orgEntry(actor, org.ID, "member.joined", now, o.ids,
		jsonvalue.Member{Key: "userId", Value: jsonvalue.String(owner.ID.String())})
	if err != nil {
		return OrgRecord{}, err
	}
	bound, err := orgEntry(actor, org.ID, "role_binding.created", now, o.ids,
		jsonvalue.Member{Key: "userId", Value: jsonvalue.String(owner.ID.String())},
		jsonvalue.Member{Key: "roleName", Value: jsonvalue.String(RoleOwner)},
		jsonvalue.Member{Key: "scopeKind", Value: jsonvalue.String(RoleScopeOrg)})
	if err != nil {
		return OrgRecord{}, err
	}
	if err := o.store.CreateOrganizationWithAdmin(ctx, org, key, membership, grant, []audit.Entry{created, joined, bound}); err != nil {
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
	case NormalizeEmail(in.OwnerEmail) == "" || !strings.Contains(NormalizeEmail(in.OwnerEmail), "@"):
		return fmt.Errorf("%w: the owner must be an email address", ErrInvalidInput)
	}
	return nil
}
