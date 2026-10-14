// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package envelope

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

func TestParseKeyMaterial(t *testing.T) {
	t.Parallel()
	raw := bytes.Repeat([]byte{0x11, 0xFE}, KeySize/2)
	tests := []struct {
		name     string
		input    []byte
		allowRaw bool
		wantErr  bool
	}{
		{"raw 32 bytes", raw, true, false},
		{"raw refused when not allowed", raw, false, true},
		{"base64 with newline", []byte(base64.StdEncoding.EncodeToString(raw) + "\n"), false, false},
		{"base64 without padding", []byte(base64.RawStdEncoding.EncodeToString(raw)), false, false},
		{"base64 url alphabet", []byte(base64.URLEncoding.EncodeToString(raw)), false, false},
		{"empty", nil, true, true},
		{"raw 31 bytes", raw[:31], true, true},
		{"base64 of 16 bytes", []byte(base64.StdEncoding.EncodeToString(raw[:16])), true, true},
		{"not base64", []byte("this is not a key at all, definitely!"), true, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			k, err := ParseKeyMaterial(tc.input, tc.allowRaw)
			if tc.wantErr {
				if !errors.Is(err, ErrMasterKeyInvalid) && !errors.Is(err, ErrInvalidKey) {
					t.Errorf("ParseKeyMaterial = %v, want a key error", err)
				}
				if err != nil && len(tc.input) > 0 && bytes.Contains([]byte(err.Error()), tc.input) {
					t.Errorf("error %q repeats the key material", err)
				}
				return
			}
			if err != nil || !bytes.Equal(k.bytes()[:], raw) {
				t.Errorf("ParseKeyMaterial = %v, %v; want the raw key", k, err)
			}
		})
	}
}

func TestLocalMasterKeyWrapUnwrap(t *testing.T) {
	t.Parallel()
	a, b := newMasterKey(t), newMasterKey(t)
	ctx := t.Context()
	aad := []byte("orcon:v1|test")
	wrapped, err := a.Wrap(ctx, []byte("plain"), aad)
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.Unwrap(ctx, wrapped, aad)
	if err != nil {
		t.Fatalf("Unwrap = %v", err)
	}
	mustEqualBytes(t, "Unwrap", got, []byte("plain"))
	if _, err := a.Unwrap(ctx, wrapped, []byte("other aad")); !errors.Is(err, ErrDecrypt) {
		t.Errorf("Unwrap with other aad = %v, want ErrDecrypt", err)
	}
	if _, err := b.Unwrap(ctx, wrapped, aad); !errors.Is(err, ErrDecrypt) {
		t.Errorf("Unwrap with other key = %v, want ErrDecrypt", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := a.Wrap(canceled, []byte("plain"), aad); !errors.Is(err, context.Canceled) {
		t.Errorf("Wrap with a canceled context = %v, want context.Canceled", err)
	}
	if _, err := a.Unwrap(canceled, wrapped, aad); !errors.Is(err, context.Canceled) {
		t.Errorf("Unwrap with a canceled context = %v, want context.Canceled", err)
	}
}

func TestBackupAckConfirmation(t *testing.T) {
	t.Parallel()
	key, err := NewKey(bytes.Repeat([]byte{5}, KeySize))
	if err != nil {
		t.Fatal(err)
	}
	fp := Fingerprint(key)
	if len(fp) != 32 || len(FingerprintSuffix(fp)) != FingerprintSuffixLen {
		t.Fatalf("fingerprint %q (%d chars), suffix %q", fp, len(fp), FingerprintSuffix(fp))
	}
	suffix := FingerprintSuffix(fp)
	tests := []struct {
		name    string
		confirm func() error
		wantErr bool
	}{
		{"typed suffix", func() error { return ConfirmBackupSuffix(fp, suffix) }, false},
		{"typed suffix upper case", func() error { return ConfirmBackupSuffix(fp, strings.ToUpper(suffix)) }, false},
		{"typed suffix with spaces", func() error { return ConfirmBackupSuffix(fp, " "+suffix+" ") }, true},
		{"typed suffix with a hyphen", func() error { return ConfirmBackupSuffix(fp, suffix[:4]+"-"+suffix[4:]) }, true},
		{"headless upper case", func() error { return ConfirmBackupFingerprint(fp, strings.ToUpper(fp)) }, false},
		{"headless with trailing newline", func() error { return ConfirmBackupFingerprint(fp, fp+"\n") }, true},
		{"headless grouped by four", func() error { return ConfirmBackupFingerprint(fp, fp[:4]+"-"+fp[4:]) }, true},
		{"typed suffix wrong", func() error { return ConfirmBackupSuffix(fp, "deadbeef") }, true},
		{"typed nothing", func() error { return ConfirmBackupSuffix(fp, "") }, true},
		{"typed the first characters", func() error { return ConfirmBackupSuffix(fp, fp[:8]) }, true},
		{"headless full fingerprint", func() error { return ConfirmBackupFingerprint(fp, fp) }, false},
		{"headless only the suffix", func() error { return ConfirmBackupFingerprint(fp, suffix) }, true},
		{"headless unset", func() error { return ConfirmBackupFingerprint(fp, "") }, true},
		{"empty fingerprint never confirms", func() error { return ConfirmBackupFingerprint("", "") }, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.confirm()
			if tc.wantErr != errors.Is(err, ErrBackupAck) || (!tc.wantErr && err != nil) {
				t.Errorf("confirmation = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestBackupAckLengthErrorNamesExpectedLength(t *testing.T) {
	t.Parallel()
	fp := strings.Repeat("ab", 16)
	err := ConfirmBackupFingerprint(fp, "abc")
	if !errors.Is(err, ErrBackupAck) || !strings.Contains(err.Error(), "expected 32 characters, got 3") {
		t.Errorf("ConfirmBackupFingerprint(short) = %v, want ErrBackupAck naming 32 characters", err)
	}
	err = ConfirmBackupSuffix(fp, "abc")
	if !errors.Is(err, ErrBackupAck) || !strings.Contains(err.Error(), "expected 8 characters, got 3") {
		t.Errorf("ConfirmBackupSuffix(short) = %v, want ErrBackupAck naming 8 characters", err)
	}
}

// TestFingerprintFormat pins the format of ADR-0042 section 3 as fixed by
// SPEC-004 plan Enmienda 8: the first 16 bytes of SHA-256, 32 lowercase
// hexadecimal characters, no separators. The vectors were computed outside
// this code base.
func TestFingerprintFormat(t *testing.T) {
	t.Parallel()
	material := make([]byte, KeySize)
	for i := range material {
		material[i] = byte(i)
	}
	key, err := NewKey(material)
	if err != nil {
		t.Fatal(err)
	}
	const want = "630dcd2966c4336691125448bbb25b4f"
	if got := Fingerprint(key); got != want {
		t.Errorf("Fingerprint(0x00..0x1f) = %q, want %q", got, want)
	}
	if got := FingerprintSuffix(want); got != "bbb25b4f" {
		t.Errorf("FingerprintSuffix = %q, want bbb25b4f", got)
	}
	// The same key given as base64 has the same fingerprint: it is taken
	// over the raw bytes.
	fromText, err := ParseKeyMaterial([]byte(base64.StdEncoding.EncodeToString(material)), false)
	if err != nil || Fingerprint(fromText) != want {
		t.Errorf("fingerprint of the base64 form = %q, %v; want %q", Fingerprint(fromText), err, want)
	}
	const wantKMS = "67e3eccb7d809f6c59b54ba1e0d07f74"
	if got := KMSFingerprint("awskms", "arn:aws:kms:us-east-1:111122223333:key/abc"); got != wantKMS {
		t.Errorf("KMSFingerprint = %q, want %q", got, wantKMS)
	}
}
