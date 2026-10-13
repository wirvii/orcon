// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package envelope

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
)

// VerifyError is the failure of [Verify]. It names what an operator needs to
// fix the start: which key failed, which provider and key reference were
// configured, and the fingerprint the stored keys expect (ADR-0042 section 3).
// It matches [ErrVerification] with [errors.Is], and its cause with
// [errors.Is] and [errors.As].
type VerifyError struct {
	// Provider is the configured master key provider name.
	Provider string
	// Reference is the configured master key reference.
	Reference string
	// ExpectedFingerprint is the fingerprint recorded in the master key
	// canary; empty when no canary could be read.
	ExpectedFingerprint string
	// Key is the key whose canary failed; the master key has LevelMaster.
	Key KeyID
	// Cause is the underlying failure.
	Cause error
}

// Error describes the failure without any key material.
func (e *VerifyError) Error() string {
	return fmt.Sprintf("startup key verification failed for %s: master key provider %q, reference %q, expected fingerprint %q: %v",
		e.Key, e.Provider, e.Reference, e.ExpectedFingerprint, e.Cause)
}

// Unwrap returns the cause and [ErrVerification].
func (e *VerifyError) Unwrap() []error { return []error{ErrVerification, e.Cause} }

// Report is the outcome of a successful [Verify].
type Report struct {
	// Fresh is true when the store holds no keys at all: this is a first
	// start, and the installation ceremony must run before anything is
	// encrypted.
	Fresh bool
	// ActiveKeys is how many organization and instance keys were verified.
	ActiveKeys int
}

// Verify is the startup check (SPEC-004 R-KEY-05). It decrypts the master
// key canary with the configured master key provider, then the canary of every
// active organization and instance key after unwrapping the key itself. It
// takes the read-only [KeyReader], so it cannot write: the caller runs it
// before migrations and before any other write, and must exit on error.
//
// The startup order that SE3b wires in cmd/orcon is: resolve the configuration
// and build the [MasterKey] without writing; open the store without writing
// and read its canaries through a [KeyReader]; call Verify and exit on error,
// before any migration; if [Report.Fresh], continue: nothing is encrypted yet
// and the instance waits for the installation ceremony; then migrate.
//
// An empty store is a first start ([Report.Fresh]); keys without a master key
// canary are a corrupt store and fail.
func Verify(ctx context.Context, mk MasterKey, store KeyReader) (Report, error) {
	fail := func(expected string, key KeyID, cause error) error {
		return &VerifyError{
			Provider: mk.Provider(), Reference: mk.Reference(),
			ExpectedFingerprint: expected, Key: key, Cause: cause,
		}
	}
	master := KeyID{Level: LevelMaster, Version: 1}

	canary, err := store.MasterCanary(ctx)
	if errors.Is(err, ErrKeyNotFound) {
		keys, kerr := store.ActiveKeys(ctx)
		if kerr != nil {
			return Report{}, fail("", master, fmt.Errorf("list active keys: %w", kerr))
		}
		if len(keys) == 0 {
			return Report{Fresh: true}, nil
		}
		return Report{}, fail("", master, fmt.Errorf("the store has %d active keys but no master key canary: %w", len(keys), ErrKeyNotFound))
	}
	if err != nil {
		return Report{}, fail("", master, fmt.Errorf("read master key canary: %w", err))
	}
	master.Version = canary.Version
	if err := master.Validate(); err != nil {
		return Report{}, fail("", master, err)
	}
	expected, wrapped, err := decodeMasterProbe(canary.Probe)
	if err != nil {
		return Report{}, fail("", master, err)
	}

	actual, err := mk.Fingerprint(ctx)
	if err != nil {
		return Report{}, fail(expected, master, fmt.Errorf("read master key fingerprint: %w", err))
	}
	if subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) != 1 {
		// The stored fingerprint is part of the canary's authenticated data.
		// If the configured key opens the canary when the data names the
		// configured key's own fingerprint, the key is the right one and the
		// stored fingerprint was edited; if not, it is a different key.
		if _, uerr := mk.Unwrap(ctx, wrapped, masterCanaryAAD(master, actual)); uerr == nil {
			return Report{}, fail(expected, master, fmt.Errorf("%w: the stored fingerprint does not match the configured key, which opens the canary", ErrCanaryTampered))
		}
		return Report{}, fail(expected, master, fmt.Errorf("%w: configured key has fingerprint %q", ErrWrongMasterKey, actual))
	}
	plain, err := mk.Unwrap(ctx, wrapped, masterCanaryAAD(master, expected))
	if err != nil {
		return Report{}, fail(expected, master, fmt.Errorf("%w (or wrong key): %w", ErrCanaryTampered, err))
	}
	if subtle.ConstantTimeCompare(plain, []byte(canaryPlaintext)) != 1 {
		return Report{}, fail(expected, master, fmt.Errorf("%w: canary content differs", ErrWrongMasterKey))
	}

	keys, err := store.ActiveKeys(ctx)
	if err != nil {
		return Report{}, fail(expected, master, fmt.Errorf("list active keys: %w", err))
	}
	for i := range keys {
		stored := &keys[i]
		if err := ctx.Err(); err != nil {
			return Report{}, fail(expected, stored.ID, err)
		}
		if err := verifyKey(ctx, mk, *stored); err != nil {
			return Report{}, fail(expected, stored.ID, err)
		}
	}
	return Report{ActiveKeys: len(keys)}, nil
}

// verifyKey unwraps one stored key and decrypts its canary.
func verifyKey(ctx context.Context, mk MasterKey, stored StoredKey) error {
	if err := stored.ID.Validate(); err != nil {
		return err
	}
	if stored.ID.Level == LevelMaster {
		return fmt.Errorf("%w: a stored key at the master level", ErrInvalidKeyID)
	}
	raw, err := mk.Unwrap(ctx, stored.Wrapped, keyAAD(purposeKeyWrap, stored.ID))
	if err != nil {
		return fmt.Errorf("unwrap key (wrapped under master key reference %q): %w", stored.MasterKeyReference, err)
	}
	defer clear(raw)
	key, err := NewKey(raw)
	if err != nil {
		return err
	}
	defer key.Destroy()
	plain, err := openBlob(key, keyAAD(purposeCanary, stored.ID), stored.Canary)
	if err != nil {
		return fmt.Errorf("decrypt canary: %w", err)
	}
	if subtle.ConstantTimeCompare(plain, []byte(canaryPlaintext)) != 1 {
		return fmt.Errorf("%w: canary content differs", ErrDecrypt)
	}
	return nil
}

// probeFormatV1 starts the master key probe: version, length of the
// fingerprint, the fingerprint, then the canary wrapped by the master key. The
// fingerprint is stored in the clear so a wrong-key failure can name the key
// the data expects; it is a truncated hash of a 256-bit random key and
// reveals nothing about it.
const probeFormatV1 = 1

// masterCanaryAAD is the additional authenticated data of the master canary.
// It includes the fingerprint stored beside the ciphertext, so changing that
// fingerprint in the database makes the canary fail to decrypt instead of
// leaving a misleading expected fingerprint behind.
func masterCanaryAAD(id KeyID, fingerprint string) []byte {
	return append(keyAAD(purposeCanary, id), "|fp="+fingerprint...)
}

func encodeMasterProbe(fingerprint string, wrapped []byte) ([]byte, error) {
	if fingerprint == "" || len(fingerprint) > 255 {
		return nil, fmt.Errorf("%w: fingerprint of %d bytes", ErrMalformed, len(fingerprint))
	}
	out := make([]byte, 0, 2+len(fingerprint)+len(wrapped))
	out = append(out, probeFormatV1, byte(len(fingerprint))) //nolint:gosec // G115: the length is checked above to be at most 255
	out = append(out, fingerprint...)
	return append(out, wrapped...), nil
}

func decodeMasterProbe(probe []byte) (fingerprint string, wrapped []byte, err error) {
	if len(probe) < 2 {
		return "", nil, fmt.Errorf("%w: master key canary is truncated", ErrMalformed)
	}
	if probe[0] != probeFormatV1 {
		return "", nil, fmt.Errorf("%w: master key canary format version %d is not supported (this build reads version %d)",
			ErrMalformed, probe[0], probeFormatV1)
	}
	n := int(probe[1])
	if n == 0 || len(probe) < 2+n {
		return "", nil, fmt.Errorf("%w: master key canary is truncated", ErrMalformed)
	}
	return string(probe[2 : 2+n]), probe[2+n:], nil
}
