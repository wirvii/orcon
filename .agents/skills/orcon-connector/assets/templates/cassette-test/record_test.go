// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package acme_test

import (
	"testing"

	"github.com/wirvii/orcon/internal/adapter/connectors/acme"
	"github.com/wirvii/orcon/internal/adapter/connectortest"
)

// TestContactCreate records the contact.create case against a test account
// when ORCON_RECORD=1 (credential from ORCON_LIVE_ACME_API_KEY) and replays
// the committed cassette offline otherwise. Recording runs the scrubber and
// the detector before writing; a finding stops the write.
func TestContactCreate(t *testing.T) {
	c := connectortest.Case{
		Operation:  "contact.create",
		Name:       "ok",
		Example:    true,
		Parameters: map[string]any{"email": "person1@example.com"},
	}
	if connectortest.Recording() {
		connectortest.Record(t, acme.Files, c)
		return
	}
	connectortest.Replay(t, acme.Files, c)
}

// TestContactGetManyTwoPages replays a paginated case: two interactions, the
// second one following the cursor returned by the first.
func TestContactGetManyTwoPages(t *testing.T) {
	connectortest.Replay(t, acme.Files, connectortest.Case{
		Operation:  "contact.getMany",
		Name:       "two-pages",
		Parameters: map[string]any{"returnAll": true},
	})
}
