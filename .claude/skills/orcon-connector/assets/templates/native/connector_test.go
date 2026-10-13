// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

package acmequeue_test

import (
	"testing"

	"github.com/wirvii/orcon/internal/adapter/connectors/acmequeue"
	"github.com/wirvii/orcon/internal/adapter/connectortest"
	"github.com/wirvii/orcon/internal/connector/conformancetest"
)

// TestConformance runs the common conformance suite (ADR-0031 §5), including
// the goroutine leak and cancellation checks for native code, the admission
// lint, the replay of every recorded test and the detector.
func TestConformance(t *testing.T) {
	conformancetest.RunNative(t, acmequeue.Connector{}, acmequeue.Files)
}

// TestFaults injects the faults of the TE-6 matrix into every operation.
func TestFaults(t *testing.T) {
	connectortest.FaultsNative(t, acmequeue.Connector{}, acmequeue.Files)
}

// TestLive is skipped unless ORCON_LIVE=1 and the secrets are set.
func TestLive(t *testing.T) {
	connectortest.LiveNative(t, acmequeue.Connector{}, acmequeue.Files)
}
