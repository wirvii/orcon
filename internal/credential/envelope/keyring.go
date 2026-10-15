// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package envelope

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/wirvii/orcon/internal/platform/clock"
)

// DefaultCacheTTL is how long an unwrapped organization or instance key stays
// in memory before it is unwrapped again. With a key management service this
// is what makes disabling the master key take effect within an hour
// (ADR-0042 section 2).
const DefaultCacheTTL = time.Hour

// DataKey is a fresh data key with the form to store beside the ciphertext it
// encrypts.
type DataKey struct {
	// ID is the organization or instance key version that wrapped the data
	// key; pass it to [Seal] and keep it with the ciphertext.
	ID KeyID
	// Key encrypts the value. It is never stored.
	Key Key
	// Wrapped is the data key wrapped by ID's key; it is what is stored.
	Wrapped []byte
}

// Option configures a [Keyring].
type Option func(*Keyring)

// WithRandom sets the random source for keys and nonces; the default is
// [crypto/rand.Reader]. Tests use it for deterministic output.
func WithRandom(r io.Reader) Option { return func(k *Keyring) { k.random = r } }

// WithClock sets the clock for creation times and cache expiry.
func WithClock(c clock.Clock) Option { return func(k *Keyring) { k.clock = c } }

// WithCacheTTL sets how long unwrapped keys are kept; the default is
// [DefaultCacheTTL]. A value of zero or less disables the cache, so every use
// unwraps again.
func WithCacheTTL(d time.Duration) Option { return func(k *Keyring) { k.ttl = d } }

// Keyring creates and unwraps the keys of the hierarchy: the master key
// wraps organization and instance keys, and those wrap data keys. It is safe
// for concurrent use.
type Keyring struct {
	mk     MasterKey
	store  KeyStore
	random io.Reader
	clock  clock.Clock
	ttl    time.Duration

	// mu guards the two caches. It is never held across a call to the store
	// or to the master key provider, which may be remote.
	mu     sync.Mutex
	keys   map[KeyID]cachedKey
	active map[Scope]cachedActive
}

type cachedKey struct {
	key     Key
	expires time.Time
}

type cachedActive struct {
	id      KeyID
	expires time.Time
}

// NewKeyring returns a keyring over the master key provider and the store.
func NewKeyring(mk MasterKey, store KeyStore, opts ...Option) *Keyring {
	k := &Keyring{
		mk:     mk,
		store:  store,
		random: rand.Reader,
		clock:  clock.System{},
		ttl:    DefaultCacheTTL,
		keys:   make(map[KeyID]cachedKey),
		active: make(map[Scope]cachedActive),
	}
	for _, opt := range opts {
		opt(k)
	}
	return k
}

// InitMasterCanary writes the master key canary, version 1. It returns
// [ErrKeyExists] if a canary is already stored: changing it is a master key
// rotation, not an initialization.
func (k *Keyring) InitMasterCanary(ctx context.Context) error {
	_, err := k.store.MasterCanary(ctx)
	switch {
	case err == nil:
		return fmt.Errorf("init master key canary: %w", ErrKeyExists)
	case !errors.Is(err, ErrKeyNotFound):
		return fmt.Errorf("init master key canary: %w", err)
	}
	fingerprint, err := k.mk.Fingerprint(ctx)
	if err != nil {
		return fmt.Errorf("init master key canary: %w", err)
	}
	id := KeyID{Level: LevelMaster, Version: 1}
	wrapped, err := k.mk.Wrap(ctx, []byte(canaryPlaintext), masterCanaryAAD(id, fingerprint))
	if err != nil {
		return fmt.Errorf("init master key canary: %w", err)
	}
	probe, err := encodeMasterProbe(fingerprint, wrapped)
	if err != nil {
		return fmt.Errorf("init master key canary: %w", err)
	}
	if err := k.store.PutMasterCanary(ctx, MasterCanary{Version: id.Version, Probe: probe}); err != nil {
		return fmt.Errorf("init master key canary: %w", err)
	}
	return nil
}

// CreateKey creates version 1 of the organization or instance key of scope:
// 256 random bits, wrapped by the master key, stored together with its canary.
// It returns [ErrKeyExists] if the scope already has an active key; a new
// version of an existing key is a rotation (SE3b).
func (k *Keyring) CreateKey(ctx context.Context, scope Scope) (KeyID, error) {
	if err := scope.Validate(); err != nil {
		return KeyID{}, fmt.Errorf("create key: %w", err)
	}
	_, err := k.store.ActiveKey(ctx, scope)
	switch {
	case err == nil:
		return KeyID{}, fmt.Errorf("create key for %s: %w", scope.Level, ErrKeyExists)
	case !errors.Is(err, ErrKeyNotFound):
		return KeyID{}, fmt.Errorf("create key for %s: %w", scope.Level, err)
	}
	stored, key, err := k.newKey(ctx, scope)
	if err != nil {
		return KeyID{}, err
	}
	if err := k.store.PutKey(ctx, stored); err != nil {
		key.Destroy()
		return KeyID{}, fmt.Errorf("create key %s: %w", stored.ID, err)
	}
	k.remember(stored.ID, key)
	key.Destroy()
	return stored.ID, nil
}

// NewKey prepares version 1 of the key of scope without storing it, for a caller
// that writes it in the same transaction as the row that needs it (an
// organization is created together with its key, so no organization exists
// without one). The caller stores the result; until it does, the key exists
// nowhere but in the returned value.
func (k *Keyring) NewKey(ctx context.Context, scope Scope) (StoredKey, error) {
	if err := scope.Validate(); err != nil {
		return StoredKey{}, fmt.Errorf("new key: %w", err)
	}
	stored, key, err := k.newKey(ctx, scope)
	if err != nil {
		return StoredKey{}, err
	}
	key.Destroy()
	return stored, nil
}

// newKey generates version 1 of the key of scope and returns its stored form and
// the key in clear; the caller destroys the key.
func (k *Keyring) newKey(ctx context.Context, scope Scope) (StoredKey, Key, error) {
	id := KeyID{Level: scope.Level, OrgID: scope.OrgID, Version: 1}
	key, err := GenerateKey(k.random)
	if err != nil {
		return StoredKey{}, Key{}, fmt.Errorf("create key %s: %w", id, err)
	}
	wrapped, err := k.mk.Wrap(ctx, key.bytes()[:], keyAAD(purposeKeyWrap, id))
	if err != nil {
		key.Destroy()
		return StoredKey{}, Key{}, fmt.Errorf("create key %s: %w", id, err)
	}
	canary, err := sealBlob(k.random, key, keyAAD(purposeCanary, id), []byte(canaryPlaintext))
	if err != nil {
		key.Destroy()
		return StoredKey{}, Key{}, fmt.Errorf("create key %s: %w", id, err)
	}
	return StoredKey{
		ID:                 id,
		Wrapped:            wrapped,
		MasterKeyReference: k.mk.Reference(),
		Status:             KeyActive,
		CreatedAt:          clock.Micros(k.clock.Now()),
		Canary:             canary,
	}, key, nil
}

// GenerateDataKey creates a data key wrapped by the active key of scope.
func (k *Keyring) GenerateDataKey(ctx context.Context, scope Scope) (DataKey, error) {
	if err := scope.Validate(); err != nil {
		return DataKey{}, fmt.Errorf("generate data key: %w", err)
	}
	id, err := k.activeID(ctx, scope)
	if err != nil {
		return DataKey{}, fmt.Errorf("generate data key: %w", err)
	}
	wrapping, err := k.unwrappedKey(ctx, id)
	if err != nil {
		return DataKey{}, fmt.Errorf("generate data key: %w", err)
	}
	defer wrapping.Destroy()
	dek, err := GenerateKey(k.random)
	if err != nil {
		return DataKey{}, fmt.Errorf("generate data key: %w", err)
	}
	wrapped, err := sealBlob(k.random, wrapping, keyAAD(purposeDEKWrap, id), dek.bytes()[:])
	if err != nil {
		dek.Destroy()
		return DataKey{}, fmt.Errorf("generate data key under %s: %w", id, err)
	}
	return DataKey{ID: id, Key: dek, Wrapped: wrapped}, nil
}

// UnwrapDataKey recovers a data key from the form stored beside its
// ciphertext, using the key version recorded with it. A data key wrapped under
// another organization's key does not unwrap ([ErrDecrypt]).
func (k *Keyring) UnwrapDataKey(ctx context.Context, id KeyID, wrapped []byte) (Key, error) {
	if err := id.Validate(); err != nil {
		return Key{}, fmt.Errorf("unwrap data key: %w", err)
	}
	if id.Level == LevelMaster {
		return Key{}, fmt.Errorf("unwrap data key: %w: the master key wraps no data key", ErrInvalidKeyID)
	}
	wrapping, err := k.unwrappedKey(ctx, id)
	if err != nil {
		return Key{}, fmt.Errorf("unwrap data key under %s: %w", id, err)
	}
	defer wrapping.Destroy()
	raw, err := openBlob(wrapping, keyAAD(purposeDEKWrap, id), wrapped)
	if err != nil {
		return Key{}, fmt.Errorf("unwrap data key under %s: %w", id, err)
	}
	defer clear(raw)
	dek, err := NewKey(raw)
	if err != nil {
		return Key{}, fmt.Errorf("unwrap data key under %s: %w", id, err)
	}
	return dek, nil
}

// activeID returns the active key version of a scope, cached for the TTL.
func (k *Keyring) activeID(ctx context.Context, scope Scope) (KeyID, error) {
	now := k.clock.Now()
	k.mu.Lock()
	c, ok := k.active[scope]
	k.mu.Unlock()
	if ok && now.Before(c.expires) {
		return c.id, nil
	}
	stored, err := k.store.ActiveKey(ctx, scope)
	if err != nil {
		return KeyID{}, fmt.Errorf("active %s key: %w", scope.Level, err)
	}
	if k.ttl > 0 {
		k.mu.Lock()
		k.active[scope] = cachedActive{id: stored.ID, expires: now.Add(k.ttl)}
		k.mu.Unlock()
	}
	return stored.ID, nil
}

// unwrappedKey returns the organization or instance key in clear, unwrapping
// it through the master key provider unless a fresh copy is cached.
func (k *Keyring) unwrappedKey(ctx context.Context, id KeyID) (Key, error) {
	now := k.clock.Now()
	// The copy is made under the lock: remember wipes the entry it replaces,
	// so reading the cached bytes after unlocking races with that wipe and can
	// copy zeros.
	k.mu.Lock()
	c, ok := k.keys[id]
	var hit Key
	if ok && now.Before(c.expires) {
		hit = c.key.clone()
	}
	k.mu.Unlock()
	if hit.get != nil {
		return hit, nil
	}
	stored, err := k.store.Key(ctx, id)
	if err != nil {
		return Key{}, fmt.Errorf("load key %s: %w", id, err)
	}
	if stored.ID != id {
		return Key{}, fmt.Errorf("load key %s: store returned %s: %w", id, stored.ID, ErrKeyNotFound)
	}
	if !stored.Status.usable() {
		return Key{}, fmt.Errorf("key %s is %s: %w", id, stored.Status, ErrKeyUnavailable)
	}
	raw, err := k.mk.Unwrap(ctx, stored.Wrapped, keyAAD(purposeKeyWrap, id))
	if err != nil {
		return Key{}, fmt.Errorf("unwrap key %s with %s master key %q: %w", id, k.mk.Provider(), k.mk.Reference(), err)
	}
	defer clear(raw)
	key, err := NewKey(raw)
	if err != nil {
		return Key{}, fmt.Errorf("unwrap key %s: %w", id, err)
	}
	k.remember(id, key)
	return key, nil
}

// remember caches a copy of an unwrapped key and wipes the copy it replaces.
func (k *Keyring) remember(id KeyID, key Key) {
	if k.ttl <= 0 {
		return
	}
	expires := k.clock.Now().Add(k.ttl)
	k.mu.Lock()
	defer k.mu.Unlock()
	if old, ok := k.keys[id]; ok {
		old.key.Destroy()
	}
	k.keys[id] = cachedKey{key: key.clone(), expires: expires}
}
