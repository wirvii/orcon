// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
	"github.com/wirvii/orcon/internal/credential/envelope"
	"github.com/wirvii/orcon/internal/flow"
	"github.com/wirvii/orcon/internal/identity"
)

// keyBatch bounds how many organization keys one read returns (startup verification
// reads them all, in batches).
const keyBatch = 500

// The key store of SPEC-004 R-KEY (ADR-0042): the adapter behind envelope.KeyStore.
// Keys are stored wrapped; the probes that prove them are stored next to them.

// MasterCanary implements [envelope.KeyReader].
func (s *Store) MasterCanary(ctx context.Context) (envelope.MasterCanary, error) {
	var out envelope.MasterCanary
	err := s.read(ctx, "get master key canary", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetMasterCanary(ctx)
		if isNoRows(err) {
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
	return s.write(ctx, "put master key canary", func(ctx context.Context, q *sqlcgen.Queries) error {
		if _, err := q.GetMasterCanary(ctx); err == nil {
			return envelope.ErrKeyExists
		}
		return keyErr(q.InsertMasterCanary(ctx, sqlcgen.InsertMasterCanaryParams{
			ID: keyRowID("canary", envelope.LevelMaster, identity.OrgID{}, c.Version), KeyVersion: int64(c.Version),
			Probe: c.Probe, CreatedAt: micros(time.Now()),
		}))
	})
}

// ActiveKeys implements [envelope.KeyReader]: the active instance key, if any, and
// the active key of every organization.
func (s *Store) ActiveKeys(ctx context.Context) ([]envelope.StoredKey, error) {
	var out []envelope.StoredKey
	err := s.read(ctx, "get active instance key", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetActiveInstanceKey(ctx)
		if isNoRows(err) {
			return nil
		}
		if err != nil {
			return err
		}
		out = append(out, instanceKeyOf(row.KeyVersion, row.Wrapped, row.MasterKeyReference, row.Status, row.CreatedAt, row.Probe))
		return nil
	})
	if err != nil {
		return nil, err
	}
	after := make([]byte, 16)
	for {
		var rows []sqlcgen.ListActiveOrgKeysRow
		err := s.read(ctx, "list active organization keys", func(ctx context.Context, q *sqlcgen.Queries) error {
			var err error
			rows, err = q.ListActiveOrgKeys(ctx, after, keyBatch)
			return err
		})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			k, err := orgKeyOf(row.OrgID, row.KeyVersion, row.Wrapped, row.MasterKeyReference, row.Status, row.CreatedAt, row.Probe)
			if err != nil {
				return nil, err
			}
			out = append(out, k)
		}
		if len(rows) < keyBatch {
			return out, nil
		}
		after = rows[len(rows)-1].OrgID
	}
}

// Key implements [envelope.KeyStore]: one version of one key, whatever its status.
func (s *Store) Key(ctx context.Context, id envelope.KeyID) (envelope.StoredKey, error) {
	var out envelope.StoredKey
	err := s.read(ctx, "get key", func(ctx context.Context, q *sqlcgen.Queries) error {
		var err error
		out, err = s.keyRow(ctx, q, id.Level, id.OrgID, id.Version, false)
		return err
	})
	return out, err
}

// ActiveKey implements [envelope.KeyStore]: the active key of a scope.
func (s *Store) ActiveKey(ctx context.Context, scope envelope.Scope) (envelope.StoredKey, error) {
	var out envelope.StoredKey
	err := s.read(ctx, "get active key", func(ctx context.Context, q *sqlcgen.Queries) error {
		var err error
		out, err = s.keyRow(ctx, q, scope.Level, scope.OrgID, 0, true)
		return err
	})
	return out, err
}

// keyRow reads one key of a level: the active one when active is true, else the version.
func (s *Store) keyRow(ctx context.Context, q *sqlcgen.Queries, level envelope.KeyLevel, org identity.OrgID, version int, active bool) (envelope.StoredKey, error) {
	switch level {
	case envelope.LevelInstance:
		var row sqlcgen.GetInstanceKeyByVersionRow
		var err error
		if active {
			a, aerr := q.GetActiveInstanceKey(ctx)
			row, err = sqlcgen.GetInstanceKeyByVersionRow(a), aerr
		} else {
			row, err = q.GetInstanceKeyByVersion(ctx, int64(version))
		}
		if isNoRows(err) {
			return envelope.StoredKey{}, envelope.ErrKeyNotFound
		}
		if err != nil {
			return envelope.StoredKey{}, err
		}
		return instanceKeyOf(row.KeyVersion, row.Wrapped, row.MasterKeyReference, row.Status, row.CreatedAt, row.Probe), nil
	case envelope.LevelOrg:
		var row sqlcgen.GetOrgKeyByVersionRow
		var err error
		if active {
			a, aerr := q.GetActiveOrgKey(ctx, uuidBytes(org))
			row, err = sqlcgen.GetOrgKeyByVersionRow(a), aerr
		} else {
			row, err = q.GetOrgKeyByVersion(ctx, uuidBytes(org), int64(version))
		}
		if isNoRows(err) {
			return envelope.StoredKey{}, envelope.ErrKeyNotFound
		}
		if err != nil {
			return envelope.StoredKey{}, err
		}
		return orgKeyOf(uuidBytes(org), row.KeyVersion, row.Wrapped, row.MasterKeyReference, row.Status, row.CreatedAt, row.Probe)
	case envelope.LevelMaster:
		return envelope.StoredKey{}, fmt.Errorf("key of level %q: %w", level, envelope.ErrKeyNotFound)
	default:
		return envelope.StoredKey{}, fmt.Errorf("key of level %q: %w", level, envelope.ErrKeyNotFound)
	}
}

// PutKey implements [envelope.KeyStore]: the key and its probe in one transaction, or
// [envelope.ErrKeyExists] when that version is stored already.
func (s *Store) PutKey(ctx context.Context, key envelope.StoredKey) error {
	return s.write(ctx, "put key", func(ctx context.Context, q *sqlcgen.Queries) error {
		switch key.ID.Level {
		case envelope.LevelInstance:
			if _, err := q.GetInstanceKeyByVersion(ctx, int64(key.ID.Version)); err == nil {
				return envelope.ErrKeyExists
			}
			if err := q.InsertInstanceKey(ctx, sqlcgen.InsertInstanceKeyParams{
				ID: keyRowID("key", envelope.LevelInstance, identity.OrgID{}, key.ID.Version), KeyLevel: string(envelope.LevelInstance), KeyVersion: int64(key.ID.Version), Wrapped: key.Wrapped,
				MasterKeyReference: key.MasterKeyReference, Status: string(key.Status), CreatedAt: micros(key.CreatedAt),
			}); err != nil {
				return keyErr(err)
			}
			return keyErr(q.InsertInstanceKeyCanary(ctx, sqlcgen.InsertInstanceKeyCanaryParams{
				ID: keyRowID("canary", envelope.LevelInstance, identity.OrgID{}, key.ID.Version), KeyLevel: string(envelope.LevelInstance), KeyVersion: int64(key.ID.Version), Probe: key.Canary, CreatedAt: micros(key.CreatedAt),
			}))
		case envelope.LevelOrg:
			org := uuidBytes(key.ID.OrgID)
			if _, err := q.GetOrgKeyByVersion(ctx, org, int64(key.ID.Version)); err == nil {
				return envelope.ErrKeyExists
			}
			return keyErr(q.InsertOrgKey(ctx, sqlcgen.InsertOrgKeyParams{
				ID: keyRowID("key", envelope.LevelOrg, key.ID.OrgID, key.ID.Version), OrgID: org, KeyVersion: int64(key.ID.Version), Wrapped: key.Wrapped,
				MasterKeyReference: key.MasterKeyReference, Status: string(key.Status), Probe: key.Canary,
				CreatedAt: micros(key.CreatedAt),
			}))
		case envelope.LevelMaster:
			return fmt.Errorf("put key of level %q: %w", key.ID.Level, envelope.ErrKeyUnavailable)
		default:
			return fmt.Errorf("put key of level %q: %w", key.ID.Level, envelope.ErrKeyUnavailable)
		}
	})
}

// keyErr maps a uniqueness violation of a key table to the key sentinel.
func keyErr(err error) error {
	if errors.Is(err, flow.ErrConflict) {
		return fmt.Errorf("%w: %w", envelope.ErrKeyExists, err)
	}
	return err
}

func instanceKeyOf(version int64, wrapped []byte, reference, status string, created int64, probe []byte) envelope.StoredKey {
	return envelope.StoredKey{
		ID:      envelope.KeyID{Level: envelope.LevelInstance, Version: int(version)},
		Wrapped: wrapped, MasterKeyReference: reference, Status: envelope.KeyStatus(status),
		CreatedAt: instant(created), Canary: probe,
	}
}

func orgKeyOf(org []byte, version int64, wrapped []byte, reference, status string, created int64, probe []byte) (envelope.StoredKey, error) {
	oid, err := uuidOf[identity.OrgID](org)
	if err != nil {
		return envelope.StoredKey{}, fmt.Errorf("key organization: %w", err)
	}
	return envelope.StoredKey{
		ID:      envelope.KeyID{Level: envelope.LevelOrg, OrgID: oid, Version: int(version)},
		Wrapped: wrapped, MasterKeyReference: reference, Status: envelope.KeyStatus(status),
		CreatedAt: instant(created), Canary: probe,
	}, nil
}

// keyRowID returns the identifier of a key row: 16 bytes derived from the identity of
// the key version, so the same version always has the same row identifier. The kind
// separates a key row from its probe row.
func keyRowID(kind string, level envelope.KeyLevel, org identity.OrgID, version int) []byte {
	sum := sha256.Sum256([]byte("orcon key row v1|" + kind + "|" + string(level) + "|" + org.String() + "|" + strconv.Itoa(version)))
	return sum[:16]
}
