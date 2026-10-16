// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package credential

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/jsonvalue"
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

// Create stores a new credential owned by project, with its first version. The audit entry of the creation is
// stored in the same transaction (R-AUD-02); it names the identity and the type, never a value.
func (s *Service) Create(ctx context.Context, org orgscope.Org, project identity.ProjectID, in Input, actor audit.Actor) (Summary, error) {
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
	by := identity.PrincipalID(actor.PrincipalID)
	ver, err := s.sealVersion(ctx, orgID, in, s.ids.New(), credID, 1, by, now)
	if err != nil {
		return Summary{}, err
	}
	rec := Record{
		ID: credID, OrgID: orgID, ProjectID: project, Name: trimmed(in.Name), ConnectorReference: in.ConnectorReference,
		Status: StatusOK, AllowedHosts: hosts, CurrentVersion: 1, CreatedBy: by, CreatedAt: now, UpdatedAt: now,
		SecretUpdatedAt: now, SecretUpdatedBy: by,
	}
	entry, err := s.entry(org, actor, ActionCreated, now, jsonvalue.Member{Key: "credentialId", Value: jsonvalue.String(credID.String())},
		jsonvalue.Member{Key: "projectId", Value: jsonvalue.String(project.String())},
		jsonvalue.Member{Key: "type", Value: jsonvalue.String(rec.ConnectorReference)})
	if err != nil {
		return Summary{}, err
	}
	if err := s.store.CreateCredential(ctx, org, rec, ver, entry); err != nil {
		return Summary{}, fmt.Errorf("create credential: %w", err)
	}
	return summaryOf(rec, project, copyFields(in.Public)), nil
}

// Replace stores a new version of the secret and public fields. Only the owning project may replace; the previous
// versions are kept (R-CRED-02). A credential that was waiting for its secret becomes OK. The audit entry names
// the change and the new version number; it never names a field value.
func (s *Service) Replace(ctx context.Context, org orgscope.Org, project identity.ProjectID, credential id.UUID, in Input, actor audit.Actor) (Summary, error) {
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
	by := identity.PrincipalID(actor.PrincipalID)
	ver, err := s.sealVersion(ctx, rec.OrgID, in, s.ids.New(), rec.ID, next, by, now)
	if err != nil {
		return Summary{}, err
	}
	status := rec.Status
	if status == StatusNeedsSecret || status == StatusNeedsReauth {
		status = StatusOK
	}
	entry, err := s.entry(org, actor, ActionUpdated, now, jsonvalue.Member{Key: "credentialId", Value: jsonvalue.String(rec.ID.String())},
		jsonvalue.Member{Key: "changedFields", Value: jsonvalue.Array(jsonvalue.String(FieldSecret))},
		jsonvalue.Member{Key: "version", Value: jsonvalue.Int(int64(next))})
	if err != nil {
		return Summary{}, err
	}
	if err := s.store.AddVersion(ctx, org, rec.ID, rec.CurrentVersion, ver, status, now, entry); err != nil {
		return Summary{}, fmt.Errorf("replace credential: %w", err)
	}
	fresh, err := s.store.GetCredential(ctx, org, rec.ID)
	if err != nil {
		return Summary{}, fmt.Errorf("read credential: %w", err)
	}
	return summaryOf(fresh, project, copyFields(in.Public)), nil
}

// Update changes the name and the allowed hosts of a credential. Only the owning project may update it. The audit
// entry names the fields that changed.
//
// The argument expected is the update time the caller saw when it checked its If-Match revision. The write itself
// compares it, so two changes that carry the same revision cannot both succeed: the second gets [ErrConflict]. The
// zero time means the state this call just read.
func (s *Service) Update(ctx context.Context, org orgscope.Org, project identity.ProjectID, credential id.UUID, set Settings, expected time.Time, actor audit.Actor) (Summary, error) {
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
	if expected.IsZero() {
		expected = rec.UpdatedAt
	}
	// The stores keep microseconds, so the revision is compared at that precision.
	now := s.clock.Now().Truncate(time.Microsecond)
	if !now.After(rec.UpdatedAt) {
		// The update time is the revision of the credential: it moves forward with every change, even when the clock
		// reads the same microsecond or went back, so a second change carrying the old revision is refused.
		now = rec.UpdatedAt.Add(time.Microsecond)
	}
	changed := changedSettings(rec, trimmed(set.Name), hosts)
	rec.Name = trimmed(set.Name)
	rec.AllowedHosts = hosts
	rec.UpdatedAt = now
	entry, err := s.entry(org, actor, ActionUpdated, now, jsonvalue.Member{Key: "credentialId", Value: jsonvalue.String(rec.ID.String())},
		jsonvalue.Member{Key: "changedFields", Value: stringArray(changed)})
	if err != nil {
		return Summary{}, err
	}
	if err := s.store.UpdateCredential(ctx, org, rec, expected, entry); err != nil {
		return Summary{}, fmt.Errorf("update credential: %w", err)
	}
	return s.Get(ctx, org, project, credential)
}

// Delete removes a credential, its versions and its shares. Only the owning project may delete it. The argument
// expected has the meaning it has in [Service.Update]: the delete is refused with [ErrConflict] when the credential
// changed since.
func (s *Service) Delete(ctx context.Context, org orgscope.Org, project identity.ProjectID, credential id.UUID, expected time.Time, actor audit.Actor) error {
	rec, err := s.owned(ctx, org, project, credential)
	if err != nil {
		return err
	}
	if expected.IsZero() {
		expected = rec.UpdatedAt
	}
	entry, err := s.entry(org, actor, ActionDeleted, s.clock.Now(), jsonvalue.Member{Key: "credentialId", Value: jsonvalue.String(rec.ID.String())},
		jsonvalue.Member{Key: "projectId", Value: jsonvalue.String(project.String())})
	if err != nil {
		return err
	}
	if err := s.store.DeleteCredential(ctx, org, project, credential, expected, entry); err != nil {
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
func (s *Service) Share(ctx context.Context, org orgscope.Org, project identity.ProjectID, credential id.UUID, target identity.ProjectID, actor audit.Actor) error {
	rec, err := s.owned(ctx, org, project, credential)
	if err != nil {
		return err
	}
	if target == project {
		return fmt.Errorf("%w: the owning project already uses the credential", ErrInvalid)
	}
	now := s.clock.Now()
	sh := Share{ID: s.ids.New(), OrgID: rec.OrgID, CredentialID: rec.ID, ProjectID: target, GrantedBy: identity.PrincipalID(actor.PrincipalID), CreatedAt: now}
	entry, err := s.entry(org, actor, ActionShared, now, shareMembers(rec.ID, target)...)
	if err != nil {
		return err
	}
	if err := s.store.CreateShare(ctx, org, sh, entry); err != nil {
		return fmt.Errorf("share credential: %w", err)
	}
	return nil
}

// Unshare removes a share. The next dispatch of the target project fails with [ErrNotUsable].
func (s *Service) Unshare(ctx context.Context, org orgscope.Org, project identity.ProjectID, credential id.UUID, target identity.ProjectID, actor audit.Actor) error {
	if _, err := s.owned(ctx, org, project, credential); err != nil {
		return err
	}
	entry, err := s.entry(org, actor, ActionUnshared, s.clock.Now(), shareMembers(credential, target)...)
	if err != nil {
		return err
	}
	if err := s.store.DeleteShare(ctx, org, credential, target, entry); err != nil {
		return fmt.Errorf("unshare credential: %w", err)
	}
	return nil
}

// OwnerProject returns the project that owns a credential of the organization. It is the authorization resolver of
// a request that names a credential (authz.OwnerResolver): an identifier that is not a credential of org is
// [identity.ErrNotFound], so the caller answers as if the credential did not exist.
func (s *Service) OwnerProject(ctx context.Context, org orgscope.Org, credential string) (identity.ProjectID, error) {
	cid, err := id.Parse(credential)
	if err != nil {
		return identity.ProjectID{}, identity.ErrNotFound
	}
	rec, err := s.store.GetCredential(ctx, org, cid)
	if errors.Is(err, ErrNotFound) {
		return identity.ProjectID{}, identity.ErrNotFound
	}
	if err != nil {
		return identity.ProjectID{}, fmt.Errorf("owner of credential: %w", err)
	}
	return rec.ProjectID, nil
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

// Action names of the audit entries of credentials (Annex B of SPEC-004).
const (
	ActionCreated  = "credential.created"
	ActionUpdated  = "credential.updated"
	ActionDeleted  = "credential.deleted"
	ActionShared   = "credential.shared"
	ActionUnshared = "credential.unshared"
)

// FieldSecret names the secret in the changed fields of an audit entry; no field name or value is ever recorded.
const FieldSecret = "secret"

// entry builds the audit entry of one change of a credential. The entry carries the actor, the action and the
// members it is given, which never hold a secret value.
func (s *Service) entry(org orgscope.Org, actor audit.Actor, action string, at time.Time, members ...jsonvalue.Member) (audit.Entry, error) {
	d, err := jsonvalue.Object(members...)
	if err != nil {
		return audit.Entry{}, fmt.Errorf("%s: details: %w", action, err)
	}
	e, err := audit.NewEntry(audit.EntryParams{
		ID: s.ids.New(), Chain: audit.OrganizationChain(org.UUID()), OccurredAt: at,
		Actor: actor, Action: action, Outcome: audit.OutcomeSuccess, Details: d,
	})
	if err != nil {
		return audit.Entry{}, fmt.Errorf("%s: audit: %w", action, err)
	}
	return e, nil
}

// shareMembers are the details of a share change: the credential and the project that gains or loses its use.
func shareMembers(credential id.UUID, target identity.ProjectID) []jsonvalue.Member {
	return []jsonvalue.Member{
		{Key: "credentialId", Value: jsonvalue.String(credential.String())},
		{Key: "projectId", Value: jsonvalue.String(target.String())},
	}
}

// changedSettings lists the settings a change alters, in a fixed order.
func changedSettings(rec Record, name string, hosts []string) []string {
	var out []string
	if rec.Name != name {
		out = append(out, "name")
	}
	if !slices.Equal(rec.AllowedHosts, hosts) {
		out = append(out, "allowedHosts")
	}
	return out
}

// stringArray is a JSON array of the given strings.
func stringArray(items []string) jsonvalue.Value {
	values := make([]jsonvalue.Value, 0, len(items))
	for _, it := range items {
		values = append(values, jsonvalue.String(it))
	}
	return jsonvalue.Array(values...)
}
