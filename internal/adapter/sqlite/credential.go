// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/credential"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/orgscope"
)

// credentialRowLimit bounds the credentials one listing returns.
const credentialRowLimit = 1000

// The credential store (SPEC-004 SE7, ADR-0043): the adapter behind credential.Store. Every statement is bound to
// the organization of its scope; a missing row is credential.ErrNotFound and a clash is credential.ErrConflict.
var _ credential.Store = (*Store)(nil)

// writeOf is [Store.write] for one organization: the zero scope is refused before the write starts.
func (s *Store) writeOf(ctx context.Context, op string, org orgscope.Org, fn func(ctx context.Context, q *sqlcgen.Queries) error) error {
	if _, err := scoped(org); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return s.write(ctx, op, fn)
}

// CreateCredential implements [credential.Store].
func (s *Store) CreateCredential(ctx context.Context, org orgscope.Org, rec credential.Record, first credential.Version, entry audit.Entry) error {
	err := s.writeOf(ctx, "create credential", org, func(ctx context.Context, q *sqlcgen.Queries) error {
		orgb, err := scopedRecord(org, rec.OrgID)
		if err != nil {
			return err
		}
		hosts, err := credential.EncodeHosts(rec.AllowedHosts)
		if err != nil {
			return err
		}
		if err := q.InsertCredential(ctx, sqlcgen.InsertCredentialParams{
			ID: uuidBytes(rec.ID), OrgID: orgb, ProjectID: uuidBytes(rec.ProjectID),
			Name: rec.Name, ConnectorReference: rec.ConnectorReference, Status: string(rec.Status),
			AllowedHostDocument: hosts, AllowedHostDocumentFormat: credential.Format,
			CurrentVersionNumber: int64(rec.CurrentVersion), CreatedByPrincipalID: nullUUIDBytes(rec.CreatedBy),
			CreatedAt: micros(rec.CreatedAt), UpdatedAt: micros(rec.UpdatedAt),
		}); err != nil {
			return err
		}
		if err := insertCredentialVersion(ctx, q, orgb, first); err != nil {
			return err
		}
		return insertPendingOrg(ctx, q, org, entry)
	})
	return credErr(err)
}

// GetCredential implements [credential.Store].
func (s *Store) GetCredential(ctx context.Context, org orgscope.Org, cred id.UUID) (credential.Record, error) {
	var out credential.Record
	err := s.readOf(ctx, "get credential", org, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetCredential(ctx, uuidBytes(identity.OrgIDOf(org)), uuidBytes(cred))
		if isNoRows(err) {
			return credential.ErrNotFound
		}
		if err != nil {
			return err
		}
		out, err = recordOf(row.ID, row.OrgID, row.ProjectID, row.Name, row.ConnectorReference, row.Status,
			row.AllowedHostDocument, row.CurrentVersionNumber, row.CreatedByPrincipalID, row.CreatedAt, row.UpdatedAt,
			row.SecretSetAt, row.SecretSetByPrincipalID)
		return err
	})
	return out, credErr(err)
}

// ListCredentials implements [credential.Store].
func (s *Store) ListCredentials(ctx context.Context, org orgscope.Org, project identity.ProjectID) ([]credential.Record, error) {
	out := []credential.Record{}
	err := s.readOf(ctx, "list credentials", org, func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListCredentialsOfProject(ctx, uuidBytes(identity.OrgIDOf(org)), uuidBytes(project), credentialRowLimit)
		if err != nil {
			return err
		}
		for i := range rows {
			row := rows[i]
			rec, err := recordOf(row.ID, row.OrgID, row.ProjectID, row.Name, row.ConnectorReference, row.Status,
				row.AllowedHostDocument, row.CurrentVersionNumber, row.CreatedByPrincipalID, row.CreatedAt, row.UpdatedAt,
				row.SecretSetAt, row.SecretSetByPrincipalID)
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
	err := s.readOf(ctx, "get credential version", org, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetCredentialVersion(ctx, uuidBytes(identity.OrgIDOf(org)), uuidBytes(cred), int64(number))
		if isNoRows(err) {
			return credential.ErrNotFound
		}
		if err != nil {
			return err
		}
		out = credential.Version{
			ID: uuidOfBytes[id.UUID](row.ID), OrgID: identity.OrgIDOf(org), CredentialID: uuidOfBytes[id.UUID](row.CredentialID),
			Number: int(row.VersionNumber), PublicFields: row.PublicFieldDocument, SecretCiphertext: row.SecretFieldCiphertext,
			DataKeyWrapped: row.DataKeyWrapped, OrgKeyVersion: int(row.OrgKeyVersionNumber),
			CreatedBy: principalOf(row.CreatedByPrincipalID), CreatedAt: instant(row.CreatedAt),
		}
		return nil
	})
	return out, credErr(err)
}

// AddVersion implements [credential.Store]: the version and the move of the current pointer in one transaction. A
// pointer that moved since the read makes no row change and is a conflict.
func (s *Store) AddVersion(ctx context.Context, org orgscope.Org, cred id.UUID, expected int, v credential.Version, status credential.Status, at time.Time, entry audit.Entry) error {
	err := s.writeOf(ctx, "add credential version", org, func(ctx context.Context, q *sqlcgen.Queries) error {
		orgb, err := scopedRecord(org, v.OrgID)
		if err != nil {
			return err
		}
		if err := insertCredentialVersion(ctx, q, orgb, v); err != nil {
			return err
		}
		n, err := q.UpdateCredentialCurrentVersion(ctx, sqlcgen.UpdateCredentialCurrentVersionParams{
			NextVersionNumber: int64(v.Number), Status: string(status), UpdatedAt: micros(at),
			OrgID: uuidBytes(identity.OrgIDOf(org)), ID: uuidBytes(cred), ExpectedVersionNumber: int64(expected),
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
	err := s.writeOf(ctx, "update credential", org, func(ctx context.Context, q *sqlcgen.Queries) error {
		hosts, err := credential.EncodeHosts(rec.AllowedHosts)
		if err != nil {
			return err
		}
		n, err := q.UpdateCredentialSettings(ctx, sqlcgen.UpdateCredentialSettingsParams{
			Name: rec.Name, AllowedHostDocument: hosts, UpdatedAt: micros(rec.UpdatedAt),
			OrgID: uuidBytes(identity.OrgIDOf(org)), ID: uuidBytes(rec.ID), ProjectID: uuidBytes(rec.ProjectID),
			ExpectedUpdatedAt: micros(expected),
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
	err := s.writeOf(ctx, "delete credential", org, func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.DeleteCredential(ctx, sqlcgen.DeleteCredentialParams{OrgID: uuidBytes(identity.OrgIDOf(org)), ID: uuidBytes(cred), ProjectID: uuidBytes(project), ExpectedUpdatedAt: micros(expected)})
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
	err := s.readOf(ctx, "get credential share", org, func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetCredentialShare(ctx, uuidBytes(identity.OrgIDOf(org)), uuidBytes(cred), uuidBytes(project))
		if isNoRows(err) {
			return credential.ErrNotFound
		}
		if err != nil {
			return err
		}
		out = credential.Share{
			ID: uuidOfBytes[id.UUID](row.ID), OrgID: identity.OrgIDOf(org), CredentialID: cred,
			ProjectID: project, GrantedBy: principalOf(row.GrantedByPrincipalID), CreatedAt: instant(row.CreatedAt),
		}
		return nil
	})
	return out, credErr(err)
}

// CreateShare implements [credential.Store]. A project of another organization is a missing foreign key: ErrNotFound.
func (s *Store) CreateShare(ctx context.Context, org orgscope.Org, sh credential.Share, entry audit.Entry) error {
	err := s.writeOf(ctx, "create credential share", org, func(ctx context.Context, q *sqlcgen.Queries) error {
		orgb, err := scopedRecord(org, sh.OrgID)
		if err != nil {
			return err
		}
		if err := q.InsertCredentialShare(ctx, sqlcgen.InsertCredentialShareParams{
			ID: uuidBytes(sh.ID), OrgID: orgb, CredentialID: uuidBytes(sh.CredentialID),
			ProjectID: uuidBytes(sh.ProjectID), GrantedByPrincipalID: nullUUIDBytes(sh.GrantedBy), CreatedAt: micros(sh.CreatedAt), UpdatedAt: micros(sh.CreatedAt),
		}); err != nil {
			return err
		}
		return insertPendingOrg(ctx, q, org, entry)
	})
	return credErr(err)
}

// DeleteShare implements [credential.Store].
func (s *Store) DeleteShare(ctx context.Context, org orgscope.Org, cred id.UUID, project identity.ProjectID, entry audit.Entry) error {
	err := s.writeOf(ctx, "delete credential share", org, func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.DeleteCredentialShare(ctx, uuidBytes(identity.OrgIDOf(org)), uuidBytes(cred), uuidBytes(project))
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

// insertCredentialVersion writes one version row of the organization org, which the caller checked against the scope.
func insertCredentialVersion(ctx context.Context, q *sqlcgen.Queries, org []byte, v credential.Version) error {
	return q.InsertCredentialVersion(ctx, sqlcgen.InsertCredentialVersionParams{
		ID: uuidBytes(v.ID), OrgID: org, CredentialID: uuidBytes(v.CredentialID), VersionNumber: int64(v.Number),
		PublicFieldDocument: v.PublicFields, PublicFieldDocumentFormat: credential.Format,
		SecretFieldCiphertext: v.SecretCiphertext, DataKeyWrapped: v.DataKeyWrapped,
		OrgKeyVersionNumber: int64(v.OrgKeyVersion), CreatedByPrincipalID: nullUUIDBytes(v.CreatedBy), CreatedAt: micros(v.CreatedAt), UpdatedAt: micros(v.CreatedAt),
	})
}

// recordOf builds a record from the columns of a credential row, whichever query read it.
func recordOf(idb, orgb, projectb []byte, name, connector, status string, hosts []byte, current int64, createdBy []byte,
	createdAt, updatedAt, secretAt int64, secretBy []byte,
) (credential.Record, error) {
	allowed, err := credential.DecodeHosts(hosts)
	if err != nil {
		return credential.Record{}, err
	}
	return credential.Record{
		ID: uuidOfBytes[id.UUID](idb), OrgID: identity.OrgID(uuidOfBytes[id.UUID](orgb)),
		ProjectID: identity.ProjectID(uuidOfBytes[id.UUID](projectb)), Name: name, ConnectorReference: connector,
		Status: credential.Status(status), AllowedHosts: allowed, CurrentVersion: int(current),
		CreatedBy: principalOf(createdBy), CreatedAt: instant(createdAt), UpdatedAt: instant(updatedAt),
		SecretUpdatedAt: instant(secretAt), SecretUpdatedBy: principalOf(secretBy),
	}, nil
}

// uuidOfBytes converts stored bytes to an identity. A value of the wrong length is a damaged row, which the stores
// report as the zero identity; every caller reads rows its own inserts wrote.
func uuidOfBytes[T ~[16]byte](b []byte) T {
	var out T
	if len(b) == 16 {
		copy(out[:], b)
	}
	return out
}

// principalOf converts an optional author: NULL is the zero principal.
func principalOf(b []byte) identity.PrincipalID {
	if len(b) != 16 {
		return identity.PrincipalID{}
	}
	return identity.PrincipalID(uuidOfBytes[id.UUID](b))
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
	row, err := q.GetCredential(ctx, uuidBytes(identity.OrgIDOf(org)), uuidBytes(cred))
	if isNoRows(err) {
		return credential.ErrNotFound
	}
	if err != nil {
		return err
	}
	if !bytes.Equal(row.ProjectID, uuidBytes(project)) {
		return credential.ErrNotFound
	}
	return fmt.Errorf("the credential changed: %w", credential.ErrConflict)
}
