// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package password hashes and checks passwords with argon2id (ADR-0039 section 1,
// SPEC-004 R-AUTH-01). It is a leaf: it imports no area and no adapter. The hash
// is stored as one encoded string that carries its own parameters, so a hash made
// with older parameters still verifies and can be upgraded on the next sign-in.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"strings"

	"golang.org/x/crypto/argon2"
)

// ErrMalformed means a stored hash is not an encoded argon2id hash.
var ErrMalformed = errors.New("password hash is malformed")

// Argon2id holds the parameters of new hashes. The zero value is not usable; use
// [Default].
type Argon2id struct {
	// Memory is the memory cost in KiB.
	Memory uint32
	// Time is the number of passes.
	Time uint32
	// Threads is the degree of parallelism.
	Threads uint8
	// KeyLength is the length of the derived key in bytes.
	KeyLength uint32
	// SaltLength is the length of the random salt in bytes.
	SaltLength int
}

// Default returns the parameters of ADR-0039 section 1: m = 19 MiB, t = 2, p = 1.
func Default() Argon2id {
	return Argon2id{Memory: 19 * 1024, Time: 2, Threads: 1, KeyLength: 32, SaltLength: 16}
}

// Hash returns the encoded argon2id hash of password with a fresh random salt.
func (a Argon2id) Hash(password string) (string, error) {
	salt := make([]byte, a.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("password salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, a.Time, a.Memory, a.Threads, a.KeyLength)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, a.Memory, a.Time, a.Threads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

// Verify reports whether password matches the encoded hash. It derives the key
// with the parameters the hash carries and compares the two keys in constant
// time. A hash that does not decode fails with [ErrMalformed].
func (Argon2id) Verify(encoded, password string) (bool, error) {
	p, salt, want, err := decode(encoded)
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(password), salt, p.Time, p.Memory, p.Threads, p.KeyLength)
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// NeedsRehash reports whether the encoded hash was made with other parameters
// than the current ones, or does not decode; the caller then hashes the password
// again after a successful check.
func (a Argon2id) NeedsRehash(encoded string) bool {
	p, _, _, err := decode(encoded)
	if err != nil {
		return true
	}
	return p.Memory != a.Memory || p.Time != a.Time || p.Threads != a.Threads
}

// decode reads an encoded argon2id hash into its parameters, salt and key.
func decode(encoded string) (Argon2id, []byte, []byte, error) {
	var out Argon2id
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return out, nil, nil, ErrMalformed
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return out, nil, nil, ErrMalformed
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &out.Memory, &out.Time, &out.Threads); err != nil {
		return out, nil, nil, ErrMalformed
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return out, nil, nil, ErrMalformed
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return out, nil, nil, ErrMalformed
	}
	keyLen := len(key)
	if keyLen > math.MaxUint32 {
		return out, nil, nil, ErrMalformed
	}
	out.KeyLength = uint32(keyLen)
	out.SaltLength = len(salt)
	return out, salt, key, nil
}
