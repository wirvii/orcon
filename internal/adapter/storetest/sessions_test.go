// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/adapter/storetest"
	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/password"
)

// testHasher is argon2id with cheap parameters, so the suite stays fast. The
// service takes its hasher as a port, so production parameters need no test change.
func testHasher() password.Argon2id {
	return password.Argon2id{Memory: 64, Time: 1, Threads: 1, KeyLength: 32, SaltLength: 16}
}

const (
	testPassword = "correct horse battery staple"
	testOrigin   = "https://orcon.test"
)

// sessionsOf builds the session service over the store of the fixture, on a fake clock.
func sessionsOf(t *testing.T, f *fixture, clk *clock.Fake) *identity.Sessions {
	t.Helper()
	identify, err := identity.HMACIdentifier(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatalf("identifier: %v", err)
	}
	svc, err := identity.NewSessions(identity.SessionDeps{
		Users: f.s, Sessions: f.s, Throttle: f.s, Hasher: testHasher(), Identify: identify, Clock: clk, IDs: f.ids,
	}, identity.SessionConfig{PublicOrigin: testOrigin})
	if err != nil {
		t.Fatalf("sessions: %v", err)
	}
	return svc
}

// userWithPassword creates a user whose password is testPassword.
func userWithPassword(t *testing.T, f *fixture, email string) identity.UserRecord {
	t.Helper()
	u := f.user(email)
	hash, err := testHasher().Hash(testPassword)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if err := f.s.SetPasswordHash(f.ctx(), u.ID, hash, base); err != nil {
		t.Fatalf("set password of %s: %v", email, err)
	}
	return u
}

// mustExec runs a statement that changes the instance tables of the store.
func mustExec(t *testing.T, r raw, query string, args ...any) {
	t.Helper()
	if err := r.exec(t.Context(), identity.OrgID{}, query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

// rejection returns the refusal of a sign-in, failing the test when it was not refused.
func rejection(t *testing.T, err error) *identity.SignInError {
	t.Helper()
	var rej *identity.SignInError
	if !errors.As(err, &rej) {
		t.Fatalf("sign-in error = %v, want a refusal", err)
	}
	return rej
}

// TestSignInUniformFailure (E-AUTH-1, SPEC-004 R-AUTH-02): every refusal looks the
// same to the caller, whatever the reason; only the audit reason differs.
func TestSignInUniformFailure(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		clk := clock.NewFake(base)
		svc := sessionsOf(t, f, clk)
		userWithPassword(t, f, "ada@example.test")
		disabled := userWithPassword(t, f, "bob@example.test")
		mustExec(t, rawOf(t, f.s), "UPDATE application_user SET status = 'DISABLED' WHERE id = ?", disabled.ID[:])
		f.user("cara@example.test") // an account without a password

		cases := []struct {
			name   string
			email  string
			pass   string
			reason identity.SignInReason
		}{
			{"unknown account", "nobody@example.test", testPassword, identity.ReasonUnknownAccount},
			{"wrong password", "ada@example.test", "not the password", identity.ReasonWrongPassword},
			{"disabled account", "bob@example.test", testPassword, identity.ReasonDisabled},
			{"no password", "cara@example.test", testPassword, identity.ReasonNoPassword},
		}
		var answers []string
		for _, c := range cases {
			_, err := svc.SignIn(f.ctx(), identity.SignIn{Email: c.email, Password: c.pass})
			rej := rejection(t, err)
			if rej.Reason != c.reason {
				t.Errorf("%s: reason = %s, want %s", c.name, rej.Reason, c.reason)
			}
			if !errors.Is(err, identity.ErrSignInError) {
				t.Errorf("%s: the refusal does not satisfy ErrSignInError", c.name)
			}
			answers = append(answers, err.Error())
		}
		for _, a := range answers[1:] {
			if a != answers[0] {
				t.Errorf("refusals differ: %q and %q", answers[0], a)
			}
		}
	})
}

// TestSignInThrottledUniformly (SPEC-004 R-AUTH-02): after five failures an
// identifier waits, doubling each time, whether or not an account exists; a right
// password after the wait signs in and clears the count.
func TestSignInThrottledUniformly(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		clk := clock.NewFake(base)
		svc := sessionsOf(t, f, clk)
		userWithPassword(t, f, "ada@example.test")

		for _, email := range []string{"ada@example.test", "nobody@example.test"} {
			for i := range 5 {
				_, err := svc.SignIn(f.ctx(), identity.SignIn{Email: email, Password: "wrong"})
				if got := rejection(t, err).Reason; got == identity.ReasonThrottled {
					t.Fatalf("%s attempt %d throttled too early", email, i+1)
				}
			}
			_, err := svc.SignIn(f.ctx(), identity.SignIn{Email: email, Password: testPassword})
			rej := rejection(t, err)
			if rej.Reason != identity.ReasonThrottled {
				t.Fatalf("%s: reason = %s after five failures, want THROTTLED", email, rej.Reason)
			}
			if rej.RetryAfter != 30*time.Second {
				t.Errorf("%s: retry after %s, want 30s", email, rej.RetryAfter)
			}
		}

		clk.Advance(31 * time.Second)
		if _, err := svc.SignIn(f.ctx(), identity.SignIn{Email: "ada@example.test", Password: testPassword}); err != nil {
			t.Fatalf("right password after the wait: %v", err)
		}
		if _, err := svc.SignIn(f.ctx(), identity.SignIn{Email: "ada@example.test", Password: "wrong"}); rejection(t, err).Reason != identity.ReasonWrongPassword {
			t.Errorf("the count was not cleared by the successful sign-in")
		}
	})
}

// TestDisabledUserCannotAuthenticate (SPEC-004 SE5a, §7-2): a disabled account
// neither signs in nor keeps the sessions it already has.
func TestDisabledUserCannotAuthenticate(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		clk := clock.NewFake(base)
		svc := sessionsOf(t, f, clk)
		u := userWithPassword(t, f, "ada@example.test")
		signed, err := svc.SignIn(f.ctx(), identity.SignIn{Email: "ada@example.test", Password: testPassword})
		if err != nil {
			t.Fatalf("sign in: %v", err)
		}
		creds := identity.Credentials{Session: signed.Token}
		if _, err := svc.Authenticate(f.ctx(), creds); err != nil {
			t.Fatalf("live session: %v", err)
		}
		mustExec(t, rawOf(t, f.s), "UPDATE application_user SET status = 'DISABLED' WHERE id = ?", u.ID[:])

		if _, err := svc.Authenticate(f.ctx(), creds); !errors.Is(err, identity.ErrUnauthenticated) {
			t.Errorf("session of a disabled account: %v, want ErrUnauthenticated", err)
		}
		_, err = svc.SignIn(f.ctx(), identity.SignIn{Email: "ada@example.test", Password: testPassword})
		if rejection(t, err).Reason != identity.ReasonDisabled {
			t.Errorf("sign-in of a disabled account: %v, want DISABLED", err)
		}
	})
}

// TestSessionRowStoresOnlyTokenHash (SPEC-004 R-ID-06): the session row keeps the
// SHA-256 of the token, never the token; the password is kept as an argon2id hash.
func TestSessionRowStoresOnlyTokenHash(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		r := rawOf(t, f.s)
		svc := sessionsOf(t, f, clock.NewFake(base))
		userWithPassword(t, f, "ada@example.test")
		signed, err := svc.SignIn(f.ctx(), identity.SignIn{Email: "ada@example.test", Password: testPassword})
		if err != nil {
			t.Fatalf("sign in: %v", err)
		}
		sum := sha256.Sum256([]byte(signed.Token))
		mustCount(t, r, identity.OrgID{}, 1, "SELECT count(*) FROM session WHERE token_hash = ?", sum[:])
		mustCount(t, r, identity.OrgID{}, 0, "SELECT count(*) FROM session WHERE token_hash = ?", []byte(signed.Token))
		mustCount(t, r, identity.OrgID{}, 1, "SELECT count(*) FROM application_user WHERE email_normalized = ? AND password_hash LIKE ?", "ada@example.test", "$argon2id$%")
		mustCount(t, r, identity.OrgID{}, 0, "SELECT count(*) FROM application_user WHERE password_hash = ?", testPassword)
		if signed.CSRFToken == signed.Token {
			t.Errorf("the CSRF value must differ from the session token")
		}
	})
}

// TestSignInRotatesPreviousSession (SPEC-016 R-SH-SES-01): signing in while a
// session is already held revokes that session and starts a new one.
func TestSignInRotatesPreviousSession(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		svc := sessionsOf(t, f, clock.NewFake(base))
		userWithPassword(t, f, "ada@example.test")
		first, err := svc.SignIn(f.ctx(), identity.SignIn{Email: "ada@example.test", Password: testPassword})
		if err != nil {
			t.Fatalf("first sign in: %v", err)
		}
		second, err := svc.SignIn(f.ctx(), identity.SignIn{Email: "ada@example.test", Password: testPassword, PriorToken: first.Token})
		if err != nil {
			t.Fatalf("second sign in: %v", err)
		}
		if _, err := svc.Authenticate(f.ctx(), identity.Credentials{Session: first.Token}); !errors.Is(err, identity.ErrUnauthenticated) {
			t.Errorf("the previous session still authenticates: %v", err)
		}
		if _, err := svc.Authenticate(f.ctx(), identity.Credentials{Session: second.Token}); err != nil {
			t.Errorf("the new session does not authenticate: %v", err)
		}
	})
}

// TestUnsafeRequestWithoutCsrfTokenRejected (SPEC-016 R-SH-SES-03): a state-changing
// request on a cookie session needs the Origin of the installation and a CSRF token
// that matches the session; a safe request needs neither.
func TestUnsafeRequestWithoutCsrfTokenRejected(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		svc := sessionsOf(t, f, clock.NewFake(base))
		userWithPassword(t, f, "ada@example.test")
		signed, err := svc.SignIn(f.ctx(), identity.SignIn{Email: "ada@example.test", Password: testPassword})
		if err != nil {
			t.Fatalf("sign in: %v", err)
		}
		good := identity.Credentials{Session: signed.Token, CSRFToken: signed.CSRFToken, CSRFCookie: signed.CSRFToken, Origin: testOrigin, UnsafeMethod: true}
		if _, err := svc.Authenticate(f.ctx(), good); err != nil {
			t.Fatalf("a complete unsafe request: %v", err)
		}
		bad := map[string]identity.Credentials{
			"no header":        {Session: signed.Token, CSRFCookie: signed.CSRFToken, Origin: testOrigin, UnsafeMethod: true},
			"header not equal": {Session: signed.Token, CSRFToken: "x", CSRFCookie: signed.CSRFToken, Origin: testOrigin, UnsafeMethod: true},
			"other origin":     {Session: signed.Token, CSRFToken: signed.CSRFToken, CSRFCookie: signed.CSRFToken, Origin: "https://evil.test", UnsafeMethod: true},
			"no origin":        {Session: signed.Token, CSRFToken: signed.CSRFToken, CSRFCookie: signed.CSRFToken, UnsafeMethod: true},
		}
		for name, creds := range bad {
			if _, err := svc.Authenticate(f.ctx(), creds); !errors.Is(err, identity.ErrCSRFCheckFailed) {
				t.Errorf("%s: %v, want ErrCSRFCheckFailed", name, err)
			}
		}
		safe := identity.Credentials{Session: signed.Token}
		if _, err := svc.Authenticate(f.ctx(), safe); err != nil {
			t.Errorf("a safe request needs no CSRF token: %v", err)
		}
	})
}
