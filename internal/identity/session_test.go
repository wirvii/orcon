// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/platform/id"
)

// TestAuthenticateResolvesLiveSession (SPEC-004 R-ID-06): a live session cookie becomes a
// user principal that names the session, never the token.
func TestAuthenticateResolvesLiveSession(t *testing.T) {
	f := newFixture(t)
	user := f.account("ada@example.test", "correct horse battery")
	signed := f.signIn(t, "ada@example.test", "correct horse battery")

	p, err := f.sessions.Authenticate(t.Context(), Credentials{Session: signed.Token})
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if p.ID != user.PrincipalID || p.Kind != PrincipalUser || p.UserID != user.ID {
		t.Errorf("principal = %+v, want the user principal of %s", p, user.ID)
	}
	if p.CredentialType != CredentialSession || p.CredentialID != signed.Session.ID.String() {
		t.Errorf("credential = %s %q, want the session", p.CredentialType, p.CredentialID)
	}
}

// TestAuthenticateRejectsDeadSessions (SPEC-004 R-ID-06): a session ends at its maximum age,
// at its idle timeout, when revoked, or when its account is disabled. Each is unauthenticated.
func TestAuthenticateRejectsDeadSessions(t *testing.T) {
	cases := map[string]struct {
		setup func(t *testing.T, f *fixture, token string, user UserRecord)
	}{
		"past the maximum age": {setup: func(t *testing.T, f *fixture, _ string, _ UserRecord) {
			t.Helper()
			f.clock.Advance(7*24*time.Hour + time.Minute)
		}},
		"past the idle timeout": {setup: func(t *testing.T, f *fixture, _ string, _ UserRecord) {
			t.Helper()
			f.clock.Advance(8*time.Hour + time.Second)
		}},
		"revoked by sign-out": {setup: func(t *testing.T, f *fixture, token string, user UserRecord) {
			t.Helper()
			sid := f.sessionIDOf(t, user, token)
			if err := f.sessions.Revoke(t.Context(), Principal{ID: user.PrincipalID, UserID: user.ID}, sid); err != nil {
				t.Fatalf("revoke: %v", err)
			}
		}},
		"account disabled": {setup: func(t *testing.T, f *fixture, _ string, user UserRecord) {
			t.Helper()
			u := f.store.users[user.ID]
			u.Status = UserDisabled
			f.store.users[user.ID] = u
		}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			user := f.account("ada@example.test", "correct horse battery")
			signed := f.signIn(t, "ada@example.test", "correct horse battery")
			tc.setup(t, f, signed.Token, user)

			_, err := f.sessions.Authenticate(t.Context(), Credentials{Session: signed.Token})
			if !errors.Is(err, ErrUnauthenticated) {
				t.Fatalf("err = %v, want ErrUnauthenticated", err)
			}
		})
	}
}

// sessionIDOf returns the identity of the session that the token opened.
func (f *fixture) sessionIDOf(t *testing.T, user UserRecord, token string) SessionID {
	t.Helper()
	sessions := f.store.sessionsOf(user.ID)
	for i := range sessions {
		if bytes.Equal(sessions[i].TokenHash, tokenHash(token)) {
			return sessions[i].ID
		}
	}
	t.Fatal("no session holds the token")
	return SessionID{}
}

// TestAuthenticateWithoutLiveSessionIsUnauthenticated: no cookie, an unknown token, and a
// session whose user row is gone all answer unauthenticated, not an internal error.
func TestAuthenticateWithoutLiveSessionIsUnauthenticated(t *testing.T) {
	f := newFixture(t)
	f.account("ada@example.test", "correct horse battery")

	for name, creds := range map[string]Credentials{
		"no cookie":     {},
		"unknown token": {Session: "never-issued"},
	} {
		if _, err := f.sessions.Authenticate(t.Context(), creds); !errors.Is(err, ErrUnauthenticated) {
			t.Errorf("%s: err = %v, want ErrUnauthenticated", name, err)
		}
	}
}

// TestAuthenticateTouchesSessionAtMostOncePerInterval: activity moves the idle deadline,
// but a busy session writes at most once a minute, and the deadline never passes the maximum age.
func TestAuthenticateTouchesSessionAtMostOncePerInterval(t *testing.T) {
	f := newFixture(t)
	f.account("ada@example.test", "correct horse battery")
	signed := f.signIn(t, "ada@example.test", "correct horse battery")
	sid := signed.Session.ID

	f.clock.Advance(30 * time.Second)
	if _, err := f.sessions.Authenticate(t.Context(), Credentials{Session: signed.Token}); err != nil {
		t.Fatalf("authenticate within the interval: %v", err)
	}
	if got := f.store.sessions[sid].LastSeenAt; !got.Equal(fixtureStart) {
		t.Errorf("last seen moved within a minute: %s", got)
	}

	f.clock.Advance(2 * time.Minute)
	if _, err := f.sessions.Authenticate(t.Context(), Credentials{Session: signed.Token}); err != nil {
		t.Fatalf("authenticate after the interval: %v", err)
	}
	touched := f.store.sessions[sid]
	if !touched.LastSeenAt.Equal(f.clock.Now()) {
		t.Errorf("last seen = %s, want the current instant", touched.LastSeenAt)
	}
	if !touched.IdleExpiresAt.Equal(f.clock.Now().Add(8 * time.Hour)) {
		t.Errorf("idle deadline = %s, want eight hours from the activity", touched.IdleExpiresAt)
	}

	// Activity late in the session cannot push the idle deadline past the maximum age.
	// Each step stays inside the eight-hour idle timeout, so the session survives to day seven.
	for range 23 {
		f.clock.Advance(7 * time.Hour)
		if _, err := f.sessions.Authenticate(t.Context(), Credentials{Session: signed.Token}); err != nil {
			t.Fatalf("authenticate while active: %v", err)
		}
	}
	f.clock.Advance(6 * time.Hour)
	if _, err := f.sessions.Authenticate(t.Context(), Credentials{Session: signed.Token}); err != nil {
		t.Fatalf("authenticate late in the session: %v", err)
	}
	late := f.store.sessions[sid]
	if !late.IdleExpiresAt.Equal(late.ExpiresAt) {
		t.Errorf("idle deadline = %s, want it capped at the maximum age %s", late.IdleExpiresAt, late.ExpiresAt)
	}
}

// TestAuthenticateTouchFailures: a session that disappears between the read and the touch is
// unauthenticated; any other storage failure is an internal error.
func TestAuthenticateTouchFailures(t *testing.T) {
	t.Run("session gone at touch", func(t *testing.T) {
		f := newFixture(t)
		f.account("ada@example.test", "correct horse battery")
		signed := f.signIn(t, "ada@example.test", "correct horse battery")
		f.clock.Advance(2 * time.Minute)
		f.store.errUpdateActivity = ErrNotFound
		if _, err := f.sessions.Authenticate(t.Context(), Credentials{Session: signed.Token}); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("err = %v, want ErrUnauthenticated", err)
		}
	})
	t.Run("storage fails at touch", func(t *testing.T) {
		f := newFixture(t)
		f.account("ada@example.test", "correct horse battery")
		signed := f.signIn(t, "ada@example.test", "correct horse battery")
		f.clock.Advance(2 * time.Minute)
		f.store.errUpdateActivity = errors.New("write failed")
		_, err := f.sessions.Authenticate(t.Context(), Credentials{Session: signed.Token})
		if err == nil || errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("err = %v, want the storage failure", err)
		}
	})
	t.Run("storage fails on the read", func(t *testing.T) {
		f := newFixture(t)
		f.store.errGetTokenHash = errors.New("connection reset")
		_, err := f.sessions.Authenticate(t.Context(), Credentials{Session: "anything"})
		if err == nil || errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("err = %v, want the storage failure", err)
		}
	})
}

// TestAuthenticateUnsafeRequestNeedsCSRF: a state-changing request must repeat the CSRF value
// of its session and the origin of the installation; a request with a stale token is refused.
func TestAuthenticateUnsafeRequestNeedsCSRF(t *testing.T) {
	f := newFixture(t)
	f.account("ada@example.test", "correct horse battery")
	signed := f.signIn(t, "ada@example.test", "correct horse battery")

	good := Credentials{
		Session: signed.Token, CSRFToken: signed.CSRFToken, CSRFCookie: signed.CSRFToken,
		Origin: fixtureOrigin, UnsafeMethod: true,
	}
	if _, err := f.sessions.Authenticate(t.Context(), good); err != nil {
		t.Fatalf("a complete unsafe request: %v", err)
	}
	stale := good
	stale.CSRFToken = CSRFTokenFor("another session")
	stale.CSRFCookie = stale.CSRFToken
	if _, err := f.sessions.Authenticate(t.Context(), stale); !errors.Is(err, ErrCSRFCheckFailed) {
		t.Errorf("stale CSRF value: %v, want ErrCSRFCheckFailed", err)
	}
	empty := good
	empty.CSRFToken, empty.CSRFCookie = "", ""
	if _, err := f.sessions.Authenticate(t.Context(), empty); !errors.Is(err, ErrCSRFCheckFailed) {
		t.Errorf("empty CSRF value: %v, want ErrCSRFCheckFailed", err)
	}
}

// TestRevokeEndsOwnSessionOnly (SPEC-016 R-SH-SES-04): the user can end their own session and
// the sign-out is audited under their principal. Another user's session is not found, and a
// session that is already revoked is not found either.
func TestRevokeEndsOwnSessionOnly(t *testing.T) {
	f := newFixture(t)
	ada := f.account("ada@example.test", "correct horse battery")
	bob := f.account("bob@example.test", "another long password")
	adaSession := f.signIn(t, "ada@example.test", "correct horse battery")
	bobSession := f.signIn(t, "bob@example.test", "another long password")

	adaPrincipal := Principal{ID: ada.PrincipalID, Kind: PrincipalUser, UserID: ada.ID}
	bobPrincipal := Principal{ID: bob.PrincipalID, Kind: PrincipalUser, UserID: bob.ID}

	if err := f.sessions.Revoke(t.Context(), bobPrincipal, adaSession.Session.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another user's session: %v, want ErrNotFound", err)
	}
	if !f.store.sessions[adaSession.Session.ID].RevokedAt.IsZero() {
		t.Fatal("another user revoked the session")
	}

	if err := f.sessions.Revoke(t.Context(), adaPrincipal, adaSession.Session.ID); err != nil {
		t.Fatalf("own session: %v", err)
	}
	if f.store.sessions[adaSession.Session.ID].RevokedAt.IsZero() {
		t.Error("own session is still live after sign-out")
	}
	logouts := f.store.entriesWith("logout")
	if len(logouts) != 1 || logouts[0].Actor.PrincipalID != id.UUID(ada.PrincipalID) || logouts[0].Actor.Kind != audit.ActorUser {
		t.Errorf("logout entries = %+v, want one under the user's principal", logouts)
	}
	if err := f.sessions.Revoke(t.Context(), adaPrincipal, adaSession.Session.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("revoking twice: %v, want ErrNotFound", err)
	}
	if err := f.sessions.Revoke(t.Context(), bobPrincipal, bobSession.Session.ID); err != nil {
		t.Errorf("bob's own session: %v", err)
	}
}

// TestCurrentReturnsUserAndSession: the signed-in user and the session of a principal; a
// principal whose session identity is unreadable or belongs to someone else is not found.
func TestCurrentReturnsUserAndSession(t *testing.T) {
	f := newFixture(t)
	ada := f.account("ada@example.test", "correct horse battery")
	signed := f.signIn(t, "ada@example.test", "correct horse battery")
	p, err := f.sessions.Authenticate(t.Context(), Credentials{Session: signed.Token})
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}

	current, err := f.sessions.Current(t.Context(), p)
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	if current.User.ID != ada.ID || current.User.EmailNormalized != "ada@example.test" {
		t.Errorf("user = %+v, want ada", current.User)
	}
	if current.Session.ID != signed.Session.ID {
		t.Errorf("session = %s, want %s", current.Session.ID, signed.Session.ID)
	}

	unreadable := Principal{ID: p.ID, Kind: PrincipalUser, UserID: ada.ID, CredentialID: "not-a-uuid"}
	if _, err := f.sessions.Current(t.Context(), unreadable); !errors.Is(err, ErrNotFound) {
		t.Errorf("unreadable session identity: %v, want ErrNotFound", err)
	}
	other := f.store.addUser("bob@example.test", "", UserActive)
	stranger := Principal{ID: other.PrincipalID, Kind: PrincipalUser, UserID: other.ID, CredentialID: p.CredentialID}
	if _, err := f.sessions.Current(t.Context(), stranger); !errors.Is(err, ErrNotFound) {
		t.Errorf("another user's session: %v, want ErrNotFound", err)
	}
}

func TestCurrentReportsStorageFailures(t *testing.T) {
	f := newFixture(t)
	ada := f.account("ada@example.test", "correct horse battery")
	signed := f.signIn(t, "ada@example.test", "correct horse battery")
	p := Principal{ID: ada.PrincipalID, Kind: PrincipalUser, UserID: ada.ID, CredentialID: signed.Session.ID.String()}

	f.store.errGetSession = errors.New("read failed")
	if _, err := f.sessions.Current(t.Context(), p); err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("session read failure: %v", err)
	}
	f.store.errGetSession = nil
	f.store.errGetUser = errors.New("read failed")
	if _, err := f.sessions.Current(t.Context(), p); err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("user read failure: %v", err)
	}
}

// TestNewSessionsRequiresAuditAndAppliesDefaults (SPEC-004 R-AUD-02): the service refuses to
// run without an audit log, and a zero policy takes the defaults of R-ID-06 and R-AUTH-02.
func TestNewSessionsRequiresAuditAndAppliesDefaults(t *testing.T) {
	f := newFixture(t)
	if _, err := NewSessions(SessionDeps{
		Users: f.store, Sessions: f.store, Throttle: f.store, Hasher: f.hasher, Identify: f.identify,
		Clock: f.clock, IDs: f.ids,
	}, SessionConfig{}); err == nil {
		t.Fatal("a service without an audit log was built")
	}

	svc := f.newSessions(t, SessionDeps{})
	if svc.policy != DefaultSessionPolicy() || svc.attempts != DefaultThrottlePolicy() {
		t.Errorf("policies = %+v / %+v, want the defaults", svc.policy, svc.attempts)
	}
}

func TestNewSessionsReportsHashFailure(t *testing.T) {
	f := newFixture(t)
	f.hasher.errHash = errors.New("argon2 out of memory")
	_, err := NewSessions(SessionDeps{
		Users: f.store, Sessions: f.store, Throttle: f.store, Hasher: f.hasher, Identify: f.identify,
		Clock: f.clock, IDs: f.ids, Audit: f.store,
	}, SessionConfig{})
	if err == nil {
		t.Fatal("the dummy hash failure was ignored")
	}
}

// TestSignInErrorHidesTheReason: a refusal names no account in its text, and it unwraps to
// the shared sentinel, so callers branch on the sentinel and never on the message.
func TestSignInErrorHidesTheReason(t *testing.T) {
	refused := &SignInError{Reason: ReasonWrongPassword}
	if refused.Error() != ErrSignInError.Error() {
		t.Errorf("message = %q, want the shared sentinel text", refused.Error())
	}
	if !errors.Is(refused, ErrSignInError) {
		t.Error("the refusal does not unwrap to ErrSignInError")
	}
}
