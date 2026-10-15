// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package password_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/wirvii/orcon/internal/platform/password"
)

// cheap keeps the tests fast; the defaults are checked by TestDefaultParameters.
var cheap = password.Argon2id{Memory: 64, Time: 1, Threads: 1, KeyLength: 32, SaltLength: 16}

func TestDefaultParameters(t *testing.T) {
	h, err := password.Default().Hash("a password of twelve")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Errorf("hash = %q, want the ADR-0039 defaults m=19456,t=2,p=1", h)
	}
}

func TestVerifyRoundTrip(t *testing.T) {
	h, err := cheap.Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	cases := []struct {
		name string
		pass string
		want bool
	}{
		{"right password", "correct horse battery staple", true},
		{"wrong password", "correct horse battery stapler", false},
		{"empty password", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := cheap.Verify(h, c.pass)
			if err != nil || got != c.want {
				t.Errorf("Verify = %v, %v; want %v, nil", got, err, c.want)
			}
		})
	}
}

func TestHashesDifferBySalt(t *testing.T) {
	a, _ := cheap.Hash("same")
	b, _ := cheap.Hash("same")
	if a == b {
		t.Error("two hashes of the same password are equal: the salt is not random")
	}
}

func TestMalformedHashFails(t *testing.T) {
	for _, h := range []string{"", "plain text", "$argon2i$v=19$m=64,t=1,p=1$c2FsdA$aGFzaA", "$argon2id$v=19$m=64,t=1,p=1$!!$aGFzaA"} {
		if _, err := cheap.Verify(h, "x"); !errors.Is(err, password.ErrMalformed) {
			t.Errorf("Verify(%q) error = %v, want ErrMalformed", h, err)
		}
		if !cheap.NeedsRehash(h) {
			t.Errorf("NeedsRehash(%q) = false, want true", h)
		}
	}
}

func TestNeedsRehashWhenParametersChange(t *testing.T) {
	h, err := cheap.Hash("pw")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if cheap.NeedsRehash(h) {
		t.Error("a hash made with the current parameters needs no rehash")
	}
	stronger := cheap
	stronger.Time = 2
	if !stronger.NeedsRehash(h) {
		t.Error("a hash with older parameters must be upgraded")
	}
}
