// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package credential

import (
	"errors"
	"testing"
)

func TestNormalizeHostAddresses(t *testing.T) {
	ok := map[string]string{"API.Example.com.": "api.example.com", "*.example.com": "*.example.com", "10.0.0.5": "10.0.0.5"}
	for in, want := range ok {
		if got, err := normalizeHost(in); err != nil || got != want {
			t.Errorf("normalizeHost(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"*.10.0.0.5", "*.10.0.0", "[::1]", "::1", "https://a.example.com", "a.example.com:8443", "a.example.com/x", "localhost", ""} {
		if _, err := normalizeHost(in); !errors.Is(err, ErrInvalid) {
			t.Errorf("normalizeHost(%q) = %v; want ErrInvalid", in, err)
		}
	}
}
