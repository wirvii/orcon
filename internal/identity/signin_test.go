// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"bytes"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/audit"
)

// signInRefusal returns the reason of a refused sign-in, failing the test when the
// error is not a [SignInError].
func signInRefusal(t *testing.T, err error) *SignInError {
	t.Helper()
	var refused *SignInError
	if !errors.As(err, &refused) {
		t.Fatalf("err = %v, want a *SignInError", err)
	}
	if !errors.Is(err, ErrSignInError) {
		t.Fatalf("refusal does not satisfy ErrSignInError: %v", err)
	}
	return refused
}

// TestSignInStartsSessionForCorrectPassword (SPEC-004 R-ID-06): the sign-in returns the
// token and its CSRF value, stores only the hash of the token, sets the maximum age and
// the idle timeout, audits the success and clears the failure count of the address.
func TestSignInStartsSessionForCorrectPassword(t *testing.T) {
	f := newFixture(t)
	user := f.account("ada@example.test", "correct horse battery")
	key := f.identify("ada@example.test")
	if _, err := f.store.IncrementLoginThrottle(t.Context(), key, f.clock.Now()); err != nil {
		t.Fatalf("seed failure: %v", err)
	}

	signed, err := f.sessions.SignIn(t.Context(), SignIn{Email: "  ADA@example.test ", Password: "correct horse battery"})
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}
	if signed.Token == "" || signed.CSRFToken != CSRFTokenFor(signed.Token) {
		t.Errorf("token %q, csrf %q: the CSRF value must be bound to the token", signed.Token, signed.CSRFToken)
	}
	if signed.User.ID != user.ID {
		t.Errorf("signed-in user = %v, want %v", signed.User.ID, user.ID)
	}
	stored := f.store.sessionsOf(user.ID)
	if len(stored) != 1 {
		t.Fatalf("sessions = %d, want 1", len(stored))
	}
	if !bytes.Equal(stored[0].TokenHash, tokenHash(signed.Token)) {
		t.Error("the stored session does not hold the hash of the token")
	}
	if got := stored[0].ExpiresAt.Sub(fixtureStart); got != 7*24*time.Hour {
		t.Errorf("maximum age = %s, want 7 days", got)
	}
	if got := stored[0].IdleExpiresAt.Sub(fixtureStart); got != 8*time.Hour {
		t.Errorf("idle deadline = %s after sign-in, want 8 hours", got)
	}
	if got := f.store.entriesWith("login.succeeded"); len(got) != 1 || got[0].Outcome != audit.OutcomeSuccess {
		t.Errorf("login.succeeded entries = %d, want one success", len(got))
	}
	if _, ok := f.store.throttleOf(key); ok {
		t.Error("a successful sign-in left the failure count of the address")
	}
}

// TestSignInRefusalsShareOneAnswerAndAudit (SPEC-004 R-AUTH-02): every refusal is a
// SignInError with its reason; the audit entry names the reason and the keyed identifier,
// never the address.
func TestSignInRefusalsShareOneAnswerAndAudit(t *testing.T) {
	const email = "ada@example.test"
	cases := map[string]struct {
		setup   func(f *fixture)
		attempt string
		reason  SignInReason
		actor   bool
	}{
		"wrong password": {
			setup:   func(f *fixture) { f.account(email, "correct horse battery") },
			attempt: "wrong guess here",
			reason:  ReasonWrongPassword, actor: true,
		},
		"unknown account": {
			setup:  func(f *fixture) {},
			reason: ReasonUnknownAccount,
		},
		"disabled account": {
			setup: func(f *fixture) {
				hash, _ := f.hasher.Hash("correct horse battery")
				f.store.addUser(email, hash, UserDisabled)
			},
			reason: ReasonDisabled, actor: true,
		},
		"no password yet": {
			setup: func(f *fixture) {
				f.store.addUser(email, "", UserActive)
			},
			reason: ReasonNoPassword, actor: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			tc.setup(f)
			attempt := tc.attempt
			if attempt == "" {
				attempt = "correct horse battery"
			}
			_, err := f.sessions.SignIn(t.Context(), SignIn{Email: email, Password: attempt})
			refused := signInRefusal(t, err)
			if refused.Reason != tc.reason {
				t.Errorf("reason = %s, want %s", refused.Reason, tc.reason)
			}
			if refused.RetryAfter != 0 {
				t.Errorf("RetryAfter = %s, want none outside the throttle", refused.RetryAfter)
			}
			if got := f.store.sessions; len(got) != 0 {
				t.Errorf("a refused sign-in created %d sessions", len(got))
			}
			entries := f.store.entriesWith("login.failed")
			if len(entries) != 1 || entries[0].Outcome != audit.OutcomeFailure {
				t.Fatalf("login.failed entries = %d, want one failure", len(entries))
			}
			if !bytes.Contains(entries[0].Body, []byte(string(tc.reason))) {
				t.Error("the audit entry does not name the reason")
			}
			if bytes.Contains(entries[0].Body, []byte(email)) {
				t.Error("the audit entry holds the address in clear")
			}
			if tc.actor == entries[0].Actor.IsAnonymous() {
				t.Errorf("actor anonymous = %t, want the user to be named only for a known account", entries[0].Actor.IsAnonymous())
			}
		})
	}
}

// TestSignInUnknownAccountChecksDummyHash (SPEC-004 R-AUTH-02): an address with no account
// still runs one password check, against the dummy hash, so the time does not reveal it.
func TestSignInUnknownAccountChecksDummyHash(t *testing.T) {
	f := newFixture(t)
	_, err := f.sessions.SignIn(t.Context(), SignIn{Email: "ghost@example.test", Password: "anything at all"})
	if got := signInRefusal(t, err).Reason; got != ReasonUnknownAccount {
		t.Errorf("reason = %s, want UNKNOWN_ACCOUNT", got)
	}

	checked := f.hasher.verifiedAgainst()
	if len(checked) != 1 || checked[0] != f.sessions.dummy {
		t.Errorf("checks = %v, want exactly one check against the dummy hash", checked)
	}
}

// TestSignInAccountWithoutPasswordChecksDummyHash: an account with no password yet is
// refused with its own reason, after the same dummy check.
func TestSignInAccountWithoutPasswordChecksDummyHash(t *testing.T) {
	f := newFixture(t)
	f.store.addUser("ada@example.test", "", UserActive)
	_, err := f.sessions.SignIn(t.Context(), SignIn{Email: "ada@example.test", Password: "anything at all"})
	if refused := signInRefusal(t, err); refused.Reason != ReasonNoPassword {
		t.Errorf("reason = %s, want NO_PASSWORD", refused.Reason)
	}
	if checked := f.hasher.verifiedAgainst(); len(checked) != 1 || checked[0] != f.sessions.dummy {
		t.Errorf("checks = %v, want one check against the dummy hash", checked)
	}
}

// TestSignInThrottleLocksThenReleases (SPEC-004 R-AUTH-02): five failures are free, the
// fifth locks the address for 30 s, a refused attempt answers with the wait and does not
// check the password, and the address is released exactly when the wait ends.
func TestSignInThrottleLocksThenReleases(t *testing.T) {
	f := newFixture(t)
	f.account("ada@example.test", "correct horse battery")
	key := f.identify("ada@example.test")

	for i := range 5 {
		_, err := f.sessions.SignIn(t.Context(), SignIn{Email: "ada@example.test", Password: "wrong guess here"})
		if refused := signInRefusal(t, err); refused.Reason != ReasonWrongPassword {
			t.Fatalf("attempt %d: reason = %s, want WRONG_PASSWORD", i+1, refused.Reason)
		}
	}
	state, ok := f.store.throttleOf(key)
	if !ok || state.Failures != 5 {
		t.Fatalf("throttle = %+v, want 5 failures", state)
	}
	if got := state.LockedUntil.Sub(fixtureStart); got != 30*time.Second {
		t.Fatalf("lock = %s after the fifth failure, want 30s", got)
	}

	checksBefore := len(f.hasher.verifiedAgainst())
	_, err := f.sessions.SignIn(t.Context(), SignIn{Email: "ada@example.test", Password: "correct horse battery"})
	refused := signInRefusal(t, err)
	if refused.Reason != ReasonThrottled || refused.RetryAfter != 30*time.Second {
		t.Errorf("throttled answer = %s / %s, want THROTTLED / 30s", refused.Reason, refused.RetryAfter)
	}
	if got := len(f.hasher.verifiedAgainst()); got != checksBefore {
		t.Error("a throttled attempt still checked a password")
	}
	if got := f.store.entriesWith("login.failed"); len(got) != 6 || got[5].Outcome != audit.OutcomeDenied {
		t.Error("the throttled attempt is not audited as a denied attempt")
	}

	f.clock.Advance(10 * time.Second)
	_, err = f.sessions.SignIn(t.Context(), SignIn{Email: "ada@example.test", Password: "correct horse battery"})
	if refused := signInRefusal(t, err); refused.RetryAfter != 20*time.Second {
		t.Errorf("RetryAfter after 10s = %s, want the remaining 20s", refused.RetryAfter)
	}

	f.clock.Advance(20 * time.Second)
	if _, err := f.sessions.SignIn(t.Context(), SignIn{Email: "ada@example.test", Password: "correct horse battery"}); err != nil {
		t.Fatalf("sign in when the wait ends exactly: %v", err)
	}
	if _, ok := f.store.throttleOf(key); ok {
		t.Error("the released address kept its failure count")
	}
}

// TestSignInSecondLockDoublesTheWait: a failure past the free attempts after the wait ends
// locks for twice as long.
func TestSignInSecondLockDoublesTheWait(t *testing.T) {
	f := newFixture(t)
	f.account("ada@example.test", "correct horse battery")
	key := f.identify("ada@example.test")
	for range 5 {
		_, _ = f.sessions.SignIn(t.Context(), SignIn{Email: "ada@example.test", Password: "wrong guess here"})
	}
	f.clock.Advance(30 * time.Second)
	_, err := f.sessions.SignIn(t.Context(), SignIn{Email: "ada@example.test", Password: "wrong guess here"})
	if got := signInRefusal(t, err).Reason; got != ReasonWrongPassword {
		t.Errorf("reason = %s, want WRONG_PASSWORD", got)
	}

	state, _ := f.store.throttleOf(key)
	if state.Failures != 6 {
		t.Fatalf("failures = %d, want 6", state.Failures)
	}
	if got := state.LockedUntil.Sub(f.clock.Now()); got != time.Minute {
		t.Errorf("lock after the sixth failure = %s, want 60s", got)
	}
}

// TestRecordFailureCountsEveryConcurrentFailure (SPEC-004 R-AUTH-02): the count is taken
// with one atomic increment per failure, so parallel failures are never lost and the
// non-atomic write is never used on the failure path.
func TestRecordFailureCountsEveryConcurrentFailure(t *testing.T) {
	f := newFixture(t)
	key := f.identify("ada@example.test")

	const attempts = 50
	var wg sync.WaitGroup
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := f.sessions.recordFailure(t.Context(), key, f.clock.Now()); err != nil {
				t.Errorf("record failure: %v", err)
			}
		}()
	}
	wg.Wait()

	state, ok := f.store.throttleOf(key)
	if !ok || state.Failures != attempts {
		t.Fatalf("failures = %d, want %d", state.Failures, attempts)
	}
	if got := f.store.incrementCount(); got != attempts {
		t.Errorf("atomic increments = %d, want one per failure", got)
	}
	if got := f.store.putCalls(); got != 0 {
		t.Errorf("non-atomic writes = %d, want none", got)
	}
	if state.LockedUntil.IsZero() {
		t.Error("fifty failures left the identifier unlocked")
	}
}

// TestSignInRotatesThePriorSession (SPEC-016 R-SH-SES-01): a sign-in that carries a live
// session revokes it, so every sign-in starts a new session. A prior token that is unknown
// or already revoked changes nothing.
func TestSignInRotatesThePriorSession(t *testing.T) {
	f := newFixture(t)
	user := f.account("ada@example.test", "correct horse battery")

	first := f.signIn(t, "ada@example.test", "correct horse battery")
	second, err := f.sessions.SignIn(t.Context(), SignIn{Email: "ada@example.test", Password: "correct horse battery", PriorToken: first.Token})
	if err != nil {
		t.Fatalf("rotating sign in: %v", err)
	}
	if second.Token == first.Token {
		t.Fatal("the rotated session reuses the old token")
	}
	for _, s := range f.store.sessionsOf(user.ID) {
		switch s.ID {
		case first.Session.ID:
			if s.RevokedAt.IsZero() {
				t.Error("the prior session stayed live after the rotation")
			}
		case second.Session.ID:
			if !s.RevokedAt.IsZero() {
				t.Error("the new session was revoked by its own sign-in")
			}
		}
	}
	if got := len(f.store.entriesWith("session.revoked")); got != 1 {
		t.Errorf("session.revoked entries = %d, want one", got)
	}

	// The prior session is already revoked: rotating from it again revokes nothing more.
	if _, err := f.sessions.SignIn(t.Context(), SignIn{Email: "ada@example.test", Password: "correct horse battery", PriorToken: first.Token}); err != nil {
		t.Fatalf("rotating from a revoked session: %v", err)
	}
	if got := len(f.store.entriesWith("session.revoked")); got != 1 {
		t.Errorf("session.revoked entries = %d after a revoked prior token, want still one", got)
	}

	// The prior token is unknown: the sign-in still succeeds.
	if _, err := f.sessions.SignIn(t.Context(), SignIn{Email: "ada@example.test", Password: "correct horse battery", PriorToken: "never-issued"}); err != nil {
		t.Fatalf("rotating from an unknown token: %v", err)
	}
}

// TestSignInRehashesOutdatedPassword: a hash that the hasher no longer accepts as current is
// replaced by a fresh one of the password that just checked out.
func TestSignInRehashesOutdatedPassword(t *testing.T) {
	f := newFixture(t)
	user := f.account("ada@example.test", "correct horse battery")
	f.hasher.rehash = true
	f.store.hashes[user.ID] = "hash:correct horse battery"
	f.signIn(t, "ada@example.test", "correct horse battery")

	if got := f.store.hashes[user.ID]; got != "hash:correct horse battery" {
		t.Errorf("stored hash = %q, want the rehashed password", got)
	}
}

// TestSignInFailuresOfTheStoreAreInternalErrors: a storage or hashing failure is an error of
// the service, not a refusal the caller answers as wrong credentials.
func TestSignInFailuresOfTheStoreAreInternalErrors(t *testing.T) {
	const email = "ada@example.test"
	const password = "correct horse battery"
	const wrong = "wrong guess here"
	cases := map[string]struct {
		setup   func(f *fixture)
		attempt string // empty means the correct password
	}{
		"throttle read fails": {setup: func(f *fixture) {
			f.store.errGetThrottle = errors.New("read failed")
		}},
		"credentials read fails": {setup: func(f *fixture) {
			f.store.errGetCredentials = errors.New("read failed")
		}},
		"password check fails": {setup: func(f *fixture) {
			f.hasher.errVerify = errors.New("argon2 failed")
		}},
		"failure count fails": {attempt: wrong, setup: func(f *fixture) {
			f.store.errIncrement = errors.New("write failed")
		}},
		"lock fails": {attempt: wrong, setup: func(f *fixture) {
			f.store.errLock = errors.New("write failed")
			key := f.identify(email)
			f.store.throttle[string(key)] = LoginThrottle{Failures: 4}
		}},
		"refusal audit fails": {attempt: wrong, setup: func(f *fixture) {
			f.store.errAppendAudit = errors.New("audit down")
		}},
		"clearing the count fails": {setup: func(f *fixture) {
			f.store.errClearThrottle = errors.New("write failed")
		}},
		"rehash fails": {setup: func(f *fixture) {
			f.hasher.rehash = true
			f.hasher.errHash = errors.New("hash failed")
		}},
		"rehash store fails": {setup: func(f *fixture) {
			f.hasher.rehash = true
			f.store.errSetPasswordHash = errors.New("write failed")
		}},
		"session cannot be stored": {setup: func(f *fixture) {
			f.store.errCreateSession = errors.New("write failed")
		}},
		"token source fails": {setup: func(f *fixture) {
			f.sessions = f.newSessions(t, SessionDeps{Random: errReader{}})
		}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			f.account(email, password)
			tc.setup(f)
			attempt := password
			if tc.attempt != "" {
				attempt = tc.attempt
			}
			_, err := f.sessions.SignIn(t.Context(), SignIn{Email: email, Password: attempt})
			if err == nil {
				t.Fatal("the failure was reported as success")
			}
			if errors.Is(err, ErrSignInError) {
				t.Errorf("internal failure reported as a refusal: %v", err)
			}
		})
	}
}
