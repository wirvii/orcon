// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package isolation

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/audit/audittest"
	"github.com/wirvii/orcon/internal/identity"
)

// Sessions, passwords and the sign-in throttle (SPEC-004 R-ID-06, R-AUTH-02). They are
// instance tables, keyed by user or by identifier: a user never reads or revokes the
// session of another user, and the throttle state of one identifier is not another's.

// tokenDigest is what the session table keeps of a token.
func tokenDigest(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// sessionOf is a live session of user, started at seededAt.
func sessionOf(user identity.UserRecord, session identity.SessionID, token string) identity.SessionRecord {
	return identity.SessionRecord{
		ID: session, UserID: user.ID, TokenHash: tokenDigest(token), AssuranceLevel: identity.AssuranceAAL1,
		CreatedAt: seededAt, ExpiresAt: seededAt.Add(7 * 24 * time.Hour), IdleExpiresAt: seededAt.Add(8 * time.Hour),
		LastSeenAt: seededAt,
	}
}

// caseCreateSession: a session is stored once per token, and only its owner reads it.
func caseCreateSession(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	rec := sessionOf(env.B.User, fresh[identity.SessionID](env), "b-token-create")
	if err := env.Stores.CreateSession(ctx, rec, audittest.Instance(tb, "login.succeeded")); err != nil {
		tb.Fatalf("create a session of B: %v", err)
	}
	again := sessionOf(env.B.User, fresh[identity.SessionID](env), "b-token-create")
	wantErr(tb, "a second session with the same token", env.Stores.CreateSession(ctx, again, audittest.Instance(tb, "login.succeeded")), identity.ErrConflict)
	if _, err := env.Stores.GetSession(ctx, env.A.User.ID, rec.ID); !errors.Is(err, identity.ErrNotFound) {
		tb.Fatalf("A reads the session of B: %v; want not found", err)
	}
}

// caseGetSession: the owner reads its session; another user gets not found.
func caseGetSession(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	rec := sessionOf(env.B.User, fresh[identity.SessionID](env), "b-token-get")
	if err := env.Stores.CreateSession(ctx, rec, audittest.Instance(tb, "login.succeeded")); err != nil {
		tb.Fatalf("create a session of B: %v", err)
	}
	got, err := env.Stores.GetSession(ctx, env.B.User.ID, rec.ID)
	if err != nil || got.ID != rec.ID {
		tb.Fatalf("the session of B = %+v, %v", got, err)
	}
	if _, err := env.Stores.GetSession(ctx, env.A.User.ID, rec.ID); !errors.Is(err, identity.ErrNotFound) {
		tb.Fatalf("A reads the session of B: %v; want not found", err)
	}
}

// caseGetSessionByTokenHash: a token resolves to its session and user; an unknown token resolves to nothing.
func caseGetSessionByTokenHash(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	rec := sessionOf(env.B.User, fresh[identity.SessionID](env), "b-token-hash")
	if err := env.Stores.CreateSession(ctx, rec, audittest.Instance(tb, "login.succeeded")); err != nil {
		tb.Fatalf("create a session of B: %v", err)
	}
	view, err := env.Stores.GetSessionByTokenHash(ctx, tokenDigest("b-token-hash"))
	if err != nil || view.Session.ID != rec.ID || view.Session.UserID != env.B.User.ID {
		tb.Fatalf("the session of the token of B = %+v, %v", view, err)
	}
	if _, err := env.Stores.GetSessionByTokenHash(ctx, tokenDigest("no-such-token")); !errors.Is(err, identity.ErrNotFound) {
		tb.Fatalf("an unknown token resolves to %v; want not found", err)
	}
}

// caseUpdateSessionActivity: activity moves a live session; a revoked one is not found.
func caseUpdateSessionActivity(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	rec := sessionOf(env.B.User, fresh[identity.SessionID](env), "b-token-activity")
	if err := env.Stores.CreateSession(ctx, rec, audittest.Instance(tb, "login.succeeded")); err != nil {
		tb.Fatalf("create a session of B: %v", err)
	}
	later := seededAt.Add(time.Minute)
	if err := env.Stores.UpdateSessionActivity(ctx, rec.ID, later, later.Add(8*time.Hour)); err != nil {
		tb.Fatalf("update the activity of a live session: %v", err)
	}
	if err := env.Stores.MarkSessionRevoked(ctx, env.B.User.ID, rec.ID, later, audittest.Instance(tb, "logout")); err != nil {
		tb.Fatalf("revoke the session of B: %v", err)
	}
	if err := env.Stores.UpdateSessionActivity(ctx, rec.ID, later.Add(time.Minute), later.Add(9*time.Hour)); !errors.Is(err, identity.ErrNotFound) {
		tb.Fatalf("activity of a revoked session: %v; want not found", err)
	}
}

// caseMarkSessionRevoked: only the owner revokes a session, and only once.
func caseMarkSessionRevoked(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	rec := sessionOf(env.B.User, fresh[identity.SessionID](env), "b-token-revoke")
	if err := env.Stores.CreateSession(ctx, rec, audittest.Instance(tb, "login.succeeded")); err != nil {
		tb.Fatalf("create a session of B: %v", err)
	}
	if err := env.Stores.MarkSessionRevoked(ctx, env.A.User.ID, rec.ID, seededAt, audittest.Instance(tb, "logout")); !errors.Is(err, identity.ErrNotFound) {
		tb.Fatalf("A revokes the session of B: %v; want not found", err)
	}
	if err := env.Stores.MarkSessionRevoked(ctx, env.B.User.ID, rec.ID, seededAt, audittest.Instance(tb, "logout")); err != nil {
		tb.Fatalf("B revokes its own session: %v", err)
	}
	if err := env.Stores.MarkSessionRevoked(ctx, env.B.User.ID, rec.ID, seededAt, audittest.Instance(tb, "logout")); !errors.Is(err, identity.ErrNotFound) {
		tb.Fatalf("a second revocation: %v; want not found", err)
	}
}

// caseSetPasswordHash: the hash of a user is stored; an unknown user is not found.
func caseSetPasswordHash(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	const hash = "$argon2id$v=19$m=64,t=1,p=1$c2FsdHNhbHQ$aGFzaA"
	if err := env.Stores.SetPasswordHash(ctx, env.B.User.ID, hash, seededAt); err != nil {
		tb.Fatalf("set the password of B: %v", err)
	}
	if err := env.Stores.SetPasswordHash(ctx, fresh[identity.UserID](env), hash, seededAt); !errors.Is(err, identity.ErrNotFound) {
		tb.Fatalf("set the password of an unknown user: %v; want not found", err)
	}
}

// caseGetUserCredentialsByEmail: the sign-in path reads the user and its hash; an unknown address is not found.
func caseGetUserCredentialsByEmail(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	got, err := env.Stores.GetUserCredentialsByEmail(ctx, env.B.User.EmailNormalized)
	if err != nil || got.User.ID != env.B.User.ID {
		tb.Fatalf("credentials of B = %+v, %v", got, err)
	}
	if _, err := env.Stores.GetUserCredentialsByEmail(ctx, "nobody@example.test"); !errors.Is(err, identity.ErrNotFound) {
		tb.Fatalf("an unknown address: %v; want not found", err)
	}
}

// caseGetLoginThrottle: the state of an identifier is read back; another identifier has none.
func caseGetLoginThrottle(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	key := []byte("0123456789abcdef")
	if err := env.Stores.PutLoginThrottle(ctx, key, identity.LoginThrottle{Failures: 6, LockedUntil: seededAt, UpdatedAt: seededAt}); err != nil {
		tb.Fatalf("put the throttle state: %v", err)
	}
	got, err := env.Stores.GetLoginThrottle(ctx, key)
	if err != nil || got.Failures != 6 {
		tb.Fatalf("throttle state = %+v, %v; want six failures", got, err)
	}
	if _, err := env.Stores.GetLoginThrottle(ctx, []byte("fedcba9876543210")); !errors.Is(err, identity.ErrNotFound) {
		tb.Fatalf("another identifier has state: %v; want not found", err)
	}
}

// casePutLoginThrottle: a second put replaces the state of the identifier.
func casePutLoginThrottle(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	key := []byte("abcdefghijklmnop")
	if err := env.Stores.PutLoginThrottle(ctx, key, identity.LoginThrottle{Failures: 5, LockedUntil: seededAt, UpdatedAt: seededAt}); err != nil {
		tb.Fatalf("first put: %v", err)
	}
	if err := env.Stores.PutLoginThrottle(ctx, key, identity.LoginThrottle{Failures: 7, LockedUntil: seededAt, UpdatedAt: seededAt}); err != nil {
		tb.Fatalf("second put: %v", err)
	}
	if got, err := env.Stores.GetLoginThrottle(ctx, key); err != nil || got.Failures != 7 {
		tb.Fatalf("throttle state after the second put = %+v, %v; want seven failures", got, err)
	}
}

// caseClearLoginThrottle: a clear removes the state; clearing an unknown identifier is not an error.
func caseClearLoginThrottle(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	key := []byte("qrstuvwxyz012345")
	if err := env.Stores.PutLoginThrottle(ctx, key, identity.LoginThrottle{Failures: 5, LockedUntil: seededAt, UpdatedAt: seededAt}); err != nil {
		tb.Fatalf("put the throttle state: %v", err)
	}
	if err := env.Stores.ClearLoginThrottle(ctx, key); err != nil {
		tb.Fatalf("clear the throttle state: %v", err)
	}
	if _, err := env.Stores.GetLoginThrottle(ctx, key); !errors.Is(err, identity.ErrNotFound) {
		tb.Fatalf("throttle state after the clear: %v; want not found", err)
	}
	if err := env.Stores.ClearLoginThrottle(ctx, key); err != nil {
		tb.Fatalf("clear an unknown identifier: %v", err)
	}
}

// caseIncrementLoginThrottle: every failed attempt is one count, and the count is read back from the store.
func caseIncrementLoginThrottle(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	key := []byte("counts-counts-ab")
	for want := 1; want <= 3; want++ {
		got, err := env.Stores.IncrementLoginThrottle(ctx, key, seededAt)
		if err != nil || got != want {
			tb.Fatalf("failure %d counted as %d, %v", want, got, err)
		}
	}
}

// caseLockLoginThrottle: a lock is set on a counted identifier; an unknown one is not found.
func caseLockLoginThrottle(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	key := []byte("lockable-id-0123")
	if _, err := env.Stores.IncrementLoginThrottle(ctx, key, seededAt); err != nil {
		tb.Fatalf("count: %v", err)
	}
	until := seededAt.Add(time.Minute)
	if err := env.Stores.LockLoginThrottle(ctx, key, until, seededAt); err != nil {
		tb.Fatalf("lock: %v", err)
	}
	got, err := env.Stores.GetLoginThrottle(ctx, key)
	if err != nil || !got.LockedUntil.Equal(until) {
		tb.Fatalf("locked until = %v, %v; want %v", got.LockedUntil, err, until)
	}
	if err := env.Stores.LockLoginThrottle(ctx, []byte("never-counted-id"), until, seededAt); !errors.Is(err, identity.ErrNotFound) {
		tb.Fatalf("lock of an unknown identifier: %v, want not found", err)
	}
}

// caseListLiveSessions: a user lists only its own live sessions, newest first; revoked and expired
// sessions and the sessions of another user are never listed.
func caseListLiveSessions(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	live := sessionOf(env.B.User, fresh[identity.SessionID](env), "b-token-list-live")
	newer := sessionOf(env.B.User, fresh[identity.SessionID](env), "b-token-list-newer")
	newer.CreatedAt = seededAt.Add(time.Minute)
	revoked := sessionOf(env.B.User, fresh[identity.SessionID](env), "b-token-list-revoked")
	expired := sessionOf(env.B.User, fresh[identity.SessionID](env), "b-token-list-expired")
	expired.ExpiresAt = seededAt.Add(time.Hour)
	for _, rec := range []*identity.SessionRecord{&live, &newer, &revoked, &expired} {
		if err := env.Stores.CreateSession(ctx, *rec, audittest.Instance(tb, "login.succeeded")); err != nil {
			tb.Fatalf("create a session of B: %v", err)
		}
	}
	if err := env.Stores.MarkSessionRevoked(ctx, env.B.User.ID, revoked.ID, seededAt, audittest.Instance(tb, "session.revoked")); err != nil {
		tb.Fatalf("revoke a session of B: %v", err)
	}
	now := seededAt.Add(2 * time.Hour)

	got, err := env.Stores.ListLiveSessions(ctx, env.B.User.ID, now, 50)
	if err != nil {
		tb.Fatalf("list the sessions of B: %v", err)
	}
	for i := range got {
		if got[i].ID == revoked.ID || got[i].ID == expired.ID || got[i].UserID != env.B.User.ID {
			tb.Fatalf("the list holds a session that is not a live session of B: %+v", got[i])
		}
	}
	if len(got) < 2 || got[0].ID != newer.ID {
		tb.Fatalf("the list = %+v; want the newer session of B first, then the older live ones", got)
	}
	if last := got[len(got)-1]; last.ID != live.ID && last.CreatedAt.After(live.CreatedAt) {
		tb.Fatalf("the list is not newest first: %+v", got)
	}
	if one, err := env.Stores.ListLiveSessions(ctx, env.B.User.ID, now, 1); err != nil || len(one) != 1 || one[0].ID != newer.ID {
		tb.Fatalf("the list with a limit of one = %+v, %v; want the newer session", one, err)
	}
	mine, err := env.Stores.ListLiveSessions(ctx, env.A.User.ID, now, 50)
	if err != nil {
		tb.Fatalf("list the sessions of A: %v", err)
	}
	for i := range mine {
		if mine[i].UserID != env.A.User.ID {
			tb.Fatalf("the sessions of A hold a session of another user: %+v", mine[i])
		}
	}
	noLeak(tb, "the sessions of A", mine, nil, &env.B)
}

// caseChangeDisplayName: the name changes only for the update instant that was read, an unknown
// user is not found, and a change of one user never touches another.
func caseChangeDisplayName(ctx context.Context, tb testing.TB, env *Env) {
	tb.Helper()
	u := identity.UserRecord{
		ID: fresh[identity.UserID](env), PrincipalID: fresh[identity.PrincipalID](env),
		EmailNormalized: "rename-" + env.IDs.New().String() + "@example.test", DisplayName: "Before", Status: identity.UserActive,
		CreatedAt: seededAt, UpdatedAt: seededAt,
	}
	if err := env.Stores.CreateUser(ctx, u); err != nil {
		tb.Fatalf("create the user to rename: %v", err)
	}
	later := seededAt.Add(time.Minute)
	if err := env.Stores.ChangeDisplayName(ctx, u.ID, "After", seededAt, later, audittest.Instance(tb, "account.renamed")); err != nil {
		tb.Fatalf("rename the user: %v", err)
	}
	got, err := env.Stores.GetUser(ctx, u.ID)
	if err != nil || got.DisplayName != "After" || !got.UpdatedAt.Equal(later) {
		tb.Fatalf("the renamed user = %+v, %v; want the new name at the new instant", got, err)
	}
	wantErr(tb, "a stale update instant", env.Stores.ChangeDisplayName(ctx, u.ID, "Stale", seededAt, later.Add(time.Minute), audittest.Instance(tb, "account.renamed")), identity.ErrConflict)
	wantErr(tb, "an unknown user", env.Stores.ChangeDisplayName(ctx, fresh[identity.UserID](env), "Ghost", seededAt, later, audittest.Instance(tb, "account.renamed")), identity.ErrNotFound)
	if again, err := env.Stores.GetUser(ctx, u.ID); err != nil || again.DisplayName != "After" {
		tb.Fatalf("a refused rename changed the user: %+v, %v", again, err)
	}
	if b, err := env.Stores.GetUser(ctx, env.B.User.ID); err != nil || b.DisplayName != env.B.User.DisplayName {
		tb.Fatalf("the user of B changed: %+v, %v", b, err)
	}
}
