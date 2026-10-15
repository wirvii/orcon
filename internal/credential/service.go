// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package credential

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// Service is the credential store of SPEC-004 SE7 and the production [Resolver] of dispatch. It enforces the rules
// that do not depend on the storage: the owner-only changes, the sharing rule, the write-only secrets and the host
// binding. Every method takes the organization scope and the acting project.
type Service struct {
	store Store
	keys  DataKeys
	clock clock.Clock
	ids   id.Generator
}

// Config holds the collaborators of a [Service]. Every field is required.
type Config struct {
	Store Store
	Keys  DataKeys
	Clock clock.Clock
	IDs   id.Generator
}

// New builds a service. It fails when a collaborator is missing.
func New(cfg Config) (*Service, error) {
	switch {
	case cfg.Store == nil:
		return nil, errors.New("credential: a store is required")
	case cfg.Keys == nil:
		return nil, errors.New("credential: a key service is required")
	case cfg.Clock == nil:
		return nil, errors.New("credential: a clock is required")
	case cfg.IDs == nil:
		return nil, errors.New("credential: an identity generator is required")
	}
	return &Service{store: cfg.Store, keys: cfg.Keys, clock: cfg.Clock, ids: cfg.IDs}, nil
}

// Create stores a new credential owned by project, with its first version.
func (s *Service) Create(ctx context.Context, org orgscope.Org, project identity.ProjectID, in Input, by identity.PrincipalID) (Summary, error) {
	if err := validateAll(in); err != nil {
		return Summary{}, err
	}
	hosts, err := normalizeHosts(in.AllowedHosts)
	if err != nil {
		return Summary{}, err
	}
	now := s.clock.Now()
	orgID := identity.OrgIDOf(org)
	credID := s.ids.New()
	ver, err := s.sealVersion(ctx, orgID, in, s.ids.New(), credID, 1, by, now)
	if err != nil {
		return Summary{}, err
	}
	rec := Record{
		ID: credID, OrgID: orgID, ProjectID: project, Name: trimmed(in.Name), ConnectorReference: in.ConnectorReference,
		Status: StatusOK, AllowedHosts: hosts, CurrentVersion: 1, CreatedBy: by, CreatedAt: now, UpdatedAt: now,
		SecretUpdatedAt: now, SecretUpdatedBy: by,
	}
	if err := s.store.CreateCredential(ctx, org, rec, ver); err != nil {
		return Summary{}, fmt.Errorf("create credential: %w", err)
	}
	return summaryOf(rec, project, copyFields(in.Public)), nil
}

// Replace stores a new version of the secret and public fields. Only the owning project may replace; the previous
// versions are kept (R-CRED-02). A credential that was waiting for its secret becomes OK.
func (s *Service) Replace(ctx context.Context, org orgscope.Org, project identity.ProjectID, credential id.UUID, in Input, by identity.PrincipalID) (Summary, error) {
	if len(in.Secret) == 0 {
		return Summary{}, fmt.Errorf("%w: a new version needs at least one secret field", ErrInvalid)
	}
	if err := validateFields(in.Public, in.Secret); err != nil {
		return Summary{}, err
	}
	rec, err := s.owned(ctx, org, project, credential)
	if err != nil {
		return Summary{}, err
	}
	now := s.clock.Now()
	next := rec.CurrentVersion + 1
	ver, err := s.sealVersion(ctx, rec.OrgID, in, s.ids.New(), rec.ID, next, by, now)
	if err != nil {
		return Summary{}, err
	}
	status := rec.Status
	if status == StatusNeedsSecret || status == StatusNeedsReauth {
		status = StatusOK
	}
	if err := s.store.AddVersion(ctx, org, rec.ID, rec.CurrentVersion, ver, status, now); err != nil {
		return Summary{}, fmt.Errorf("replace credential: %w", err)
	}
	fresh, err := s.store.GetCredential(ctx, org, rec.ID)
	if err != nil {
		return Summary{}, fmt.Errorf("read credential: %w", err)
	}
	return summaryOf(fresh, project, copyFields(in.Public)), nil
}

// Update changes the name and the allowed hosts of a credential. Only the owning project may update it.
func (s *Service) Update(ctx context.Context, org orgscope.Org, project identity.ProjectID, credential id.UUID, set Settings) (Summary, error) {
	if err := validateName(set.Name); err != nil {
		return Summary{}, err
	}
	hosts, err := normalizeHosts(set.AllowedHosts)
	if err != nil {
		return Summary{}, err
	}
	rec, err := s.owned(ctx, org, project, credential)
	if err != nil {
		return Summary{}, err
	}
	rec.Name = trimmed(set.Name)
	rec.AllowedHosts = hosts
	rec.UpdatedAt = s.clock.Now()
	if err := s.store.UpdateCredential(ctx, org, rec); err != nil {
		return Summary{}, fmt.Errorf("update credential: %w", err)
	}
	return s.Get(ctx, org, project, credential)
}

// Delete removes a credential, its versions and its shares. Only the owning project may delete it.
func (s *Service) Delete(ctx context.Context, org orgscope.Org, project identity.ProjectID, credential id.UUID) error {
	if _, err := s.owned(ctx, org, project, credential); err != nil {
		return err
	}
	if err := s.store.DeleteCredential(ctx, org, project, credential); err != nil {
		return fmt.Errorf("delete credential: %w", err)
	}
	return nil
}

// Get returns one credential the project owns or holds a share of. The secret is never in the result; only whether
// one is set and when and by whom it was written (R-CRED-01).
func (s *Service) Get(ctx context.Context, org orgscope.Org, project identity.ProjectID, credential id.UUID) (Summary, error) {
	rec, err := s.readable(ctx, org, project, credential)
	if err != nil {
		return Summary{}, err
	}
	ver, err := s.store.GetCredentialVersion(ctx, org, rec.ID, rec.CurrentVersion)
	if err != nil {
		return Summary{}, fmt.Errorf("read credential version: %w", err)
	}
	public, err := decodeFields(ver.PublicFields)
	if err != nil {
		return Summary{}, err
	}
	return summaryOf(rec, project, public), nil
}

// List returns the credentials the project owns or holds a share of, without secrets and public fields.
func (s *Service) List(ctx context.Context, org orgscope.Org, project identity.ProjectID) ([]Summary, error) {
	recs, err := s.store.ListCredentials(ctx, org, project)
	if err != nil {
		return nil, fmt.Errorf("list credentials: %w", err)
	}
	out := make([]Summary, 0, len(recs))
	for i := range recs {
		out = append(out, summaryOf(recs[i], project, nil))
	}
	return out, nil
}

// Share lets target use the credential. The owner only; a project never shares with itself, and a share never
// crosses organizations (the store's composite keys refuse it, and an unknown project is ErrNotFound).
func (s *Service) Share(ctx context.Context, org orgscope.Org, project identity.ProjectID, credential id.UUID, target identity.ProjectID, by identity.PrincipalID) error {
	rec, err := s.owned(ctx, org, project, credential)
	if err != nil {
		return err
	}
	if target == project {
		return fmt.Errorf("%w: the owning project already uses the credential", ErrInvalid)
	}
	sh := Share{ID: s.ids.New(), OrgID: rec.OrgID, CredentialID: rec.ID, ProjectID: target, GrantedBy: by, CreatedAt: s.clock.Now()}
	if err := s.store.CreateShare(ctx, org, sh); err != nil {
		return fmt.Errorf("share credential: %w", err)
	}
	return nil
}

// Unshare removes a share. The next dispatch of the target project fails with [ErrNotUsable].
func (s *Service) Unshare(ctx context.Context, org orgscope.Org, project identity.ProjectID, credential id.UUID, target identity.ProjectID) error {
	if _, err := s.owned(ctx, org, project, credential); err != nil {
		return err
	}
	if err := s.store.DeleteShare(ctx, org, credential, target); err != nil {
		return fmt.Errorf("unshare credential: %w", err)
	}
	return nil
}

// Resolve returns the current version with its secret decrypted, for one node execution of project. It reads the
// share and the status on every call (R-AZ-07, ADR-0043 section 5), so a removed share fails the next dispatch.
func (s *Service) Resolve(ctx context.Context, org orgscope.Org, project identity.ProjectID, credential id.UUID) (Resolved, error) {
	rec, err := s.store.GetCredential(ctx, org, credential)
	if err != nil {
		return Resolved{}, fmt.Errorf("resolve credential: %w", err)
	}
	if err := s.mayUse(ctx, org, rec, project); err != nil {
		return Resolved{}, err
	}
	if rec.Status != StatusOK {
		return Resolved{}, fmt.Errorf("%w: the credential status is %s", ErrNotUsable, rec.Status)
	}
	ver, err := s.store.GetCredentialVersion(ctx, org, rec.ID, rec.CurrentVersion)
	if err != nil {
		return Resolved{}, fmt.Errorf("resolve credential version: %w", err)
	}
	public, err := decodeFields(ver.PublicFields)
	if err != nil {
		return Resolved{}, err
	}
	secret, err := s.openSecret(ctx, ver)
	if err != nil {
		return Resolved{}, err
	}
	return Resolved{ID: rec.ID, Name: rec.Name, Version: ver.Number, Public: public, hosts: rec.AllowedHosts, secret: secret}, nil
}

// owned returns the credential when project owns it. A credential the project only uses is ErrNotOwner; one it cannot
// see at all is ErrNotFound.
func (s *Service) owned(ctx context.Context, org orgscope.Org, project identity.ProjectID, credential id.UUID) (Record, error) {
	rec, err := s.store.GetCredential(ctx, org, credential)
	if err != nil {
		return Record{}, fmt.Errorf("read credential: %w", err)
	}
	if rec.ProjectID == project {
		return rec, nil
	}
	if _, err := s.store.GetShare(ctx, org, credential, project); err == nil {
		return Record{}, ErrNotOwner
	}
	return Record{}, ErrNotFound
}

// readable returns the credential when project owns it or holds a share of it; otherwise ErrNotFound.
func (s *Service) readable(ctx context.Context, org orgscope.Org, project identity.ProjectID, credential id.UUID) (Record, error) {
	rec, err := s.store.GetCredential(ctx, org, credential)
	if err != nil {
		return Record{}, fmt.Errorf("read credential: %w", err)
	}
	if rec.ProjectID == project {
		return rec, nil
	}
	if _, err := s.store.GetShare(ctx, org, credential, project); err != nil {
		return Record{}, ErrNotFound
	}
	return rec, nil
}

// mayUse returns nil when project owns the credential or holds a share of it, else ErrNotUsable.
func (s *Service) mayUse(ctx context.Context, org orgscope.Org, rec Record, project identity.ProjectID) error {
	if rec.ProjectID == project {
		return nil
	}
	if _, err := s.store.GetShare(ctx, org, rec.ID, project); err != nil {
		return fmt.Errorf("%w: no share with this project", ErrNotUsable)
	}
	return nil
}

// summaryOf builds the reader's view of a record; public is nil when the caller did not read the fields.
func summaryOf(rec Record, project identity.ProjectID, public map[string]string) Summary {
	return Summary{
		Record: rec,
		Owned:  rec.ProjectID == project,
		Secret: SecretState{Set: rec.CurrentVersion > 0, UpdatedAt: rec.SecretUpdatedAt, UpdatedBy: rec.SecretUpdatedBy},
		Public: public,
	}
}

// validateAll checks a whole input for creation.
func validateAll(in Input) error {
	if err := validateName(in.Name); err != nil {
		return err
	}
	if err := validateConnector(in.ConnectorReference); err != nil {
		return err
	}
	if len(in.Secret) == 0 {
		return fmt.Errorf("%w: a credential needs at least one secret field", ErrInvalid)
	}
	return validateFields(in.Public, in.Secret)
}

// trimmed returns the name without surrounding space.
func trimmed(name string) string { return strings.TrimSpace(name) }

// copyFields returns a copy of a field map, or nil.
func copyFields(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
