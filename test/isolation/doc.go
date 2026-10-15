// SPDX-FileCopyrightText: The Orcon Authors
// SPDX-License-Identifier: Apache-2.0

// Package isolation is wall 4 of organization isolation (ADR-0046 section 2,
// ADR-0050 section 1, SPEC-004 R-ISO-10): the adversarial cross-organization
// battery. Two organizations, A and B, exist in every run; a case acts as a
// member of A (every role, Owner included) and tries to read, list, count,
// modify, delete, export or reference something of B. The only acceptable
// outcomes are not-found, empty or validation errors, and nothing that carries
// a value planted in B (see [Org.Sentinel]).
//
//   - [Registry] holds one [Case] per [Subject], a subject being an HTTP route
//     or a store method. [Registry.Missing] and [Registry.Orphans] compare the
//     registry with a [Surface], so a new route or store method without a case
//     fails TestRegistryComplete. The surface of the store methods is
//     [StoreSurface]: every method of [Stores], and TestSurfaceIsEverythingTheAdaptersImplement
//     ties that list to what the adapters implement. No HTTP route handles
//     organization data yet, so [StoreSurface] has no routes.
//   - [Env] carries what a case needs: the store under test, seeded by [Seed]
//     with both organizations, and whether the run is the PostgreSQL
//     row-level-security only mode (build tag isolation_rls_only). That mode needs
//     a test-only hook that switches the filter of wall 1 off; the hook arrives
//     with the full battery (SPEC-004 SE17), and until then the cases are skipped
//     in that mode instead of claiming to test wall 3 alone.
//   - [Leaks] and [AssertNoLeak] search anything returned to, or stored for,
//     the attacking organization for the sentinel values of the other one.
//
// The store is chosen with the -store flag, which follows -args (auto, the default,
// is SQLite plus PostgreSQL when ORCON_TEST_POSTGRES=1):
//
//	go test ./test/isolation/... -args -store=sqlite
//	go test ./test/isolation/... -args -store=postgres
//	go test -tags isolation_rls_only ./test/isolation/... -args -store=postgres
//
// Every case gets a fresh database through internal/adapter/storetest, the only
// adapter package the test files here may import (a proposed exception to
// architecture-layers.md AL-2.4, see .golangci.yml).
//
// Cuts that add routes or store methods add their cases here in the same
// change (orcon-db TI-11); the case for webhook path collisions between
// organizations (TestCrossOrgWebhookPathCollision, SPEC-002 T2) joins in the
// same way.
package isolation
