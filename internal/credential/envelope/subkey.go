// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package envelope

import (
	"context"
	"crypto/hkdf"
	"crypto/sha256"
	"fmt"
)

// Subkey derives a 32-byte key from k with HKDF-SHA-256 (RFC 5869). The purpose
// names what the subkey is for, for example "orcon/audit@1 mac"; two purposes give
// unrelated keys, so a key that authenticates one thing never authenticates another.
func (k Key) Subkey(purpose string) ([]byte, error) {
	b := k.bytes()
	if b == nil {
		return nil, ErrInvalidKey
	}
	out, err := hkdf.Key(sha256.New, b[:], nil, purpose, KeySize)
	if err != nil {
		return nil, fmt.Errorf("derive subkey: %w", err)
	}
	return out, nil
}

// SubkeyFor returns the purpose-bound subkey of one key version. The key is
// unwrapped through the master key unless a fresh copy is cached (ADR-0042 section 3).
// The caller owns the returned bytes.
func (k *Keyring) SubkeyFor(ctx context.Context, id KeyID, purpose string) ([]byte, error) {
	if err := id.Validate(); err != nil {
		return nil, fmt.Errorf("subkey: %w", err)
	}
	if id.Level == LevelMaster {
		return nil, fmt.Errorf("subkey: %w: the master key has no subkeys", ErrInvalidKeyID)
	}
	key, err := k.unwrappedKey(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("subkey under %s: %w", id, err)
	}
	defer key.Destroy()
	return key.Subkey(purpose)
}

// ActiveSubkey returns the active version of scope and its subkey for purpose.
func (k *Keyring) ActiveSubkey(ctx context.Context, scope Scope, purpose string) (KeyID, []byte, error) {
	id, err := k.activeID(ctx, scope)
	if err != nil {
		return KeyID{}, nil, err
	}
	sub, err := k.SubkeyFor(ctx, id, purpose)
	if err != nil {
		return KeyID{}, nil, err
	}
	return id, sub, nil
}
