// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wirvii/orcon/internal/adapter/postgres/sqlcgen"
	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/credential"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// credentialRowLimit bounds the credentials one listing returns.
const credentialRowLimit = 1000

// The credential store (SPEC-004 SE7, ADR-0043): the adapter behind credential.Store. Every statement runs in a
// transaction that binds the organization of its scope, so row-level security keeps other organizations out.
var _ credential.Store = (*Store)(nil)

// CreateCredential implements [credential.Store].
func (s *Store) CreateCredential(ctx context.Context, org orgscope.Org, rec credential.Record, first credential.Version, entry audit.Entry) error {
	err := s.inScopeRecord(ctx, "create credential", org, rec.OrgID, func(ctx context.Context, q *sqlcgen.Queries) error {
		hosts, err := credential.EncodeHosts(rec.AllowedHosts)
		if err != nil {
			return err
		}
		if err := q.InsertCredential(ctx, sqlcgen.InsertCredentialParams{
			ID: pgUUID(rec.ID), OrgID: pgUUID(rec.OrgID), ProjectID: pgUUID(rec.ProjectID),
			Name: rec.Name, ConnectorReference: rec.ConnectorReference, Status: string(rec.Status),
			AllowedHostDocument: hosts, AllowedHostDocumentFormat: credential.Format,
			CurrentVersionNumber: int64(rec.CurrentVersion), CreatedByPrincipalID: nullPGUUID(rec.CreatedBy),
			CreatedAt: timestamp(rec.CreatedAt), UpdatedAt: timestamp(rec.UpdatedAt),
		}); err != nil {
			return err
		}
		if err := insertCredentialVersionPG(ctx, q, first); err != nil {
			return err
		}
		return insertPendingOrg(ctx, q, org, entry)
	})
	return credErr(err)
}

// GetCredential implements [credential.Store].
func (s *Store) GetCredential(ctx context.Context, org orgscope.Org, cred id.UUID) (credential.Record, error) {
	var out credential.Record
	err := s.inScope(ctx, "get credential", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetCredential(ctx, pgUUID(identity.OrgIDOf(org)), pgUUID(cred))
		if errors.Is(err, pgx.ErrNoRows) {
			return credential.ErrNotFound
		}
		if err != nil {
			return err
		}
		out, err = recordOfPG(row.ID, row.OrgID, row.ProjectID, row.Name, row.ConnectorReference, row.Status,
			row.AllowedHostDocument, row.CreatedByPrincipalID, row.CreatedAt, row.UpdatedAt, row.SecretSetAt, row.SecretSetByPrincipalID,
			row.CurrentVersionNumber)
		return err
	})
	return out, credErr(err)
}

// ListCredentials implements [credential.Store].
func (s *Store) ListCredentials(ctx context.Context, org orgscope.Org, project identity.ProjectID) ([]credential.Record, error) {
	out := []credential.Record{}
	err := s.inScope(ctx, "list credentials", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListCredentialsOfProject(ctx, pgUUID(identity.OrgIDOf(org)), pgUUID(project), clampPage(credentialRowLimit))
		if err != nil {
			return err
		}
		for i := range rows {
			row := rows[i]
			rec, err := recordOfPG(row.ID, row.OrgID, row.ProjectID, row.Name, row.ConnectorReference, row.Status,
				row.AllowedHostDocument, row.CreatedByPrincipalID, row.CreatedAt, row.UpdatedAt, row.SecretSetAt, row.SecretSetByPrincipalID,
				row.CurrentVersionNumber)
			if err != nil {
				return err
			}
			out = append(out, rec)
		}
		return nil
	})
	return out, credErr(err)
}

// GetCredentialVersion implements [credential.Store].
func (s *Store) GetCredentialVersion(ctx context.Context, org orgscope.Org, cred id.UUID, number int) (credential.Version, error) {
	var out credential.Version
	err := s.inScope(ctx, "get credential version", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetCredentialVersion(ctx, pgUUID(identity.OrgIDOf(org)), pgUUID(cred), int64(number))
		if errors.Is(err, pgx.ErrNoRows) {
			return credential.ErrNotFound
		}
		if err != nil {
			return err
		}
		out = credential.Version{
			ID: idOfPG(row.ID), OrgID: identity.OrgIDOf(org), CredentialID: idOfPG(row.CredentialID),
			Number: int(row.VersionNumber), PublicFields: row.PublicFieldDocument, SecretCiphertext: row.SecretFieldCiphertext,
			DataKeyWrapped: row.DataKeyWrapped, OrgKeyVersion: int(row.OrgKeyVersionNumber),
			CreatedBy: principalOfPG(row.CreatedByPrincipalID), CreatedAt: instant(row.CreatedAt),
		}
		return nil
	})
	return out, credErr(err)
}

// AddVersion implements [credential.Store]: the version and the move of the current pointer in one transaction. A
// pointer that moved since the read makes no row change and is a conflict.
func (s *Store) AddVersion(ctx context.Context, org orgscope.Org, cred id.UUID, expected int, v credential.Version, status credential.Status, at time.Time, entry audit.Entry) error {
	err := s.inScopeRecord(ctx, "add credential version", org, v.OrgID, func(ctx context.Context, q *sqlcgen.Queries) error {
		if err := insertCredentialVersionPG(ctx, q, v); err != nil {
			return err
		}
		n, err := q.UpdateCredentialCurrentVersion(ctx, sqlcgen.UpdateCredentialCurrentVersionParams{
			NextVersionNumber: int64(v.Number), Status: string(status), UpdatedAt: timestamp(at),
			OrgID: pgUUID(identity.OrgIDOf(org)), ID: pgUUID(cred), ExpectedVersionNumber: int64(expected),
		})
		if err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("the current version moved: %w", credential.ErrConflict)
		}
		return insertPendingOrg(ctx, q, org, entry)
	})
	return credErr(err)
}

// UpdateCredential implements [credential.Store].
func (s *Store) UpdateCredential(ctx context.Context, org orgscope.Org, rec credential.Record, expected time.Time, entry audit.Entry) error {
	err := s.inScope(ctx, "update credential", org, false, func(ctx context.Context, q *sqlcgen.Queries) error {
		hosts, err := credential.EncodeHosts(rec.AllowedHosts)
		if err != nil {
			return err
		}
		n, err := q.UpdateCredentialSettings(ctx, sqlcgen.UpdateCredentialSettingsParams{
			Name: rec.Name, AllowedHostDocument: hosts, UpdatedAt: timestamp(rec.UpdatedAt),
			OrgID: pgUUID(identity.OrgIDOf(org)), ID: pgUUID(rec.ID), ProjectID: pgUUID(rec.ProjectID),
			ExpectedUpdatedAt: timestamp(expected),
		})
		if err != nil {
			return err
		}
		if n != 1 {
			return missOrChanged(ctx, q, org, rec.ProjectID, rec.ID)
		}
		return insertPendingOrg(ctx, q, org, entry)
	})
	return credErr(err)
}

// DeleteCredential implements [credential.Store]. The versions and the shares go with the row (ON DELETE CASCADE).
func (s *Store) DeleteCredential(ctx context.Context, org orgscope.Org, project identity.ProjectID, cred id.UUID, expected time.Time, entry audit.Entry) error {
	err := s.inScope(ctx, "delete credential", org, false, func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.DeleteCredential(ctx, sqlcgen.DeleteCredentialParams{OrgID: pgUUID(identity.OrgIDOf(org)), ID: pgUUID(cred), ProjectID: pgUUID(project), ExpectedUpdatedAt: timestamp(expected)})
		if err != nil {
			return err
		}
		if n != 1 {
			return missOrChanged(ctx, q, org, project, cred)
		}
		return insertPendingOrg(ctx, q, org, entry)
	})
	return credErr(err)
}

// GetShare implements [credential.Store].
func (s *Store) GetShare(ctx context.Context, org orgscope.Org, cred id.UUID, project identity.ProjectID) (credential.Share, error) {
	var out credential.Share
	err := s.inScope(ctx, "get credential share", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetCredentialShare(ctx, pgUUID(identity.OrgIDOf(org)), pgUUID(cred), pgUUID(project))
		if errors.Is(err, pgx.ErrNoRows) {
			return credential.ErrNotFound
		}
		if err != nil {
			return err
		}
		out = credential.Share{
			ID: idOfPG(row.ID), OrgID: identity.OrgIDOf(org), CredentialID: cred, ProjectID: project,
			GrantedBy: principalOfPG(row.GrantedByPrincipalID), CreatedAt: instant(row.CreatedAt),
		}
		return nil
	})
	return out, credErr(err)
}

// CreateShare implements [credential.Store]. A project of another organization is a missing foreign key: ErrNotFound.
func (s *Store) CreateShare(ctx context.Context, org orgscope.Org, sh credential.Share, entry audit.Entry) error {
	err := s.inScopeRecord(ctx, "create credential share", org, sh.OrgID, func(ctx context.Context, q *sqlcgen.Queries) error {
		if err := q.InsertCredentialShare(ctx, sqlcgen.InsertCredentialShareParams{
			ID: pgUUID(sh.ID), OrgID: pgUUID(sh.OrgID), CredentialID: pgUUID(sh.CredentialID),
			ProjectID: pgUUID(sh.ProjectID), GrantedByPrincipalID: nullPGUUID(sh.GrantedBy), CreatedAt: timestamp(sh.CreatedAt), UpdatedAt: timestamp(sh.CreatedAt),
		}); err != nil {
			return err
		}
		return insertPendingOrg(ctx, q, org, entry)
	})
	return credErr(err)
}

// DeleteShare implements [credential.Store].
func (s *Store) DeleteShare(ctx context.Context, org orgscope.Org, cred id.UUID, project identity.ProjectID, entry audit.Entry) error {
	err := s.inScope(ctx, "delete credential share", org, false, func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.DeleteCredentialShare(ctx, pgUUID(identity.OrgIDOf(org)), pgUUID(cred), pgUUID(project))
		if err != nil {
			return err
		}
		if n != 1 {
			return credential.ErrNotFound
		}
		return insertPendingOrg(ctx, q, org, entry)
	})
	return credErr(err)
}

// insertCredentialVersionPG writes one version row.
func insertCredentialVersionPG(ctx context.Context, q *sqlcgen.Queries, v credential.Version) error {
	return q.InsertCredentialVersion(ctx, sqlcgen.InsertCredentialVersionParams{
		ID: pgUUID(v.ID), OrgID: pgUUID(v.OrgID), CredentialID: pgUUID(v.CredentialID), VersionNumber: int64(v.Number),
		PublicFieldDocument: v.PublicFields, PublicFieldDocumentFormat: credential.Format,
		SecretFieldCiphertext: v.SecretCiphertext, DataKeyWrapped: v.DataKeyWrapped,
		OrgKeyVersionNumber: int64(v.OrgKeyVersion), CreatedByPrincipalID: nullPGUUID(v.CreatedBy), CreatedAt: timestamp(v.CreatedAt), UpdatedAt: timestamp(v.CreatedAt),
	})
}

// recordOfPG builds a record from the columns of a credential row, whichever query read it.
func recordOfPG(idv, orgv, projectv pgtype.UUID, name, connector, status string, hosts []byte, createdBy pgtype.UUID,
	createdAt, updatedAt, secretAt pgtype.Timestamptz, secretBy pgtype.UUID, current int64,
) (credential.Record, error) {
	allowed, err := credential.DecodeHosts(hosts)
	if err != nil {
		return credential.Record{}, err
	}
	return credential.Record{
		ID: idOfPG(idv), OrgID: identity.OrgID(idOfPG(orgv)), ProjectID: identity.ProjectID(idOfPG(projectv)),
		Name: name, ConnectorReference: connector, Status: credential.Status(status), AllowedHosts: allowed,
		CurrentVersion: int(current), CreatedBy: principalOfPG(createdBy), CreatedAt: instant(createdAt),
		UpdatedAt: instant(updatedAt), SecretUpdatedAt: instant(secretAt), SecretUpdatedBy: principalOfPG(secretBy),
	}, nil
}

// idOfPG converts a stored uuid to an identity. A NULL reads as the zero identity.
func idOfPG(u pgtype.UUID) id.UUID {
	if !u.Valid {
		return id.UUID{}
	}
	return id.UUID(u.Bytes)
}

// principalOfPG converts an optional author: NULL is the zero principal.
func principalOfPG(u pgtype.UUID) identity.PrincipalID {
	return identity.PrincipalID(idOfPG(u))
}

// credErr maps the errors of the flow adapters (which the shared mapping returns) to the credential sentinels. The
// original error stays in the chain.
func credErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, credential.ErrNotFound), errors.Is(err, credential.ErrConflict), errors.Is(err, credential.ErrInvalid):
		return err
	case errors.Is(err, flow.ErrNotFound), errors.Is(err, flow.ErrReference):
		return fmt.Errorf("%w: %w", credential.ErrNotFound, err)
	case errors.Is(err, flow.ErrConflict):
		return fmt.Errorf("%w: %w", credential.ErrConflict, err)
	case errors.Is(err, flow.ErrInvalidRecord):
		return fmt.Errorf("%w: %w", credential.ErrInvalid, err)
	}
	return err
}

// missOrChanged tells why a compare-and-swap write of a credential touched no row: the credential is not one of the
// project in this organization ([credential.ErrNotFound]), or it exists and changed since the caller read it
// ([credential.ErrConflict]).
func missOrChanged(ctx context.Context, q *sqlcgen.Queries, org orgscope.Org, project identity.ProjectID, cred id.UUID) error {
	row, err := q.GetCredential(ctx, pgUUID(identity.OrgIDOf(org)), pgUUID(cred))
	if errors.Is(err, pgx.ErrNoRows) {
		return credential.ErrNotFound
	}
	if err != nil {
		return err
	}
	if row.ProjectID != pgUUID(project) {
		return credential.ErrNotFound
	}
	return fmt.Errorf("the credential changed: %w", credential.ErrConflict)
}
