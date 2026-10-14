// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package envelope

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func baseAAD(t *testing.T) AAD {
	t.Helper()
	return AAD{OrgID: newOrg(t), ObjectType: "credential", ObjectID: "cred-1", Version: 3, Field: "password"}
}

func testDEK(t *testing.T) Key {
	t.Helper()
	k, err := GenerateKey(bytes.NewReader(bytes.Repeat([]byte{7}, KeySize)))
	if err != nil {
		t.Fatalf("GenerateKey = %v", err)
	}
	return k
}

func testKID(t *testing.T) KeyID {
	t.Helper()
	return KeyID{Level: LevelOrg, OrgID: newOrg(t), Version: 1}
}

func TestEnvelopeAADBindsOrgAndRow(t *testing.T) {
	t.Parallel()
	aad := baseAAD(t)
	dek := testDEK(t)
	env, err := Seal(dek, testKID(t), aad, []byte("s3cret"))
	if err != nil {
		t.Fatalf("Seal = %v", err)
	}
	got, err := env.Open(dek, aad)
	if err != nil || string(got) != "s3cret" {
		t.Fatalf("Open(same aad) = %q, %v; want s3cret, nil", got, err)
	}

	mutations := map[string]func(a AAD) AAD{
		"organization": func(a AAD) AAD { a.OrgID = newOrg(t); return a },
		"no organization": func(a AAD) AAD {
			a.OrgID = [16]byte{}
			return a
		},
		"object type": func(a AAD) AAD { a.ObjectType = "oauth_token"; return a },
		"object id":   func(a AAD) AAD { a.ObjectID = "cred-2"; return a },
		"version":     func(a AAD) AAD { a.Version++; return a },
		"field":       func(a AAD) AAD { a.Field = "username"; return a },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := env.Open(dek, mutate(aad)); !errors.Is(err, ErrDecrypt) {
				t.Errorf("Open with another %s = %v, want ErrDecrypt", name, err)
			}
		})
	}

	t.Run("another key", func(t *testing.T) {
		t.Parallel()
		other, err := GenerateKey(bytes.NewReader(bytes.Repeat([]byte{9}, KeySize)))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := env.Open(other, aad); !errors.Is(err, ErrDecrypt) {
			t.Errorf("Open with another key = %v, want ErrDecrypt", err)
		}
	})
	t.Run("flipped ciphertext bit", func(t *testing.T) {
		t.Parallel()
		tampered := env
		tampered.CT = bytes.Clone(env.CT)
		tampered.CT[0] ^= 1
		if _, err := tampered.Open(dek, aad); !errors.Is(err, ErrDecrypt) {
			t.Errorf("Open tampered = %v, want ErrDecrypt", err)
		}
	})
}

func TestAADEncoding(t *testing.T) {
	t.Parallel()
	org := newOrg(t)
	tests := []struct {
		name    string
		aad     AAD
		want    string
		wantErr bool
	}{
		{"organization secret", AAD{org, "credential", "c1", 2, "token"},
			"orcon:v1|" + org.String() + "|credential|c1|2|token", false},
		{"instance secret", AAD{ObjectType: "totp", ObjectID: "u1", Version: 0, Field: "seed"},
			"orcon:v1||totp|u1|0|seed", false},
		{"separator in object id", AAD{org, "credential", "a|b", 1, "f"}, "", true},
		{"newline in field", AAD{org, "credential", "c1", 1, "f\n"}, "", true},
		{"empty object type", AAD{org, "", "c1", 1, "f"}, "", true},
		{"empty field", AAD{org, "credential", "c1", 1, ""}, "", true},
		{"negative version", AAD{org, "credential", "c1", -1, "f"}, "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.aad.Bytes()
			if tc.wantErr {
				if !errors.Is(err, ErrInvalidAAD) {
					t.Errorf("Bytes() error = %v, want ErrInvalidAAD", err)
				}
				return
			}
			if err != nil || string(got) != tc.want {
				t.Errorf("Bytes() = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestSealRejectsBadInputs(t *testing.T) {
	t.Parallel()
	dek := testDEK(t)
	if _, err := Seal(dek, KeyID{}, baseAAD(t), []byte("x")); !errors.Is(err, ErrInvalidKeyID) {
		t.Errorf("Seal with zero key id = %v, want ErrInvalidKeyID", err)
	}
	bad := baseAAD(t)
	bad.Field = "a|b"
	if _, err := Seal(dek, testKID(t), bad, []byte("x")); !errors.Is(err, ErrInvalidAAD) {
		t.Errorf("Seal with bad aad = %v, want ErrInvalidAAD", err)
	}
	if _, err := seal(strings.NewReader("short"), dek, testKID(t), baseAAD(t), []byte("x")); err == nil {
		t.Error("seal with an exhausted random source = nil, want error")
	}
}

func TestNonceIsFreshPerSeal(t *testing.T) {
	t.Parallel()
	dek, aad, kid := testDEK(t), baseAAD(t), testKID(t)
	a, err := Seal(dek, kid, aad, []byte("same"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Seal(dek, kid, aad, []byte("same"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(a.Nonce, b.Nonce) || bytes.Equal(a.CT, b.CT) {
		t.Error("two seals of the same value produced the same nonce or ciphertext")
	}
}

func TestEnvelopeBinaryRoundTrip(t *testing.T) {
	t.Parallel()
	dek, aad := testDEK(t), baseAAD(t)
	for _, kid := range []KeyID{testKID(t), {Level: LevelInstance, Version: 4}} {
		env, err := Seal(dek, kid, aad, []byte("value"))
		if err != nil {
			t.Fatal(err)
		}
		b, err := env.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary = %v", err)
		}
		var back Envelope
		if err := back.UnmarshalBinary(b); err != nil {
			t.Fatalf("UnmarshalBinary = %v", err)
		}
		if diff := cmp.Diff(env, back); diff != "" {
			t.Errorf("round trip mismatch (-want +got):\n%s", diff)
		}
		if got, err := back.Open(dek, aad); err != nil || string(got) != "value" {
			t.Errorf("Open after round trip = %q, %v", got, err)
		}
	}
}

func TestUnmarshalRejectsMalformed(t *testing.T) {
	t.Parallel()
	dek, aad := testDEK(t), baseAAD(t)
	env, err := Seal(dek, testKID(t), aad, []byte("v"))
	if err != nil {
		t.Fatal(err)
	}
	good, _ := env.MarshalBinary()
	clone := func(edit func(b []byte) []byte) []byte { return edit(bytes.Clone(good)) }
	tests := map[string][]byte{
		"empty":           nil,
		"truncated":       good[:20],
		"unknown version": clone(func(b []byte) []byte { b[0] = 9; return b }),
		"unknown level":   clone(func(b []byte) []byte { b[1] = 9; return b }),
		"zero version":    clone(func(b []byte) []byte { copy(b[18:22], []byte{0, 0, 0, 0}); return b }),
	}
	for name, b := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var e Envelope
			if err := e.UnmarshalBinary(b); !errors.Is(err, ErrMalformed) {
				t.Errorf("UnmarshalBinary = %v, want ErrMalformed", err)
			}
		})
	}
	if _, err := (Envelope{V: 2}).MarshalBinary(); !errors.Is(err, ErrMalformed) {
		t.Errorf("MarshalBinary of v2 = %v, want ErrMalformed", err)
	}
	if _, err := (Envelope{V: 2}).Open(dek, aad); !errors.Is(err, ErrMalformed) {
		t.Errorf("Open of v2 = %v, want ErrMalformed", err)
	}
}

func FuzzEnvelopeUnmarshal(f *testing.F) {
	dek, derr := GenerateKey(bytes.NewReader(bytes.Repeat([]byte{4}, KeySize)))
	if derr != nil {
		f.Fatal(derr)
	}
	kid := KeyID{Level: LevelInstance, Version: 1}
	env, err := seal(bytes.NewReader(bytes.Repeat([]byte{1}, 64)), dek, kid,
		AAD{ObjectType: "t", ObjectID: "i", Field: "f"}, []byte("v"))
	if err != nil {
		f.Fatal(err)
	}
	seed, _ := env.MarshalBinary()
	f.Add(seed)
	f.Add([]byte{})
	f.Add(seed[:25])
	f.Fuzz(func(t *testing.T, b []byte) {
		var e Envelope
		if err := e.UnmarshalBinary(b); err != nil {
			if !errors.Is(err, ErrMalformed) {
				t.Fatalf("UnmarshalBinary error %v does not match ErrMalformed", err)
			}
			return
		}
		again, err := e.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary of a parsed envelope = %v", err)
		}
		if !bytes.Equal(again, b) {
			t.Fatalf("round trip changed the bytes: %x -> %x", b, again)
		}
	})
}

func TestKeyNeverPrints(t *testing.T) {
	t.Parallel()
	k := testDEK(t)
	var logs bytes.Buffer
	slog.New(slog.NewTextHandler(&logs, nil)).Info("key", slog.Any("k", k))
	text, err := k.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	outputs := []string{
		fmt.Sprint(k), fmt.Sprintf("%v %+v %#v %s %x %q", k, k, k, k, k, k), logs.String(), string(text),
	}
	for _, out := range outputs {
		if !strings.Contains(out, "[REDACTED]") {
			t.Errorf("output %q does not show [REDACTED]", out)
		}
		if strings.Contains(out, "\x07\x07") || strings.Contains(out, "0707") || strings.Contains(out, "BwcH") {
			t.Errorf("output %q leaks key bytes", out)
		}
	}
}

func TestNewKeyRequires32Bytes(t *testing.T) {
	t.Parallel()
	for _, n := range []int{0, 16, 31, 33, 64} {
		if _, err := NewKey(make([]byte, n)); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("NewKey(%d bytes) = %v, want ErrInvalidKey", n, err)
		}
	}
	k, err := NewKey(make([]byte, KeySize))
	if err != nil {
		t.Fatalf("NewKey(32 bytes) = %v", err)
	}
	k.Destroy()
}
