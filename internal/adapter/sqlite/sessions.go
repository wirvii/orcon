// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/wirvii/orcon/internal/adapter/sqlite/sqlcgen"
	"github.com/wirvii/orcon/internal/audit"
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

// ChangePassword implements [identity.UserStore]: the new hash, the revocation of every live
// session of the user and the audit entry are written in one transaction.
func (s *Store) ChangePassword(ctx context.Context, user identity.UserID, hash string, now time.Time, entry audit.Entry) error {
	return s.write(ctx, "change password", func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.UpdateApplicationUserPasswordHash(ctx, sql.NullString{String: hash, Valid: true}, micros(now), uuidBytes(user))
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("user %s: %w", user, identity.ErrNotFound)
		}
		if err := q.MarkUserSessionsRevoked(ctx, sql.NullInt64{Int64: micros(now), Valid: true}, uuidBytes(user)); err != nil {
			return err
		}
		return insertPendingInstance(ctx, q, entry)
	})
}

// ChangeDisplayName implements [identity.UserStore]: the new name and the audit entry are written
// in one transaction, and only when the row still has the update instant the caller read.
func (s *Store) ChangeDisplayName(ctx context.Context, user identity.UserID, displayName string, expected, now time.Time, entry audit.Entry) error {
	return s.write(ctx, "change display name", func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.UpdateApplicationUserDisplayName(ctx, sqlcgen.UpdateApplicationUserDisplayNameParams{
			DisplayName: displayName, UpdatedAt: micros(now), ID: uuidBytes(user), ExpectedUpdatedAt: micros(expected),
		})
		if err != nil {
			return err
		}
		if n == 0 {
			if _, err := q.GetApplicationUser(ctx, uuidBytes(user)); err != nil {
				return err
			}
			return fmt.Errorf("user %s changed since it was read: %w", user, identity.ErrConflict)
		}
		return insertPendingInstance(ctx, q, entry)
	})
}

// ListLiveSessions implements [identity.SessionStore].
func (s *Store) ListLiveSessions(ctx context.Context, user identity.UserID, now time.Time, limit int) ([]identity.SessionRecord, error) {
	var out []identity.SessionRecord
	err := s.read(ctx, "list live sessions", func(ctx context.Context, q *sqlcgen.Queries) error {
		rows, err := q.ListLiveSessionsOfUser(ctx, uuidBytes(user), micros(now), int64(limit))
		if err != nil {
			return err
		}
		out = make([]identity.SessionRecord, 0, len(rows))
		for i := range rows {
			r := &rows[i]
			rec, err := sessionRecord(r.ID, r.ApplicationUserID, r.AssuranceLevel, r.CreatedAt, r.ExpiresAt, r.IdleExpiresAt, r.LastSeenAt, r.RevokedAt)
			if err != nil {
				return err
			}
			out = append(out, rec)
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

// CreateSession implements [identity.SessionStore]. The sign-in entry is inserted in the
// same transaction as the session (R-AUD-02).
func (s *Store) CreateSession(ctx context.Context, rec identity.SessionRecord, entry audit.Entry) error {
	return s.write(ctx, "create session", func(ctx context.Context, q *sqlcgen.Queries) error {
		if err := q.InsertSession(ctx, sqlcgen.InsertSessionParams{
			ID: uuidBytes(rec.ID), ApplicationUserID: uuidBytes(rec.UserID), TokenHash: rec.TokenHash,
			AssuranceLevel: rec.AssuranceLevel, CreatedAt: micros(rec.CreatedAt), ExpiresAt: micros(rec.ExpiresAt),
			IdleExpiresAt: micros(rec.IdleExpiresAt), LastSeenAt: micros(rec.LastSeenAt),
		}); err != nil {
			return err
		}
		return insertPendingInstance(ctx, q, entry)
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
func (s *Store) MarkSessionRevoked(ctx context.Context, user identity.UserID, session identity.SessionID, at time.Time, entry audit.Entry) error {
	return s.write(ctx, "revoke session", func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.MarkSessionRevoked(ctx, sql.NullInt64{Int64: micros(at), Valid: true}, uuidBytes(session), uuidBytes(user))
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("session %s: %w", session, identity.ErrNotFound)
		}
		return insertPendingInstance(ctx, q, entry)
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
		out = identity.LoginThrottle{Failures: int(row.Failures), LockedUntil: instant(row.LockedUntilAt), UpdatedAt: instant(row.UpdatedAt)}
		return nil
	})
	return out, err
}

// PutLoginThrottle implements [identity.LoginThrottleStore].
func (s *Store) PutLoginThrottle(ctx context.Context, identifier []byte, t identity.LoginThrottle) error {
	return s.write(ctx, "put login throttle", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.UpsertLoginThrottle(ctx, sqlcgen.UpsertLoginThrottleParams{
			ID: identifier, Failures: int64(t.Failures), LockedUntilAt: micros(t.LockedUntil),
			CreatedAt: micros(t.UpdatedAt), UpdatedAt: micros(t.UpdatedAt),
		})
	})
}

// IncrementLoginThrottle implements [identity.LoginThrottleStore].
func (s *Store) IncrementLoginThrottle(ctx context.Context, identifier []byte, now time.Time) (int, error) {
	var out int
	err := s.write(ctx, "increment login throttle", func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.UpsertLoginThrottleFailure(ctx, identifier, micros(now))
		out = int(n)
		return err
	})
	return out, err
}

// LockLoginThrottle implements [identity.LoginThrottleStore].
func (s *Store) LockLoginThrottle(ctx context.Context, identifier []byte, until, now time.Time) error {
	return s.write(ctx, "lock login throttle", func(ctx context.Context, q *sqlcgen.Queries) error {
		n, err := q.MarkLoginThrottleLocked(ctx, micros(until), micros(now), identifier)
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
