// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/audit"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
)

// fakeStore is an in-memory implementation of the storage ports the account and session
// services use. It keeps the rules the adapters enforce (unique addresses, one system
// principal, one session per token hash, ownership of sessions by user) so the services
// are tested against the same contract the database gives them. Every method is safe
// for concurrent use.
type fakeStore struct {
	mu sync.Mutex

	users      map[UserID]UserRecord
	hashes     map[UserID]string
	principals map[PrincipalID]PrincipalRecord
	sessions   map[SessionID]SessionRecord
	throttle   map[string]LoginThrottle
	entries    []audit.Entry

	// putThrottleCalls counts the non-atomic writes of the throttle; the failure
	// path must never use them (SPEC-004 R-AUTH-02).
	putThrottleCalls int
	// incrementCalls counts the atomic failure counter updates.
	incrementCalls int

	// raceSystemPrincipal makes CreatePrincipal behave as if another writer had
	// created the system principal first: it stores that principal and reports a
	// conflict.
	raceSystemPrincipal bool

	// ids creates the identities of the accounts the tests seed. The fixture shares
	// its generator, so no two rows of one test ever get the same identity.
	ids *id.Sequence

	// Injected failures. A nil value means the call succeeds.
	errGetTokenHash       error
	errGetSession         error
	errGetUser            error
	errGetThrottle        error
	errGetCredentials     error
	errGetSystemPrincipal error
	errCreatePrincipal    error
	errCreateUserEntry    error
	errChangePassword     error
	errUpdateActivity     error
	errSetPasswordHash    error
	errClearThrottle      error
	errAppendAudit        error
	errIncrement          error
	errLock               error
	errCreateSession      error
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		users:      map[UserID]UserRecord{},
		hashes:     map[UserID]string{},
		principals: map[PrincipalID]PrincipalRecord{},
		sessions:   map[SessionID]SessionRecord{},
		throttle:   map[string]LoginThrottle{},
		ids:        id.NewSequence(),
	}
}

// addUser stores an account with its password hash and returns its identity.
func (f *fakeStore) addUser(email, hash, status string) UserRecord {
	f.mu.Lock()
	defer f.mu.Unlock()
	u := UserRecord{
		ID: UserID(f.ids.New()), PrincipalID: PrincipalID(f.ids.New()),
		EmailNormalized: email, DisplayName: email, Status: status,
	}
	f.users[u.ID] = u
	f.hashes[u.ID] = hash
	f.principals[u.PrincipalID] = PrincipalRecord{ID: u.PrincipalID, Kind: PrincipalUser}
	return u
}

// userByEmail returns the stored user with that address; the caller holds no lock.
func (f *fakeStore) userByEmail(email string) (UserRecord, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.users {
		u := f.users[i]
		if u.EmailNormalized == email {
			return u, true
		}
	}
	return UserRecord{}, false
}

// sessionsOf returns the stored sessions of the user.
func (f *fakeStore) sessionsOf(user UserID) []SessionRecord {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []SessionRecord
	for i := range f.sessions {
		s := f.sessions[i]
		if s.UserID == user {
			out = append(out, s)
		}
	}
	return out
}

// entriesWith returns the audit entries with that action name.
func (f *fakeStore) entriesWith(action string) []audit.Entry {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []audit.Entry
	for i := range f.entries {
		e := f.entries[i]
		if e.Action == action {
			out = append(out, e)
		}
	}
	return out
}

// throttleOf returns the stored throttle state of the identifier.
func (f *fakeStore) throttleOf(key []byte) (LoginThrottle, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	t, ok := f.throttle[string(key)]
	return t, ok
}

func (f *fakeStore) putCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.putThrottleCalls
}

func (f *fakeStore) incrementCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.incrementCalls
}

// UserStore.

func (f *fakeStore) CreateUser(_ context.Context, u UserRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.users {
		other := f.users[i]
		if other.EmailNormalized == u.EmailNormalized {
			return ErrConflict
		}
	}
	f.users[u.ID] = u
	return nil
}

func (f *fakeStore) GetUser(_ context.Context, user UserID) (UserRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errGetUser != nil {
		return UserRecord{}, f.errGetUser
	}
	u, ok := f.users[user]
	if !ok {
		return UserRecord{}, ErrNotFound
	}
	return u, nil
}

func (f *fakeStore) GetUserByEmail(ctx context.Context, email string) (UserRecord, error) {
	u, ok := f.userByEmail(email)
	if !ok {
		return UserRecord{}, ErrNotFound
	}
	return u, nil
}

func (f *fakeStore) GetUserCredentialsByEmail(_ context.Context, email string) (UserCredentials, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errGetCredentials != nil {
		return UserCredentials{}, f.errGetCredentials
	}
	for uid := range f.users {
		u := f.users[uid]
		if u.EmailNormalized == email {
			return UserCredentials{User: u, PasswordHash: f.hashes[uid]}, nil
		}
	}
	return UserCredentials{}, ErrNotFound
}

func (f *fakeStore) SetPasswordHash(_ context.Context, user UserID, hash string, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errSetPasswordHash != nil {
		return f.errSetPasswordHash
	}
	if _, ok := f.users[user]; !ok {
		return ErrNotFound
	}
	f.hashes[user] = hash
	return nil
}

func (f *fakeStore) CreateUserWithEntry(_ context.Context, u UserRecord, hash string, entry audit.Entry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errCreateUserEntry != nil {
		return f.errCreateUserEntry
	}
	for i := range f.users {
		other := f.users[i]
		if other.EmailNormalized == u.EmailNormalized {
			return ErrConflict
		}
	}
	f.users[u.ID] = u
	f.hashes[u.ID] = hash
	f.principals[u.PrincipalID] = PrincipalRecord{ID: u.PrincipalID, Kind: PrincipalUser}
	f.entries = append(f.entries, entry)
	return nil
}

func (f *fakeStore) ChangePassword(_ context.Context, user UserID, hash string, now time.Time, entry audit.Entry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errChangePassword != nil {
		return f.errChangePassword
	}
	if _, ok := f.users[user]; !ok {
		return ErrNotFound
	}
	f.hashes[user] = hash
	for sid := range f.sessions {
		s := f.sessions[sid]
		if s.UserID == user && s.RevokedAt.IsZero() {
			s.RevokedAt = now
			f.sessions[sid] = s
		}
	}
	f.entries = append(f.entries, entry)
	return nil
}

// PrincipalStore.

func (f *fakeStore) CreatePrincipal(_ context.Context, p PrincipalRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.raceSystemPrincipal {
		// Another writer won the race: the system principal exists, this call fails.
		f.principals[p.ID] = p
		return ErrConflict
	}
	if f.errCreatePrincipal != nil {
		return f.errCreatePrincipal
	}
	if p.Kind == PrincipalSystem {
		for i := range f.principals {
			other := f.principals[i]
			if other.Kind == PrincipalSystem {
				return ErrConflict
			}
		}
	}
	f.principals[p.ID] = p
	return nil
}

func (f *fakeStore) GetSystemPrincipal(context.Context) (PrincipalRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errGetSystemPrincipal != nil {
		return PrincipalRecord{}, f.errGetSystemPrincipal
	}
	for i := range f.principals {
		p := f.principals[i]
		if p.Kind == PrincipalSystem {
			return p, nil
		}
	}
	return PrincipalRecord{}, ErrNotFound
}

func (f *fakeStore) DeletePrincipal(_ context.Context, principal PrincipalID, _ int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.principals[principal]; !ok {
		return ErrNotFound
	}
	delete(f.principals, principal)
	return nil
}

// SessionStore.

func (f *fakeStore) CreateSession(_ context.Context, s SessionRecord, entry audit.Entry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errCreateSession != nil {
		return f.errCreateSession
	}
	for i := range f.sessions {
		other := f.sessions[i]
		if bytes.Equal(other.TokenHash, s.TokenHash) {
			return ErrConflict
		}
	}
	f.sessions[s.ID] = s
	f.entries = append(f.entries, entry)
	return nil
}

func (f *fakeStore) GetSessionByTokenHash(_ context.Context, hash []byte) (SessionView, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errGetTokenHash != nil {
		return SessionView{}, f.errGetTokenHash
	}
	for i := range f.sessions {
		s := f.sessions[i]
		if !bytes.Equal(s.TokenHash, hash) {
			continue
		}
		u, ok := f.users[s.UserID]
		if !ok {
			return SessionView{}, ErrNotFound
		}
		return SessionView{Session: s, PrincipalID: u.PrincipalID, UserStatus: u.Status, Email: u.EmailNormalized, DisplayName: u.DisplayName}, nil
	}
	return SessionView{}, ErrNotFound
}

func (f *fakeStore) GetSession(_ context.Context, user UserID, session SessionID) (SessionRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errGetSession != nil {
		return SessionRecord{}, f.errGetSession
	}
	s, ok := f.sessions[session]
	if !ok || s.UserID != user {
		return SessionRecord{}, ErrNotFound
	}
	return s, nil
}

func (f *fakeStore) UpdateSessionActivity(_ context.Context, session SessionID, lastSeen, idleExpiresAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errUpdateActivity != nil {
		return f.errUpdateActivity
	}
	s, ok := f.sessions[session]
	if !ok || !s.RevokedAt.IsZero() {
		return ErrNotFound
	}
	s.LastSeenAt = lastSeen
	s.IdleExpiresAt = idleExpiresAt
	f.sessions[session] = s
	return nil
}

func (f *fakeStore) MarkSessionRevoked(_ context.Context, user UserID, session SessionID, at time.Time, entry audit.Entry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.sessions[session]
	if !ok || s.UserID != user || !s.RevokedAt.IsZero() {
		return ErrNotFound
	}
	s.RevokedAt = at
	f.sessions[session] = s
	f.entries = append(f.entries, entry)
	return nil
}

// LoginThrottleStore.

func (f *fakeStore) GetLoginThrottle(_ context.Context, key []byte) (LoginThrottle, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errGetThrottle != nil {
		return LoginThrottle{}, f.errGetThrottle
	}
	t, ok := f.throttle[string(key)]
	if !ok {
		return LoginThrottle{}, ErrNotFound
	}
	return t, nil
}

func (f *fakeStore) PutLoginThrottle(_ context.Context, key []byte, t LoginThrottle) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.putThrottleCalls++
	f.throttle[string(key)] = t
	return nil
}

func (f *fakeStore) ClearLoginThrottle(_ context.Context, key []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errClearThrottle != nil {
		return f.errClearThrottle
	}
	delete(f.throttle, string(key))
	return nil
}

// IncrementLoginThrottle counts one failure under the lock, so the count it returns
// includes every concurrent failure before it.
func (f *fakeStore) IncrementLoginThrottle(_ context.Context, key []byte, now time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.incrementCalls++
	if f.errIncrement != nil {
		return 0, f.errIncrement
	}
	t := f.throttle[string(key)]
	t.Failures++
	t.UpdatedAt = now
	f.throttle[string(key)] = t
	return t.Failures, nil
}

func (f *fakeStore) LockLoginThrottle(_ context.Context, key []byte, until, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errLock != nil {
		return f.errLock
	}
	t, ok := f.throttle[string(key)]
	if !ok {
		return ErrNotFound
	}
	t.LockedUntil = until
	t.UpdatedAt = now
	f.throttle[string(key)] = t
	return nil
}

// AuditLog.

func (f *fakeStore) AppendInstanceEntry(_ context.Context, entry audit.Entry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errAppendAudit != nil {
		return f.errAppendAudit
	}
	f.entries = append(f.entries, entry)
	return nil
}

// fakeHasher stores a password as "hash:<password>" so a test can tell which hash a
// check ran against. It records every encoded hash Verify receives.
type fakeHasher struct {
	mu       sync.Mutex
	verified []string
	// rehash makes NeedsRehash report true for every hash.
	rehash    bool
	errHash   error
	errVerify error
}

func (h *fakeHasher) Hash(password string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.errHash != nil {
		return "", h.errHash
	}
	return "hash:" + password, nil
}

func (h *fakeHasher) Verify(encoded, password string) (bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.verified = append(h.verified, encoded)
	if h.errVerify != nil {
		return false, h.errVerify
	}
	return encoded == "hash:"+password, nil
}

func (h *fakeHasher) NeedsRehash(string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.rehash
}

func (h *fakeHasher) verifiedAgainst() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.verified...)
}

// fixture wires the account and session services over one fake store and one fake
// clock. The clock starts at a fixed instant so every expiry in the tests is exact.
type fixture struct {
	store    *fakeStore
	hasher   *fakeHasher
	clock    *clock.Fake
	ids      *id.Sequence
	identify Identifier
	sessions *Sessions
	accounts *Accounts
}

var fixtureStart = time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)

const fixtureOrigin = "https://orcon.test"

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{
		store:  newFakeStore(),
		hasher: &fakeHasher{},
		clock:  clock.NewFake(fixtureStart),
		ids:    id.NewSequence(),
	}
	f.store.ids = f.ids
	identify, err := HMACIdentifier(bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatalf("identifier: %v", err)
	}
	f.identify = identify
	f.sessions = f.newSessions(t, SessionDeps{})
	f.accounts = NewAccounts(f.store, f.hasher, f.clock, f.ids)
	return f
}

// newSessions builds a session service over the fixture's fakes. Fields set in deps
// replace the fixture defaults, so one test can inject a failing port.
func (f *fixture) newSessions(t *testing.T, deps SessionDeps) *Sessions {
	t.Helper()
	if deps.Users == nil {
		deps.Users = f.store
	}
	if deps.Sessions == nil {
		deps.Sessions = f.store
	}
	if deps.Throttle == nil {
		deps.Throttle = f.store
	}
	if deps.Hasher == nil {
		deps.Hasher = f.hasher
	}
	if deps.Identify == nil {
		deps.Identify = f.identify
	}
	if deps.Clock == nil {
		deps.Clock = f.clock
	}
	if deps.IDs == nil {
		deps.IDs = f.ids
	}
	if deps.Audit == nil {
		deps.Audit = f.store
	}
	svc, err := NewSessions(deps, SessionConfig{PublicOrigin: fixtureOrigin})
	if err != nil {
		t.Fatalf("NewSessions: %v", err)
	}
	return svc
}

// account stores an ACTIVE account whose password is the given one.
func (f *fixture) account(email, password string) UserRecord {
	hash, _ := f.hasher.Hash(password)
	return f.store.addUser(email, hash, UserActive)
}

// signIn signs the account in and fails the test on a refusal.
func (f *fixture) signIn(t *testing.T, email, password string) SignedIn {
	t.Helper()
	signed, err := f.sessions.SignIn(t.Context(), SignIn{Email: email, Password: password})
	if err != nil {
		t.Fatalf("sign in %s: %v", email, err)
	}
	return signed
}

// errReader fails every read, to reach the token-generation error path.
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("entropy source is down") }
