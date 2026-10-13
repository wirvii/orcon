// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package acme_test

import (
	"testing"

	"github.com/wirvii/orcon/internal/adapter/connectors/acme"
	"github.com/wirvii/orcon/internal/adapter/connectortest"
	"github.com/wirvii/orcon/internal/connector/conformancetest"
)

// TestConformance runs the common conformance suite (ADR-0031 §5), the
// admission lint, the replay of every recorded test and the detector.
func TestConformance(t *testing.T) {
	conformancetest.Run(t, acme.Files)
}

// TestFaults injects the faults of the TE-6 matrix into every operation and
// checks retries, resends, verification and UNCERTAIN outcomes.
func TestFaults(t *testing.T) {
	connectortest.Faults(t, acme.Files)
}

// TestLive runs the operations of tests/live.json against the real service.
// It is skipped unless ORCON_LIVE=1 and the secrets are set; it never runs
// for pull requests.
func TestLive(t *testing.T) {
	connectortest.Live(t, acme.Files)
}
