// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package envelope

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// MasterKey is the master key provider (ADR-0042 section 2). The port exposes
// wrapping, not the key, so a provider backed by a key management service never
// has to release the master key: only the organization and instance keys are
// unwrapped and held in memory.
type MasterKey interface {
	// Provider returns the provider name: "file", "env", "awskms", ...
	Provider() string
	// Reference identifies the key inside the provider: a path, a variable
	// name or a key resource name. It is stored beside every wrapped key as
	// master_key_reference and appears in startup failures.
	Reference() string
	// Fingerprint returns the expected-key fingerprint shown in the recovery
	// kit and in startup failures: SHA-256 of the key, truncated, for local
	// providers; the key reference for a key management service.
	Fingerprint(ctx context.Context) (string, error)
	// Wrap encrypts plaintext under the master key, authenticating aad.
	Wrap(ctx context.Context, plaintext, aad []byte) ([]byte, error)
	// Unwrap reverses Wrap. It fails with [ErrDecrypt] when the key or aad
	// differ.
	Unwrap(ctx context.Context, wrapped, aad []byte) ([]byte, error)
}

// fingerprintBytes is the length of the truncated SHA-256 of a master key.
const fingerprintBytes = 16

// Fingerprint returns the fingerprint of a local master key: the first 16
// bytes of SHA-256 of the 32 raw key bytes (after base64 decoding, when the
// key was stored that way), in lowercase hexadecimal, 32 characters, without
// separators. It is derived from the key but does not reveal it.
func Fingerprint(key Key) string {
	b := key.bytes()
	if b == nil {
		return ""
	}
	sum := sha256.Sum256(b[:])
	return hex.EncodeToString(sum[:fingerprintBytes])
}

// KMSFingerprint returns the fingerprint of a master key that lives in a key
// management service and so cannot be read: the first 16 bytes of SHA-256 of
// the UTF-8 string orcon-kms-v1|provider|reference, in the same 32-character
// form. The provider is the configured provider name and the reference is the
// canonical key reference (ARN, resource name, key id, or
// address/mount/keyName for vault-transit).
func KMSFingerprint(provider, reference string) string {
	sum := sha256.Sum256([]byte("orcon-kms-v1|" + provider + "|" + reference))
	return hex.EncodeToString(sum[:fingerprintBytes])
}

// LocalMasterKey implements [MasterKey] for providers whose key material is
// read into the process, such as the file and env providers of
// internal/adapter/kms. The adapter reads and validates the material and
// builds one with [NewLocalMasterKey].
type LocalMasterKey struct {
	provider  string
	reference string
	key       Key
}

// NewLocalMasterKey returns a master key held in process memory. The provider
// name and reference appear in startup failures and in master_key_reference.
func NewLocalMasterKey(provider, reference string, key Key) *LocalMasterKey {
	return &LocalMasterKey{provider: provider, reference: reference, key: key}
}

var _ MasterKey = (*LocalMasterKey)(nil)

// Provider returns the provider name given to [NewLocalMasterKey].
func (l *LocalMasterKey) Provider() string { return l.provider }

// Reference returns the reference given to [NewLocalMasterKey].
func (l *LocalMasterKey) Reference() string { return l.reference }

// Fingerprint returns [Fingerprint] of the key.
func (l *LocalMasterKey) Fingerprint(context.Context) (string, error) {
	return Fingerprint(l.key), nil
}

// Wrap encrypts plaintext with AES-256-GCM under the key, authenticating aad.
func (l *LocalMasterKey) Wrap(ctx context.Context, plaintext, aad []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("wrap with %s master key: %w", l.provider, err)
	}
	out, err := sealBlob(rand.Reader, l.key, aad, plaintext)
	if err != nil {
		return nil, fmt.Errorf("wrap with %s master key: %w", l.provider, err)
	}
	return out, nil
}

// Unwrap reverses Wrap; a different key or aad is [ErrDecrypt].
func (l *LocalMasterKey) Unwrap(ctx context.Context, wrapped, aad []byte) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("unwrap with %s master key: %w", l.provider, err)
	}
	out, err := openBlob(l.key, aad, wrapped)
	if err != nil {
		return nil, fmt.Errorf("unwrap with %s master key: %w", l.provider, err)
	}
	return out, nil
}

// MaxKeyMaterial bounds what a provider reads for one key: a base64 key with
// whitespace is far below it, so a wrong file is rejected without being read
// whole.
const MaxKeyMaterial = 4096

// ParseKeyMaterial accepts exactly 32 raw bytes, or the base64 text of 32
// bytes (standard or URL alphabet, padded or not, surrounding whitespace
// ignored). A file of exactly 32 bytes is always read as raw. The error never
// includes the material.
func ParseKeyMaterial(raw []byte, allowRaw bool) (Key, error) {
	if allowRaw && len(raw) == KeySize {
		return NewKey(raw)
	}
	text := strings.TrimSpace(string(raw))
	for _, enc := range [...]*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding,
	} {
		decoded, err := enc.DecodeString(text)
		if err != nil {
			continue
		}
		key, kerr := NewKey(decoded)
		clear(decoded)
		if kerr != nil {
			return Key{}, fmt.Errorf("%w: base64 text does not decode to %d bytes", ErrMasterKeyInvalid, KeySize)
		}
		return key, nil
	}
	return Key{}, fmt.Errorf("%w: neither %d raw bytes nor base64 of %d bytes", ErrMasterKeyInvalid, KeySize, KeySize)
}

// FingerprintSuffixLen is how many trailing characters of the fingerprint the
// operator types to confirm the recovery kit (ADR-0042 section 5).
const FingerprintSuffixLen = 8

// FingerprintSuffix returns the last [FingerprintSuffixLen] characters of a
// fingerprint, which the setup flow asks the operator to type.
func FingerprintSuffix(fingerprint string) string {
	if len(fingerprint) <= FingerprintSuffixLen {
		return fingerprint
	}
	return fingerprint[len(fingerprint)-FingerprintSuffixLen:]
}

// ConfirmBackupSuffix checks what the operator typed in the setup flow against
// the last [FingerprintSuffixLen] characters of the fingerprint. Upper case
// letters are accepted; nothing else is (no spaces, no hyphens). A value of
// another length is an error that names the expected length; any mismatch
// matches [ErrBackupAck].
func ConfirmBackupSuffix(fingerprint, typed string) error {
	return confirm(FingerprintSuffix(fingerprint), typed)
}

// ConfirmBackupFingerprint checks the value of ORCON_MASTER_KEY_BACKUP_ACK of
// a headless install against the whole fingerprint, with the same rules as
// [ConfirmBackupSuffix]. There is no way to skip the check (ADR-0042
// section 5).
func ConfirmBackupFingerprint(fingerprint, given string) error {
	return confirm(fingerprint, given)
}

func confirm(want, got string) error {
	if want == "" {
		return fmt.Errorf("%w: no fingerprint to compare with", ErrBackupAck)
	}
	if len(got) != len(want) {
		return fmt.Errorf("%w: expected %d characters, got %d", ErrBackupAck, len(want), len(got))
	}
	if subtle.ConstantTimeCompare([]byte(strings.ToLower(want)), []byte(strings.ToLower(got))) != 1 {
		return ErrBackupAck
	}
	return nil
}
