<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR 0075: Go module and package layout

Status: Proposed

## Context
ADR 0001 joined four independent Go modules with go.work, and ADR-0002 placed shared libraries in packages/go. No package has external consumers, yet every new package costs a module, a go.work entry, a require line and CI edits. The accepted roadmap plans about forty server packages. Independent modules for server-internal code give no versioning benefit and fragment tooling. Community-facing Go code (API client SDK, connector SDK, WASM guest SDK) does need independent semantic versions.

## Decision
1. One module, `github.com/wirvii/orcon`, at the repository root, holds the server: `cmd/orcon` (composition root), `cmd/orcon-catalog` (catalog service), `internal/**`, `bench/`.
2. `internal/` has three package kinds: business-area packages (`internal/<area>`: domain rules plus the small interfaces they consume), adapter packages (`internal/adapter/<technology>`: implement those interfaces), platform packages (`internal/platform/<concern>`: technical building blocks with no business rules). No `models`, `utils`, `common` or `shared` packages.
3. Dependencies point inward: areas never import adapters, database drivers, net/http or serialization formats; platform imports neither areas nor adapters; only `cmd/**` wires adapters. Enforced by depguard in golangci-lint, with planted-violation tests.
4. Wiring is manual constructor injection in `cmd/orcon`. No dependency-injection framework.
5. `packages/go/` contains only community modules, each with its own go.mod, semantic-version tags (`packages/go/<path>/vX.Y.Z`) and no dependency on the server module. go.work joins the root module and these modules; the root go.mod has no replace directives so `go install github.com/wirvii/orcon/cmd/orcon@latest` works.
6. Go version policy: the go directive names the older of the two supported Go releases; the toolchain directive names the latest stable patch release; CI tests both.
7. Amends ADR 0001 (Go part only; pnpm/Turborepo unchanged) and the "shared libraries remain in packages/go" sentence of ADR-0002.

## Consequences
One go.mod to tidy, lint, test and audit; package moves are refactors, not module surgery. Community APIs stay stable behind their own versions. Existing design documents that reference packages/go/* paths are updated to the new layout.

## Alternatives considered
One module per package (rejected: cost without benefit for internal code). One module including community SDKs (rejected: SDK users would download the server dependency graph and inherit its versioning). Root module under apps/orcon (rejected: longer import and install paths for no gain).
