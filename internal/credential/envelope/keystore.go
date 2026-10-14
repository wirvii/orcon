// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package envelope

import (
	"context"
	"time"
)

// KeyStatus is the lifecycle state of a stored key. The values are the ones the
// key tables store (ADR-0042 section 1).
type KeyStatus string

// The states of a stored key.
const (
	// KeyActive is the version new data keys are wrapped with.
	KeyActive KeyStatus = "ACTIVE"
	// KeyRetiring still unwraps data keys while a rotation re-wraps them.
	KeyRetiring KeyStatus = "RETIRING"
	// KeyRetired is no longer referenced by any data key.
	KeyRetired KeyStatus = "RETIRED"
	// KeyDestroyed has been overwritten; its data is unreadable.
	KeyDestroyed KeyStatus = "DESTROYED"
)

func (s KeyStatus) usable() bool { return s == KeyActive || s == KeyRetiring }

// StoredKey is an organization or instance key as the key store keeps it:
// wrapped by the master key, never in clear.
type StoredKey struct {
	ID KeyID
	// Wrapped is the key wrapped by the master key; opaque to the store.
	Wrapped []byte
	// MasterKeyReference is the [MasterKey.Reference] that wrapped it.
	MasterKeyReference string
	Status             KeyStatus
	CreatedAt          time.Time
	// Canary is the probe encrypted under this key version (the
	// org_key_canary or instance_key_canary row); opaque to the store.
	Canary []byte
}

// MasterCanary is the probe of the master key (the instance_key_canary row of
// level MASTER); opaque to the store. Version starts at 1.
type MasterCanary struct {
	Version int
	Probe   []byte
}

// KeyReader is the read-only part of the key store that startup verification
// needs. [Verify] takes it, and not [KeyStore], so verification cannot write.
type KeyReader interface {
	// MasterCanary returns the current master key canary, or
	// [ErrKeyNotFound] when none was ever written.
	MasterCanary(ctx context.Context) (MasterCanary, error)
	// ActiveKeys returns every key whose status is ACTIVE, of every
	// organization and of the instance. The adapter reads them through the
	// engine plane (ADR-0042 section 3, ADR-0047 section 2).
	ActiveKeys(ctx context.Context) ([]StoredKey, error)
}

// KeyStore is the port to the stored keys and canaries. The tables
// (instance_key, instance_key_canary, org_key, org_key_canary) and the
// adapters for both databases arrive with SE3b of SPEC-004; until then the
// tests use a fake.
//
// Adapters map their errors to this package's sentinels: [ErrKeyNotFound] when
// a row does not exist and [ErrKeyExists] on a uniqueness violation.
type KeyStore interface {
	KeyReader
	// Key returns one version of one key, whatever its status.
	Key(ctx context.Context, id KeyID) (StoredKey, error)
	// ActiveKey returns the active key of a scope, or [ErrKeyNotFound].
	ActiveKey(ctx context.Context, scope Scope) (StoredKey, error)
	// PutKey stores a new key together with its canary in one atomic write,
	// so no key exists without a canary.
	PutKey(ctx context.Context, key StoredKey) error
	// PutMasterCanary stores the first master key canary.
	PutMasterCanary(ctx context.Context, canary MasterCanary) error
}
