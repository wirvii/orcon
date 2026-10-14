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
// This package is the skeleton the later cuts fill:
//
//   - [Registry] holds one [Case] per [Subject], a subject being an HTTP route
//     or a store method. [Registry.Missing] and [Registry.Orphans] compare the
//     registry with a [Surface], so a new route or store method without a case
//     fails TestRegistryComplete. The surface is empty today because no route
//     or store method that handles organization data exists yet.
//   - [Env] carries what a case needs: the store under test, the two
//     organizations and whether the run is the PostgreSQL row-level-security
//     only mode (build tag isolation_rls_only), in which the filter of wall 1
//     is switched off by a test-only hook so wall 3 stands alone.
//   - [Leaks] and [AssertNoLeak] search anything returned to, or stored for,
//     the attacking organization for the sentinel values of the other one.
//
// The store is chosen with the -store flag, which follows -args:
//
//	go test ./test/isolation/... -args -store=sqlite
//	go test ./test/isolation/... -args -store=postgres
//	go test -tags isolation_rls_only ./test/isolation/... -args -store=postgres
//
// Cuts that add routes or store methods add their cases here in the same
// change (orcon-db TI-11); the case for webhook path collisions between
// organizations (TestCrossOrgWebhookPathCollision, SPEC-002 T2) joins in the
// same way.
package isolation
