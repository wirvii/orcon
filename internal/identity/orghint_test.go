// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package identity

import "testing"

// TestOrgHintIsAContextOnly (ADR-0038 section 6): a client names its organization with Orcon-Org; a value that is not
// an identifier leaves the context empty, so every protected operation is refused rather than guessed.
func TestOrgHintIsAContextOnly(t *testing.T) {
	const valid = "0191f6a2-3c4d-7e8f-9a0b-1c2d3e4f5a71"
	if got := orgHint(""); !got.IsZero() {
		t.Errorf("no hint = %v, want the empty context", got)
	}
	if got := orgHint("acme"); !got.IsZero() {
		t.Errorf("a slug = %v, want the empty context", got)
	}
	got := orgHint(valid)
	if got.IsZero() || got.String() != valid {
		t.Errorf("valid hint = %v, want %s", got, valid)
	}
}
