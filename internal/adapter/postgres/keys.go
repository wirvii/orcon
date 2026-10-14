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
	"github.com/wirvii/orcon/internal/credential/envelope"
	"github.com/wirvii/orcon/internal/identity"
)

// The key store of SPEC-004 R-KEY (ADR-0042): the adapter behind envelope.KeyStore.
// Organization keys are organization-scoped rows (row-level security applies), so
// their reads and writes run under the organization of the key; the list of the
// active keys of every organization is the engine function orcon_engine.list_active_org_keys.

// MasterCanary implements [envelope.KeyReader].
func (s *Store) MasterCanary(ctx context.Context) (envelope.MasterCanary, error) {
	var out envelope.MasterCanary
	err := s.instance(ctx, "get master key canary", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetMasterCanary(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			return envelope.ErrKeyNotFound
		}
		if err != nil {
			return err
		}
		out = envelope.MasterCanary{Version: int(row.KeyVersion), Probe: row.Probe}
		return nil
	})
	return out, err
}

// PutMasterCanary implements [envelope.KeyStore].
func (s *Store) PutMasterCanary(ctx context.Context, c envelope.MasterCanary) error {
	return s.instanceTx(ctx, "put master key canary", func(ctx context.Context, q *sqlcgen.Queries) error {
		if _, err := q.GetMasterCanary(ctx); err == nil {
			return envelope.ErrKeyExists
		}
		return keyErr(q.InsertMasterCanary(ctx, int64(c.Version), c.Probe, timestamp(time.Now())))
	})
}

// ActiveKeys implements [envelope.KeyReader]: the active instance key, if any, and the
// active key of every organization, read through the engine function.
func (s *Store) ActiveKeys(ctx context.Context) ([]envelope.StoredKey, error) {
	var out []envelope.StoredKey
	err := s.instance(ctx, "get active instance key", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetActiveInstanceKey(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		out = append(out, pgInstanceKey(row.KeyVersion, row.Wrapped, row.MasterKeyReference, row.Status, row.CreatedAt, row.Probe))
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = s.engineRows(ctx, "list active organization keys",
		"SELECT org_id, key_version, wrapped, master_key_reference, status, created_at, probe FROM orcon_engine.list_active_org_keys()",
		nil,
		func(rows pgx.Rows) error {
			var org pgtype.UUID
			var version int64
			var wrapped []byte
			var reference, status string
			var created pgtype.Timestamptz
			var probe []byte
			if err := rows.Scan(&org, &version, &wrapped, &reference, &status, &created, &probe); err != nil {
				return err
			}
			out = append(out, pgOrgKey(fromPGUUID[identity.OrgID](org), version, wrapped, reference, status, created, probe))
			return nil
		})
	return out, err
}

// Key implements [envelope.KeyStore]: one version of one key, whatever its status.
func (s *Store) Key(ctx context.Context, id envelope.KeyID) (envelope.StoredKey, error) {
	return s.keyRow(ctx, id.Level, id.OrgID, id.Version, false)
}

// ActiveKey implements [envelope.KeyStore]: the active key of a scope.
func (s *Store) ActiveKey(ctx context.Context, scope envelope.Scope) (envelope.StoredKey, error) {
	return s.keyRow(ctx, scope.Level, scope.OrgID, 0, true)
}

// keyRow reads one key of a level: the active one when active is true, else the version.
func (s *Store) keyRow(ctx context.Context, level envelope.KeyLevel, org identity.OrgID, version int, active bool) (envelope.StoredKey, error) {
	var out envelope.StoredKey
	var err error
	switch level {
	case envelope.LevelInstance:
		err = s.instance(ctx, "get instance key", func(ctx context.Context, q *sqlcgen.Queries) error {
			var row sqlcgen.GetInstanceKeyByVersionRow
			var rerr error
			if active {
				a, aerr := q.GetActiveInstanceKey(ctx)
				row, rerr = sqlcgen.GetInstanceKeyByVersionRow(a), aerr
			} else {
				row, rerr = q.GetInstanceKeyByVersion(ctx, int64(version))
			}
			if errors.Is(rerr, pgx.ErrNoRows) {
				return envelope.ErrKeyNotFound
			}
			if rerr != nil {
				return rerr
			}
			out = pgInstanceKey(row.KeyVersion, row.Wrapped, row.MasterKeyReference, row.Status, row.CreatedAt, row.Probe)
			return nil
		})
	case envelope.LevelOrg:
		err = s.inOrg(ctx, "get organization key", org, true, func(ctx context.Context, q *sqlcgen.Queries) error {
			var row sqlcgen.GetOrgKeyByVersionRow
			var rerr error
			if active {
				a, aerr := q.GetActiveOrgKey(ctx, pgUUID(org))
				row, rerr = sqlcgen.GetOrgKeyByVersionRow(a), aerr
			} else {
				row, rerr = q.GetOrgKeyByVersion(ctx, pgUUID(org), int64(version))
			}
			if errors.Is(rerr, pgx.ErrNoRows) {
				return envelope.ErrKeyNotFound
			}
			if rerr != nil {
				return rerr
			}
			out = pgOrgKey(org, row.KeyVersion, row.Wrapped, row.MasterKeyReference, row.Status, row.CreatedAt, row.Probe)
			return nil
		})
	case envelope.LevelMaster:
		return out, fmt.Errorf("key of level %q: %w", level, envelope.ErrKeyNotFound)
	default:
		return out, fmt.Errorf("key of level %q: %w", level, envelope.ErrKeyNotFound)
	}
	return out, err
}

// PutKey implements [envelope.KeyStore]: the key and its probe in one transaction, or
// [envelope.ErrKeyExists] when that version is stored already.
func (s *Store) PutKey(ctx context.Context, key envelope.StoredKey) error {
	switch key.ID.Level {
	case envelope.LevelInstance:
		return s.instanceTx(ctx, "put instance key", func(ctx context.Context, q *sqlcgen.Queries) error {
			if _, err := q.GetInstanceKeyByVersion(ctx, int64(key.ID.Version)); err == nil {
				return envelope.ErrKeyExists
			}
			if err := q.InsertInstanceKey(ctx, sqlcgen.InsertInstanceKeyParams{
				KeyLevel: string(envelope.LevelInstance), KeyVersion: int64(key.ID.Version), Wrapped: key.Wrapped,
				MasterKeyReference: key.MasterKeyReference, Status: string(key.Status), CreatedAt: timestamp(key.CreatedAt),
			}); err != nil {
				return keyErr(err)
			}
			return keyErr(q.InsertInstanceKeyCanary(ctx, sqlcgen.InsertInstanceKeyCanaryParams{
				KeyLevel: string(envelope.LevelInstance), KeyVersion: int64(key.ID.Version), Probe: key.Canary, CreatedAt: timestamp(key.CreatedAt),
			}))
		})
	case envelope.LevelOrg:
		return s.inOrg(ctx, "put organization key", key.ID.OrgID, false, func(ctx context.Context, q *sqlcgen.Queries) error {
			org := pgUUID(key.ID.OrgID)
			if _, err := q.GetOrgKeyByVersion(ctx, org, int64(key.ID.Version)); err == nil {
				return envelope.ErrKeyExists
			}
			return keyErr(q.InsertOrgKey(ctx, sqlcgen.InsertOrgKeyParams{
				OrgID: org, KeyVersion: int64(key.ID.Version), Wrapped: key.Wrapped,
				MasterKeyReference: key.MasterKeyReference, Status: string(key.Status), Probe: key.Canary,
				CreatedAt: timestamp(key.CreatedAt),
			}))
		})
	case envelope.LevelMaster:
		return fmt.Errorf("put key of level %q: %w", key.ID.Level, envelope.ErrKeyUnavailable)
	default:
		return fmt.Errorf("put key of level %q: %w", key.ID.Level, envelope.ErrKeyUnavailable)
	}
}

// keyErr maps a uniqueness violation of a key table to the key sentinel.
func keyErr(err error) error {
	if errors.Is(err, identity.ErrConflict) {
		return fmt.Errorf("%w: %w", envelope.ErrKeyExists, err)
	}
	return err
}

func pgInstanceKey(version int64, wrapped []byte, reference, status string, created pgtype.Timestamptz, probe []byte) envelope.StoredKey {
	return envelope.StoredKey{
		ID:      envelope.KeyID{Level: envelope.LevelInstance, Version: int(version)},
		Wrapped: wrapped, MasterKeyReference: reference, Status: envelope.KeyStatus(status),
		CreatedAt: instant(created), Canary: probe,
	}
}

func pgOrgKey(org identity.OrgID, version int64, wrapped []byte, reference, status string, created pgtype.Timestamptz, probe []byte) envelope.StoredKey {
	return envelope.StoredKey{
		ID:      envelope.KeyID{Level: envelope.LevelOrg, OrgID: org, Version: int(version)},
		Wrapped: wrapped, MasterKeyReference: reference, Status: envelope.KeyStatus(status),
		CreatedAt: instant(created), Canary: probe,
	}
}
