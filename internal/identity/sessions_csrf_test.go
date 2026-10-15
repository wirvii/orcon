// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package identity_test

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/wirvii/orcon/internal/identity"
	"github.com/wirvii/orcon/internal/platform/clock"
	"github.com/wirvii/orcon/internal/platform/id"
	"github.com/wirvii/orcon/internal/platform/password"
)

// memStore is the in-memory store the session service runs against in this test. Only the
// methods the service calls are implemented; the embedded interfaces stay nil for the rest.
type memStore struct {
	identity.UserStore
	identity.SessionStore
	identity.LoginThrottleStore

	mu       sync.Mutex
	creds    map[string]identity.UserCredentials
	sessions map[string]identity.SessionRecord
	throttle map[string]identity.LoginThrottle
}

func newMemStore() *memStore {
	return &memStore{
		creds:    map[string]identity.UserCredentials{},
		sessions: map[string]identity.SessionRecord{},
		throttle: map[string]identity.LoginThrottle{},
	}
}

func (m *memStore) GetUserCredentialsByEmail(_ context.Context, email string) (identity.UserCredentials, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.creds[email]
	if !ok {
		return identity.UserCredentials{}, identity.ErrNotFound
	}
	return c, nil
}

func (m *memStore) CreateSession(_ context.Context, rec identity.SessionRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[string(rec.TokenHash)] = rec
	return nil
}

func (m *memStore) GetSessionByTokenHash(_ context.Context, hash []byte) (identity.SessionView, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	rec, ok := m.sessions[string(hash)]
	if !ok {
		return identity.SessionView{}, identity.ErrNotFound
	}
	return identity.SessionView{Session: rec, UserStatus: identity.UserActive}, nil
}

func (m *memStore) UpdateSessionActivity(context.Context, identity.SessionID, time.Time, time.Time) error {
	return nil
}

func (m *memStore) GetLoginThrottle(_ context.Context, key []byte) (identity.LoginThrottle, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.throttle[string(key)]
	if !ok {
		return identity.LoginThrottle{}, identity.ErrNotFound
	}
	return t, nil
}

func (m *memStore) IncrementLoginThrottle(_ context.Context, key []byte, now time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.throttle[string(key)]
	t.Failures++
	t.UpdatedAt = now
	m.throttle[string(key)] = t
	return t.Failures, nil
}

func (m *memStore) LockLoginThrottle(_ context.Context, key []byte, until, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := m.throttle[string(key)]
	t.LockedUntil = until
	t.UpdatedAt = now
	m.throttle[string(key)] = t
	return nil
}

func (m *memStore) ClearLoginThrottle(_ context.Context, key []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.throttle, string(key))
	return nil
}

const csrfOrigin = "https://orcon.test"

// TestUnsafeRequestWithoutCsrfTokenRejected (SPEC-016 R-SH-SES-03): a state-changing request on a cookie
// session needs the Origin of the installation and a CSRF token that matches the session; a safe request
// needs neither.
func TestUnsafeRequestWithoutCsrfTokenRejected(t *testing.T) {
	ctx := t.Context()
	store := newMemStore()
	hasher := password.Argon2id{Memory: 64, Time: 1, Threads: 1, KeyLength: 32, SaltLength: 16}
	hash, err := hasher.Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	store.creds["ada@example.test"] = identity.UserCredentials{
		User:         identity.UserRecord{ID: identity.UserID(id.NewSequence().New()), EmailNormalized: "ada@example.test", Status: identity.UserActive},
		PasswordHash: hash,
	}
	identify, err := identity.HMACIdentifier(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatalf("identifier: %v", err)
	}
	svc, err := identity.NewSessions(identity.SessionDeps{
		Users: store, Sessions: store, Throttle: store, Hasher: hasher, Identify: identify,
		Clock: clock.NewFake(time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)), IDs: id.NewSequence(),
	}, identity.SessionConfig{PublicOrigin: csrfOrigin})
	if err != nil {
		t.Fatalf("sessions: %v", err)
	}
	signed, err := svc.SignIn(ctx, identity.SignIn{Email: "ada@example.test", Password: "correct horse battery staple"})
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}

	good := identity.Credentials{Session: signed.Token, CSRFToken: signed.CSRFToken, CSRFCookie: signed.CSRFToken, Origin: csrfOrigin, UnsafeMethod: true}
	if _, err := svc.Authenticate(ctx, good); err != nil {
		t.Fatalf("a complete unsafe request: %v", err)
	}
	bad := map[string]identity.Credentials{
		"no header":        {Session: signed.Token, CSRFCookie: signed.CSRFToken, Origin: csrfOrigin, UnsafeMethod: true},
		"header not equal": {Session: signed.Token, CSRFToken: "x", CSRFCookie: signed.CSRFToken, Origin: csrfOrigin, UnsafeMethod: true},
		"other origin":     {Session: signed.Token, CSRFToken: signed.CSRFToken, CSRFCookie: signed.CSRFToken, Origin: "https://evil.test", UnsafeMethod: true},
		"no origin":        {Session: signed.Token, CSRFToken: signed.CSRFToken, CSRFCookie: signed.CSRFToken, UnsafeMethod: true},
	}
	for name, creds := range bad {
		if _, err := svc.Authenticate(ctx, creds); !errors.Is(err, identity.ErrCSRFCheckFailed) {
			t.Errorf("%s: %v, want ErrCSRFCheckFailed", name, err)
		}
	}
	if _, err := svc.Authenticate(ctx, identity.Credentials{Session: signed.Token}); err != nil {
		t.Errorf("a safe request needs no CSRF token: %v", err)
	}
}
