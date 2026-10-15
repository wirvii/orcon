// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wirvii/orcon/internal/adapter/postgres/sqlcgen"
	"github.com/wirvii/orcon/internal/identity"
)

// GetUserCredentialsByEmail implements [identity.UserStore].
func (s *Store) GetUserCredentialsByEmail(ctx context.Context, emailNormalized string) (identity.UserCredentials, error) {
	var out identity.UserCredentials
	err := s.instance(ctx, "get user credentials by email", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetApplicationUserCredentialsByEmail(ctx, emailNormalized)
		if err != nil {
			return err
		}
		out = identity.UserCredentials{
			User: identity.UserRecord{
				ID: fromPGUUID[identity.UserID](row.ID), PrincipalID: fromPGUUID[identity.PrincipalID](row.PrincipalID),
				EmailNormalized: row.EmailNormalized, DisplayName: row.DisplayName, Status: row.Status,
				CreatedAt: instant(row.CreatedAt), UpdatedAt: instant(row.UpdatedAt),
			},
			PasswordHash: row.PasswordHash.String,
		}
		return nil
	})
	return out, err
}

// SetPasswordHash implements [identity.UserStore].
func (s *Store) SetPasswordHash(ctx context.Context, user identity.UserID, hash string, now time.Time) error {
	return s.instanceTx(ctx, "set password hash", func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.UpdateApplicationUserPasswordHash(ctx, pgtype.Text{String: hash, Valid: true}, timestamp(now), pgUUID(user))
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
	return s.instanceTx(ctx, "create session", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.InsertSession(ctx, sqlcgen.InsertSessionParams{
			ID: pgUUID(rec.ID), ApplicationUserID: pgUUID(rec.UserID), TokenHash: rec.TokenHash,
			AssuranceLevel: rec.AssuranceLevel, CreatedAt: timestamp(rec.CreatedAt), ExpiresAt: timestamp(rec.ExpiresAt),
			IdleExpiresAt: timestamp(rec.IdleExpiresAt), LastSeenAt: timestamp(rec.LastSeenAt),
		})
	})
}

// GetSessionByTokenHash implements [identity.SessionStore].
func (s *Store) GetSessionByTokenHash(ctx context.Context, tokenHash []byte) (identity.SessionView, error) {
	var out identity.SessionView
	err := s.instance(ctx, "get session by token", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetSessionByTokenHash(ctx, tokenHash)
		if err != nil {
			return err
		}
		out = identity.SessionView{
			Session: sessionRecord(row.ID, row.ApplicationUserID, row.AssuranceLevel, row.CreatedAt, row.ExpiresAt,
				row.IdleExpiresAt, row.LastSeenAt, row.RevokedAt),
			PrincipalID: fromPGUUID[identity.PrincipalID](row.PrincipalID), UserStatus: row.UserStatus,
			Email: row.EmailNormalized, DisplayName: row.DisplayName,
		}
		return nil
	})
	return out, err
}

// GetSession implements [identity.SessionStore].
func (s *Store) GetSession(ctx context.Context, user identity.UserID, session identity.SessionID) (identity.SessionRecord, error) {
	var out identity.SessionRecord
	err := s.instance(ctx, "get session", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetSession(ctx, pgUUID(user), pgUUID(session))
		if err != nil {
			return err
		}
		out = sessionRecord(row.ID, row.ApplicationUserID, row.AssuranceLevel, row.CreatedAt, row.ExpiresAt,
			row.IdleExpiresAt, row.LastSeenAt, row.RevokedAt)
		return nil
	})
	return out, err
}

// UpdateSessionActivity implements [identity.SessionStore].
func (s *Store) UpdateSessionActivity(ctx context.Context, session identity.SessionID, lastSeen, idleExpiresAt time.Time) error {
	return s.instanceTx(ctx, "update session activity", func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.UpdateSessionActivity(ctx, timestamp(lastSeen), timestamp(idleExpiresAt), pgUUID(session))
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
	return s.instanceTx(ctx, "revoke session", func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.MarkSessionRevoked(ctx, timestamp(at), pgUUID(session), pgUUID(user))
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
	err := s.instance(ctx, "get login throttle", func(ctx context.Context, q *sqlcgen.Queries) error {
		row, err := q.GetLoginThrottle(ctx, identifier)
		if err != nil {
			return err
		}
		out = identity.LoginThrottle{Failures: int(row.Failures), LockedUntil: instant(row.LockedUntilAt), UpdatedAt: instant(row.UpdatedAt)}
		return nil
	})
	return out, err
}

// PutLoginThrottle implements [identity.LoginThrottleStore].
func (s *Store) PutLoginThrottle(ctx context.Context, identifier []byte, t identity.LoginThrottle) error {
	return s.instanceTx(ctx, "put login throttle", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.UpsertLoginThrottle(ctx, sqlcgen.UpsertLoginThrottleParams{
			ID: identifier, Failures: int64(t.Failures), LockedUntilAt: timestamp(t.LockedUntil),
			CreatedAt: timestamp(t.UpdatedAt), UpdatedAt: timestamp(t.UpdatedAt),
		})
	})
}

// IncrementLoginThrottle implements [identity.LoginThrottleStore].
func (s *Store) IncrementLoginThrottle(ctx context.Context, identifier []byte, now time.Time) (int, error) {
	var out int
	err := s.instanceTx(ctx, "increment login throttle", func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.UpsertLoginThrottleFailure(ctx, identifier, timestamp(now))
		out = int(n)
		return err
	})
	return out, err
}

// LockLoginThrottle implements [identity.LoginThrottleStore].
func (s *Store) LockLoginThrottle(ctx context.Context, identifier []byte, until, now time.Time) error {
	return s.instanceTx(ctx, "lock login throttle", func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.MarkLoginThrottleLocked(ctx, timestamp(until), timestamp(now), identifier)
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("login throttle: %w", identity.ErrNotFound)
		}
		return nil
	})
}

// ClearLoginThrottle implements [identity.LoginThrottleStore].
func (s *Store) ClearLoginThrottle(ctx context.Context, identifier []byte) error {
	return s.instanceTx(ctx, "clear login throttle", func(ctx context.Context, q *sqlcgen.Queries) error {
		_, err := q.DeleteLoginThrottle(ctx, identifier)
		return err
	})
}

// sessionRecord converts the columns of a session row. A null revocation is the zero time.
func sessionRecord(id, user pgtype.UUID, assurance string, created, expires, idle, lastSeen, revoked pgtype.Timestamptz) identity.SessionRecord {
	rec := identity.SessionRecord{
		ID: fromPGUUID[identity.SessionID](id), UserID: fromPGUUID[identity.UserID](user), AssuranceLevel: assurance,
		CreatedAt: instant(created), ExpiresAt: instant(expires), IdleExpiresAt: instant(idle), LastSeenAt: instant(lastSeen),
	}
	if revoked.Valid {
		rec.RevokedAt = instant(revoked)
	}
	return rec
}
