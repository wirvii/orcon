// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package envelope

import "errors"

// Sentinel errors of the package. Callers inspect them with [errors.Is]; no
// error message ever contains key material.
var (
	// ErrDecrypt means an authenticated decryption failed: the key, the
	// additional authenticated data or the ciphertext is not the one that
	// produced the data. The cause is deliberately not distinguished.
	ErrDecrypt = errors.New("envelope: decryption failed")

	// ErrMalformed means a serialized envelope or probe is not well formed.
	ErrMalformed = errors.New("envelope: malformed data")

	// ErrInvalidAAD means an additional authenticated data field is empty
	// where it is required or contains a character the encoding reserves.
	ErrInvalidAAD = errors.New("envelope: invalid additional authenticated data")

	// ErrInvalidKeyID means a key identifier breaks the shape of its level.
	ErrInvalidKeyID = errors.New("envelope: invalid key identifier")

	// ErrInvalidKey means key material is not exactly 256 bits.
	ErrInvalidKey = errors.New("envelope: key material must be 32 bytes")

	// ErrKeyNotFound means the key store has no such key or canary.
	ErrKeyNotFound = errors.New("envelope: key not found")

	// ErrKeyExists means the key or canary to create already exists.
	ErrKeyExists = errors.New("envelope: key already exists")

	// ErrKeyUnavailable means the key exists but its status forbids using it:
	// it is retired or destroyed.
	ErrKeyUnavailable = errors.New("envelope: key unavailable")

	// ErrKeyFilePermissions means the master key file is accessible by its
	// group or by other users.
	ErrKeyFilePermissions = errors.New("envelope: master key file permissions too broad")

	// ErrMasterKeyInvalid means the configured master key material cannot be
	// used: missing, unreadable, not a regular file or not 256 bits.
	ErrMasterKeyInvalid = errors.New("envelope: master key invalid")

	// ErrWrongMasterKey means the configured master key is not the one the
	// stored keys were wrapped with.
	ErrWrongMasterKey = errors.New("envelope: wrong master key")

	// ErrCanaryTampered means the configured master key is, or may be, the
	// right one, but the stored canary does not authenticate: it is corrupt or
	// was edited (for instance its stored fingerprint).
	ErrCanaryTampered = errors.New("envelope: canary corrupt or tampered")

	// ErrVerification is matched by every [VerifyError].
	ErrVerification = errors.New("envelope: startup key verification failed")

	// ErrBackupAck means the typed or configured backup acknowledgement does
	// not match the recovery kit fingerprint.
	ErrBackupAck = errors.New("envelope: backup acknowledgement does not match the fingerprint")
)
