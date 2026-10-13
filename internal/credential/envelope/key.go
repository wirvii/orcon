// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package envelope

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"io"
	"log/slog"
)

// KeySize is the size in bytes of every key of the hierarchy: AES-256.
const KeySize = 32

const redacted = "[REDACTED]"

// Key is 256-bit key material. It prints, logs and marshals as [REDACTED] so
// it cannot reach a log or a response by accident (go-style GS-O5).
//
// The bytes sit behind a function on purpose: fmt cannot call methods on
// unexported fields, so a struct that holds a Key in a private field, printed
// with %v, %+v, %#v or a wrong verb, would otherwise print the array, and fmt
// even follows a pointer to an array at the top level. A function value can
// only be printed as an address. The consequence is that copies of a Key share
// the bytes, and [Key.Destroy] wipes them for every copy; code that keeps a Key
// beyond its caller's use takes its own copy with clone. The wipe is a best
// effort: the runtime may have moved the value.
type Key struct {
	get func() *[KeySize]byte
}

// newKey allocates zeroed key bytes and returns the Key and the bytes to fill.
func newKey() (Key, *[KeySize]byte) {
	b := new([KeySize]byte)
	return Key{get: func() *[KeySize]byte { return b }}, b
}

// bytes returns the key bytes, or nil for the zero Key.
func (k Key) bytes() *[KeySize]byte {
	if k.get == nil {
		return nil
	}
	return k.get()
}

// NewKey copies material into a Key. It returns [ErrInvalidKey] unless
// material is exactly [KeySize] bytes.
func NewKey(material []byte) (Key, error) {
	if len(material) != KeySize {
		return Key{}, fmt.Errorf("%w: got %d bytes", ErrInvalidKey, len(material))
	}
	k, b := newKey()
	copy(b[:], material)
	return k, nil
}

// GenerateKey reads a fresh key from random, normally [crypto/rand.Reader].
func GenerateKey(random io.Reader) (Key, error) {
	k, b := newKey()
	if _, err := io.ReadFull(random, b[:]); err != nil {
		return Key{}, fmt.Errorf("generate key: %w", err)
	}
	return k, nil
}

// Destroy overwrites the key material with zeros, for every copy of the Key.
func (k Key) Destroy() {
	if b := k.bytes(); b != nil {
		clear(b[:])
	}
}

// clone returns a Key with its own copy of the bytes.
func (k Key) clone() Key {
	b := k.bytes()
	if b == nil {
		return Key{}
	}
	c, cb := newKey()
	*cb = *b
	return c
}

// String returns [REDACTED].
func (Key) String() string { return redacted }

// GoString returns [REDACTED] for the %#v verb.
func (Key) GoString() string { return redacted }

// Format writes [REDACTED] for every verb, including %x and %v.
func (Key) Format(f fmt.State, _ rune) { _, _ = io.WriteString(f, redacted) }

// LogValue makes [log/slog] record [REDACTED].
func (Key) LogValue() slog.Value { return slog.StringValue(redacted) }

// MarshalText makes text and JSON encoders write [REDACTED].
func (Key) MarshalText() ([]byte, error) { return []byte(redacted), nil }

// gcm builds the AES-256-GCM instance of a key. The nonce is the standard 96
// bits (ADR-0042 section 1).
func (k Key) gcm() (cipher.AEAD, error) {
	b := k.bytes()
	if b == nil {
		return nil, fmt.Errorf("%w: empty key", ErrInvalidKey)
	}
	block, err := aes.NewCipher(b[:])
	if err != nil {
		return nil, fmt.Errorf("aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	return aead, nil
}

// sealBlob encrypts plaintext and returns nonce||ciphertext||tag.
func sealBlob(random io.Reader, key Key, aad, plaintext []byte) ([]byte, error) {
	aead, err := key.gcm()
	if err != nil {
		return nil, err
	}
	out := make([]byte, aead.NonceSize(), aead.NonceSize()+len(plaintext)+aead.Overhead())
	if _, err := io.ReadFull(random, out); err != nil {
		return nil, fmt.Errorf("read nonce: %w", err)
	}
	return aead.Seal(out, out, plaintext, aad), nil
}

// openBlob reverses sealBlob. Any authentication failure is [ErrDecrypt].
func openBlob(key Key, aad, blob []byte) ([]byte, error) {
	aead, err := key.gcm()
	if err != nil {
		return nil, err
	}
	n := aead.NonceSize()
	if len(blob) < n+aead.Overhead() {
		return nil, fmt.Errorf("%w: ciphertext too short", ErrMalformed)
	}
	plaintext, err := aead.Open(nil, blob[:n], blob[n:], aad)
	if err != nil {
		return nil, ErrDecrypt
	}
	return plaintext, nil
}
