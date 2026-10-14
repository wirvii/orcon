// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package envelope

import (
	"crypto/rand"
	"fmt"
	"io"
)

// FormatV1 is the version of the ciphertext envelope this package writes.
const FormatV1 = 1

// nonceSize is the AES-GCM nonce size: 96 bits.
const nonceSize = 12

// tagSize is the AES-GCM authentication tag size.
const tagSize = 16

// Envelope is the ciphertext of one value: {v, kid, nonce, ct}
// (ADR-0042 section 1). KID names the organization or instance key version
// that wraps the data key which encrypted the value; the wrapped data key is
// stored beside the envelope by the caller. CT holds the ciphertext and the
// authentication tag.
type Envelope struct {
	V     int
	KID   KeyID
	Nonce []byte
	CT    []byte
}

// Seal encrypts plaintext with the data key under aad. The key identifier kid
// names the key that wraps dek; it is recorded so the data key can be found
// again.
func Seal(dek Key, kid KeyID, aad AAD, plaintext []byte) (Envelope, error) {
	return seal(rand.Reader, dek, kid, aad, plaintext)
}

func seal(random io.Reader, dek Key, kid KeyID, aad AAD, plaintext []byte) (Envelope, error) {
	if err := kid.Validate(); err != nil {
		return Envelope{}, fmt.Errorf("seal: %w", err)
	}
	ad, err := aad.Bytes()
	if err != nil {
		return Envelope{}, fmt.Errorf("seal: %w", err)
	}
	blob, err := sealBlob(random, dek, ad, plaintext)
	if err != nil {
		return Envelope{}, fmt.Errorf("seal: %w", err)
	}
	return Envelope{V: FormatV1, KID: kid, Nonce: blob[:nonceSize:nonceSize], CT: blob[nonceSize:]}, nil
}

// Open decrypts the envelope with the data key. It fails with [ErrDecrypt]
// when the key, the aad or the ciphertext differ from the ones that sealed it,
// which is what stops a ciphertext copied to another row or organization.
func (e Envelope) Open(dek Key, aad AAD) ([]byte, error) {
	if e.V != FormatV1 {
		return nil, fmt.Errorf("open: %w: unsupported envelope version %d", ErrMalformed, e.V)
	}
	if len(e.Nonce) != nonceSize {
		return nil, fmt.Errorf("open: %w: nonce of %d bytes", ErrMalformed, len(e.Nonce))
	}
	ad, err := aad.Bytes()
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	blob := make([]byte, 0, len(e.Nonce)+len(e.CT))
	blob = append(append(blob, e.Nonce...), e.CT...)
	plaintext, err := openBlob(dek, ad, blob)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	return plaintext, nil
}

// MarshalBinary returns the stored form: version byte, key identifier (21
// bytes), nonce (12 bytes), ciphertext. The form is opaque to the database.
func (e Envelope) MarshalBinary() ([]byte, error) {
	if e.V != FormatV1 {
		return nil, fmt.Errorf("%w: unsupported envelope version %d", ErrMalformed, e.V)
	}
	if err := e.KID.Validate(); err != nil {
		return nil, err
	}
	if len(e.Nonce) != nonceSize || len(e.CT) < tagSize {
		return nil, fmt.Errorf("%w: nonce of %d bytes, ciphertext of %d bytes", ErrMalformed, len(e.Nonce), len(e.CT))
	}
	out := make([]byte, 0, 1+keyIDBinarySize+nonceSize+len(e.CT))
	out = append(out, byte(e.V))
	out = e.KID.appendBinary(out)
	out = append(out, e.Nonce...)
	return append(out, e.CT...), nil
}

// UnmarshalBinary parses the form MarshalBinary writes.
func (e *Envelope) UnmarshalBinary(b []byte) error {
	const header = 1 + keyIDBinarySize + nonceSize
	if len(b) < header+tagSize {
		return fmt.Errorf("%w: envelope of %d bytes", ErrMalformed, len(b))
	}
	if b[0] != FormatV1 {
		return fmt.Errorf("%w: unsupported envelope version %d", ErrMalformed, b[0])
	}
	kid, err := keyIDFromBinary(b[1 : 1+keyIDBinarySize])
	if err != nil {
		return err
	}
	*e = Envelope{
		V:     FormatV1,
		KID:   kid,
		Nonce: append([]byte(nil), b[1+keyIDBinarySize:header]...),
		CT:    append([]byte(nil), b[header:]...),
	}
	return nil
}
