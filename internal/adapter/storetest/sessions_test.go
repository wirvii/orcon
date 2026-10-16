// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package storetest_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
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
	return sessionsWith(t, f, clk, identity.SessionConfig{PublicOrigin: testOrigin})
}

// sessionsWith builds the session service with the given configuration.
func sessionsWith(t *testing.T, f *fixture, clk *clock.Fake, cfg identity.SessionConfig) *identity.Sessions {
	t.Helper()
	identify, err := identity.HMACIdentifier(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatalf("identifier: %v", err)
	}
	svc, err := identity.NewSessions(identity.SessionDeps{
		Users: f.s, Sessions: f.s, Throttle: f.s, Hasher: testHasher(), Identify: identify, Clock: clk, IDs: f.ids,
		Audit: f.s,
	}, cfg)
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

// TestParallelFailedAttemptsAreAllCounted (SPEC-004 R-AUTH-02): concurrent wrong passwords for one
// identifier each count once. The lock threshold is out of reach here, so every attempt is a failure.
func TestParallelFailedAttemptsAreAllCounted(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		clk := clock.NewFake(base)
		svc := sessionsWith(t, f, clk, identity.SessionConfig{
			PublicOrigin: testOrigin,
			Attempts:     identity.ThrottlePolicy{FreeAttempts: 1000, BaseDelay: time.Second, MaxDelay: time.Minute},
		})
		userWithPassword(t, f, "ada@example.test")

		const attempts = 40
		var wg sync.WaitGroup
		failures := make(chan error, attempts)
		for range attempts {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := svc.SignIn(t.Context(), identity.SignIn{Email: "ada@example.test", Password: "wrong password"})
				var rej *identity.SignInError
				if !errors.As(err, &rej) || rej.Reason != identity.ReasonWrongPassword {
					failures <- fmt.Errorf("attempt answered %w, want a wrong-password refusal", err)
				}
			}()
		}
		wg.Wait()
		close(failures)
		for err := range failures {
			t.Error(err)
		}

		identify, err := identity.HMACIdentifier(bytes.Repeat([]byte{7}, 32))
		if err != nil {
			t.Fatalf("identifier: %v", err)
		}
		key := identify("ada@example.test")
		got, err := f.s.GetLoginThrottle(f.ctx(), key)
		if err != nil || got.Failures != attempts {
			t.Errorf("stored failures = %d, %v; want %d (no count may be lost)", got.Failures, err, attempts)
		}
	})
}

// TestListAndRevokeOwnSessions (SPEC-004 R-ID-08): a user lists its live sessions, ends another
// one with session.revoked and keeps the current one; the ended session no longer authenticates
// and no longer appears; another user never sees or ends it.
func TestListAndRevokeOwnSessions(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		clk := clock.NewFake(base)
		svc := sessionsOf(t, f, clk)
		ada := userWithPassword(t, f, "ada@example.test")
		bob := userWithPassword(t, f, "bob@example.test")
		laptop, err := svc.SignIn(f.ctx(), identity.SignIn{Email: "ada@example.test", Password: testPassword})
		if err != nil {
			t.Fatalf("sign in: %v", err)
		}
		clk.Advance(time.Minute)
		phone, err := svc.SignIn(f.ctx(), identity.SignIn{Email: "ada@example.test", Password: testPassword})
		if err != nil {
			t.Fatalf("second sign in: %v", err)
		}
		if _, err := svc.SignIn(f.ctx(), identity.SignIn{Email: "bob@example.test", Password: testPassword}); err != nil {
			t.Fatalf("bob signs in: %v", err)
		}
		me := identity.Principal{ID: ada.PrincipalID, Kind: identity.PrincipalUser, UserID: ada.ID, CredentialID: laptop.Session.ID.String()}
		bobMe := identity.Principal{ID: bob.PrincipalID, Kind: identity.PrincipalUser, UserID: bob.ID}

		got, err := svc.ListMine(f.ctx(), me)
		if err != nil || len(got) != 2 || got[0].ID != phone.Session.ID || got[1].ID != laptop.Session.ID {
			t.Fatalf("listed %+v, %v; want the phone then the laptop", got, err)
		}
		if err := svc.Revoke(f.ctx(), bobMe, phone.Session.ID); !errors.Is(err, identity.ErrNotFound) {
			t.Fatalf("bob ends the phone of ada: %v, want not found", err)
		}
		if err := svc.Revoke(f.ctx(), me, phone.Session.ID); err != nil {
			t.Fatalf("ada ends her phone: %v", err)
		}
		if _, err := svc.Authenticate(f.ctx(), identity.Credentials{Session: phone.Token}); !errors.Is(err, identity.ErrUnauthenticated) {
			t.Errorf("the ended session still authenticates: %v", err)
		}
		if _, err := svc.Authenticate(f.ctx(), identity.Credentials{Session: laptop.Token}); err != nil {
			t.Errorf("the current session stopped authenticating: %v", err)
		}
		if got, err = svc.ListMine(f.ctx(), me); err != nil || len(got) != 1 || got[0].ID != laptop.Session.ID {
			t.Errorf("listed after the revocation: %+v, %v; want only the laptop", got, err)
		}
		r := rawOf(t, f.s)
		mustCount(t, r, identity.OrgID{}, 1, "SELECT count(*) FROM instance_audit_pending WHERE action_name = ?", "session.revoked")
	})
}

// TestRenameOwnAccount (SPEC-019 F-04): the name changes under the revision the user read, the
// next write with the old revision is refused, and the change is audited as account.renamed.
func TestRenameOwnAccount(t *testing.T) {
	forEachStore(t, func(t *testing.T, newStore storetest.Factory) {
		t.Helper()
		f := newFixture(t, newStore)
		clk := clock.NewFake(base)
		svc := sessionsOf(t, f, clk)
		ada := userWithPassword(t, f, "ada@example.test")
		read, err := f.s.GetUser(f.ctx(), ada.ID)
		if err != nil {
			t.Fatalf("read the user: %v", err)
		}
		me := identity.Principal{ID: ada.PrincipalID, Kind: identity.PrincipalUser, UserID: ada.ID}

		clk.Advance(time.Minute)
		renamed, err := svc.Rename(f.ctx(), me, "Ada Lovelace", read.UpdatedAt)
		if err != nil {
			t.Fatalf("rename: %v", err)
		}
		if renamed.DisplayName != "Ada Lovelace" || !renamed.UpdatedAt.After(read.UpdatedAt) {
			t.Errorf("renamed = %+v, want the new name at a later instant", renamed)
		}
		if _, err := svc.Rename(f.ctx(), me, "Stale", read.UpdatedAt); !errors.Is(err, identity.ErrConflict) {
			t.Errorf("rename with the old revision: %v, want conflict", err)
		}
		r := rawOf(t, f.s)
		mustCount(t, r, identity.OrgID{}, 1, "SELECT count(*) FROM application_user WHERE id = ? AND display_name = ?", ada.ID[:], "Ada Lovelace")
		mustCount(t, r, identity.OrgID{}, 1, "SELECT count(*) FROM instance_audit_pending WHERE action_name = ?", "account.renamed")
	})
}
