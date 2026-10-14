// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
	"github.com/wirvii/orcon/internal/identity"
)

// GetUserCredentialsByEmail implements [identity.UserStore].
func (s *Store) GetUserCredentialsByEmail(ctx context.Context, emailNormalized string) (identity.UserCredentials, error) {
	var out identity.UserCredentials
	err := s.read(ctx, "get user credentials by email", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetApplicationUserCredentialsByEmail(ctx, emailNormalized)
		if err != nil {
			return err
		}
		uid, err := uuidOf[identity.UserID](row.ID)
		if err != nil {
			return err
		}
		pid, err := uuidOf[identity.PrincipalID](row.PrincipalID)
		if err != nil {
			return err
		}
		out = identity.UserCredentials{
			User: identity.UserRecord{
				ID: uid, PrincipalID: pid, EmailNormalized: row.EmailNormalized, DisplayName: row.DisplayName,
				Status: row.Status, CreatedAt: instant(row.CreatedAt), UpdatedAt: instant(row.UpdatedAt),
			},
			PasswordHash: row.PasswordHash.String,
		}
		return nil
	})
	return out, err
}

// SetPasswordHash implements [identity.UserStore].
func (s *Store) SetPasswordHash(ctx context.Context, user identity.UserID, hash string, now time.Time) error {
	return s.write(ctx, "set password hash", func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.UpdateApplicationUserPasswordHash(ctx, sql.NullString{String: hash, Valid: true}, micros(now), uuidBytes(user))
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("user %s: %w", user, identity.ErrNotFound)
		}
		return nil
	})
}

// CreateSession implements [identity.SessionStore].
func (s *Store) CreateSession(ctx context.Context, rec identity.SessionRecord) error {
	return s.write(ctx, "create session", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertSession(ctx, sqlcgen.InsertSessionParams{
			ID: uuidBytes(rec.ID), ApplicationUserID: uuidBytes(rec.UserID), TokenHash: rec.TokenHash,
			AssuranceLevel: rec.AssuranceLevel, CreatedAt: micros(rec.CreatedAt), ExpiresAt: micros(rec.ExpiresAt),
			IdleExpiresAt: micros(rec.IdleExpiresAt), LastSeenAt: micros(rec.LastSeenAt),
		})
	})
}

// GetSessionByTokenHash implements [identity.SessionStore].
func (s *Store) GetSessionByTokenHash(ctx context.Context, tokenHash []byte) (identity.SessionView, error) {
	var out identity.SessionView
	err := s.read(ctx, "get session by token", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetSessionByTokenHash(ctx, tokenHash)
		if err != nil {
			return err
		}
		rec, err := sessionRecord(row.ID, row.ApplicationUserID, row.AssuranceLevel, row.CreatedAt, row.ExpiresAt, row.IdleExpiresAt, row.LastSeenAt, row.RevokedAt)
		if err != nil {
			return err
		}
		pid, err := uuidOf[identity.PrincipalID](row.PrincipalID)
		if err != nil {
			return err
		}
		out = identity.SessionView{
			Session: rec, PrincipalID: pid, UserStatus: row.UserStatus,
			Email: row.EmailNormalized, DisplayName: row.DisplayName,
		}
		return nil
	})
	return out, err
}

// GetSession implements [identity.SessionStore].
func (s *Store) GetSession(ctx context.Context, user identity.UserID, session identity.SessionID) (identity.SessionRecord, error) {
	var out identity.SessionRecord
	err := s.read(ctx, "get session", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetSession(ctx, uuidBytes(user), uuidBytes(session))
		if err != nil {
			return err
		}
		out, err = sessionRecord(row.ID, row.ApplicationUserID, row.AssuranceLevel, row.CreatedAt, row.ExpiresAt, row.IdleExpiresAt, row.LastSeenAt, row.RevokedAt)
		return err
	})
	return out, err
}

// UpdateSessionActivity implements [identity.SessionStore].
func (s *Store) UpdateSessionActivity(ctx context.Context, session identity.SessionID, lastSeen, idleExpiresAt time.Time) error {
	return s.write(ctx, "update session activity", func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.UpdateSessionActivity(ctx, micros(lastSeen), micros(idleExpiresAt), uuidBytes(session))
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("session %s: %w", session, identity.ErrNotFound)
		}
		return nil
	})
}

// MarkSessionRevoked implements [identity.SessionStore].
func (s *Store) MarkSessionRevoked(ctx context.Context, user identity.UserID, session identity.SessionID, at time.Time) error {
	return s.write(ctx, "revoke session", func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.MarkSessionRevoked(ctx, sql.NullInt64{Int64: micros(at), Valid: true}, uuidBytes(session), uuidBytes(user))
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("session %s: %w", session, identity.ErrNotFound)
		}
		return nil
	})
}

// GetLoginThrottle implements [identity.LoginThrottleStore].
func (s *Store) GetLoginThrottle(ctx context.Context, identifier []byte) (identity.LoginThrottle, error) {
	var out identity.LoginThrottle
	err := s.read(ctx, "get login throttle", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetLoginThrottle(ctx, identifier)
		if err != nil {
			return err
		}
		out = identity.LoginThrottle{Failures: int(row.Failures), LockedUntil: instant(row.LockedUntil), UpdatedAt: instant(row.UpdatedAt)}
		return nil
	})
	return out, err
}

// PutLoginThrottle implements [identity.LoginThrottleStore].
func (s *Store) PutLoginThrottle(ctx context.Context, identifier []byte, t identity.LoginThrottle) error {
	return s.write(ctx, "put login throttle", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.UpsertLoginThrottle(ctx, sqlcgen.UpsertLoginThrottleParams{
			ID: identifier, Failures: int64(t.Failures), LockedUntil: micros(t.LockedUntil),
			CreatedAt: micros(t.UpdatedAt), UpdatedAt: micros(t.UpdatedAt),
		})
	})
}

// ClearLoginThrottle implements [identity.LoginThrottleStore].
func (s *Store) ClearLoginThrottle(ctx context.Context, identifier []byte) error {
	return s.write(ctx, "clear login throttle", func(ctx context.Context, q *sqlcgen.Queries) error {
		_, err := q.DeleteLoginThrottle(ctx, identifier)
		return err
	})
}

// sessionRecord converts the columns of a session row.
func sessionRecord(id, user []byte, assurance string, created, expires, idle, lastSeen int64, revoked sql.NullInt64) (identity.SessionRecord, error) {
	sid, err := uuidOf[identity.SessionID](id)
	if err != nil {
		return identity.SessionRecord{}, fmt.Errorf("session id: %w", err)
	}
	uid, err := uuidOf[identity.UserID](user)
	if err != nil {
		return identity.SessionRecord{}, fmt.Errorf("session user id: %w", err)
	}
	return identity.SessionRecord{
		ID: sid, UserID: uid, AssuranceLevel: assurance, CreatedAt: instant(created),
		ExpiresAt: instant(expires), IdleExpiresAt: instant(idle), LastSeenAt: instant(lastSeen),
		RevokedAt: nullInstant(revoked),
	}, nil
}
