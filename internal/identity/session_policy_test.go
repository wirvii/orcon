// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package identity

import (
	"bytes"
	"testing"
	"time"
)

func TestThrottleDelayDoublesThenCaps(t *testing.T) {
	p := DefaultThrottlePolicy()
	cases := map[int]time.Duration{
		5: 30 * time.Second, 6: 60 * time.Second, 7: 2 * time.Minute, 8: 4 * time.Minute,
		9: 8 * time.Minute, 10: 15 * time.Minute, 40: 15 * time.Minute,
	}
	for failures, want := range cases {
		if got := p.Delay(failures); got != want {
			t.Errorf("Delay(%d) = %s, want %s", failures, got, want)
		}
	}
}

func TestHMACIdentifierIsSixteenBytesAndKeyed(t *testing.T) {
	if _, err := HMACIdentifier(make([]byte, 16)); err == nil {
		t.Fatal("a 16-byte key was accepted")
	}
	a, err := HMACIdentifier(make([]byte, 32))
	if err != nil {
		t.Fatalf("identifier: %v", err)
	}
	key2 := make([]byte, 32)
	key2[0] = 1
	b, err := HMACIdentifier(key2)
	if err != nil {
		t.Fatalf("identifier: %v", err)
	}
	if got := len(a("ada@example.test")); got != IdentifierSize {
		t.Errorf("identifier length = %d, want %d", got, IdentifierSize)
	}
	if bytes.Equal(a("ada@example.test"), b("ada@example.test")) {
		t.Error("two keys give the same identifier")
	}
}

func TestCSRFTokenIsBoundToTheSession(t *testing.T) {
	if CSRFTokenFor("token-one") == CSRFTokenFor("token-two") {
		t.Error("two sessions share a CSRF value")
	}
	first, second := CSRFTokenFor("token-one"), CSRFTokenFor("token-one")
	if first != second {
		t.Error("the CSRF value is not stable for one session")
	}
}
