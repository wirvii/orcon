<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0054 — API contract source of truth, versioning, deprecation, documentation and SDK generation

Status: **Accepted** | Date: 2026-10-11 (amended 2026-10-12: owner answers Q1–Q14, human-attention surface; §1 amended 2026-10-12 by SPEC-008: Go generator fixed as `oapi-codegen` strict, ADR-0076; amended 2026-10-12 by ADR-0080: lint toolchain, extensions, executed examples, reference site, changelog; amended 2026-10-13 by SPEC-012 Amendment 2: one built-in lint rule off for protocol errors, oasdiff severity file and events compared as responses, reference-site policy)

Accepted by the owner on 2026-10-12 via SPEC-005.

## Context
P1: the API is versioned (`/v1`) with no breaking changes within a major; breaking changes go to v2 with an announced deprecation window; SDKs are generated for TypeScript, Go and Python. `overview.md` lists API versioning as deferred; this ADR resolves it. ADR-0080 fixes the lint rules, the documentation standard and the contract toolchain.

## Decision

### 1. Spec-first
- The OpenAPI 3.1 document in `packages/openapi/v1/` (split per resource, bundled by `@orcon/openapi-tools` for publication and for code generation) is the source of truth (owner decision Q9: OpenAPI 3.1; the lint forbids constructs the Go generator does not support).
- Generated and committed: Go server types and strict handler interfaces (`internal/adapter/httpapi/apigen`), TypeScript SDK (`packages/js/sdk`), Go SDK (`packages/go/sdk`), Python SDK (`packages/python/orcon`), CLI command metadata (ADR-0058), MCP tool catalog (ADR-0059). CI regenerates and fails on any diff.
- The Go server generator is `oapi-codegen` v2 (Apache-2.0) in strict-server mode targeting `net/http`, configured by `internal/adapter/httpapi/apigen/oapi-codegen.yaml` (ADR-0076; owner decision SPEC-008 Q12, which replaces the earlier plan to choose between `oapi-codegen` and `ogen` by measurement in the first slice). It must support every construct the contract uses; otherwise a lint rule forbids the construct.
- Vendor extensions (full table and shapes in the `orcon-api` skill, `extensions.md`; ADR-0080 §4): `x-orcon-resource`, `x-orcon-plural-exception`, `x-orcon-permission` (a permission that exists in `internal/authz/permissions.yaml`, ADR-0041 §2, or `{allOf}`, `{anyOf}`, `authenticated`, `anonymous` for protocol only), `x-orcon-rate-class`, `x-orcon-stability` (`stable` | `beta` | `experimental`), `x-orcon-idempotency` (`required` | `optional`), `x-orcon-audit` (action name or `none`) and `x-orcon-audit-reason`, `x-orcon-problems`, `x-orcon-cli` (`{ command }` or `{ none: { category, reason } }`, ADR-0058 §3), `x-orcon-mcp` (`{ level, tool?, description? }`, ADR-0059 §2), `x-orcon-examples`, `x-orcon-long-running`, `x-orcon-sort-default`, `x-orcon-include-total`, `x-orcon-sunset` and `x-orcon-deprecation-guide`, `x-orcon-enum-kind`, `x-orcon-external-id`, `x-orcon-volatile`, `x-orcon-data`, `x-orcon-event`, `x-orcon-problem-catalog`. Extension values are not API enums and keep their lowercase form.
- Lint: Redocly CLI (MIT) with the project plugin `orcon` of `@orcon/openapi-tools` (ADR-0080 §9): built-in rules plus 31 project rules at error severity; the only rule turned off for the real contract is the built-in `operation-4xx-problem-details-rfc7807`, because protocol-tagged operations answer errors in their standard's format, and the project rule `orcon/problem-responses` requires problem+json for every other operation — path grammar, action verbs (the closed list of thirty-one), path parameter names, `operationId`, header names, camelCase fields, UUID ids, UPPER_SNAKE enums (including the document schemas the contract references, such as `orcon/flow@1`), time formats, write-only secrets, named and bounded schemas, response shapes without envelopes, pagination, sort allowlist, problem+json and declared problem codes, idempotency, concurrency, rate class, long-running operations, stability, deprecation, permission, audit, CLI and MCP declarations, documentation of operations and fields, examples, events, and the construct subset the Go generator supports.
- Documentation (ADR-0080 §8): every operation has a `summary` (≤ 80 characters, starting with an uppercase verb), a `description` of what it does and when to use it (at least two sentences), a permission, every possible problem code and at least one success example; every property and parameter has a description and, where applicable, a format, allowed values and an example. Permission, rate class, stability, idempotency, problem codes, CLI command and MCP tool are rendered from the extensions, never written by hand.
- Executed examples: each stable or beta operation has `x-orcon-examples` naming a scenario registered in `internal/adapter/httpapi/examplestest/scenarios`; scenarios seed state only through the public API and never reach the internet. The examples runner starts the real binary and checks status, schema and values: on SQLite for every pull request (required CI job `api-contract`) and on PostgreSQL every night (not required; a nightly failure is fixed before other API work).

### 2. Versioning
- Major version in the path: `/api/v1`. A new major lives in `packages/openapi/v2/` and can be served by the same binary alongside v1.
- Within a major, only backward-compatible changes for `stable` and `beta` operations.
- `experimental` operations are excluded from guarantees, require `Orcon-Experimental: <feature>`, are flagged in SDKs, and must graduate or be removed. The editor may use them (P1 forbids private APIs, not unstable public ones).
- `protocol`-tagged endpoints (ADR-0052 §1) follow their external standards and are excluded from this policy.
- Public event types and webhook payloads follow the same rules: each type is an entry of the OpenAPI `webhooks` section with `x-orcon-event` (versioned types; ADR-0055).

### 3. Breaking changes (normative, for `stable`/`beta`)
Removing or renaming an operation, path, parameter, field or response enum value; adding a required request parameter or field; making an optional request field required; narrowing accepted values; changing a type or format; changing defaults with observable effect; changing an error `code` for the same condition; making pagination, sort defaults or idempotency requirements stricter; requiring a stricter permission (security fixes excepted and announced). Non-breaking: new optional request fields, new response fields, new operations, new request enum values, new error codes for new conditions. Clients must ignore unknown response fields and enum values.

### 4. Breaking-change checker and changelog in CI
`oasdiff` (Apache-2.0) compares the current contract with the one of the latest released Orcon tag; any breaking change on `stable`/`beta` fails CI. Synthetic fixtures, one per row of §3, test the checker itself; the check runs `oasdiff breaking --fail-on ERR` with a committed severity file that raises every oasdiff check matching a §3 breaking change to an error and lowers the additions §3 allows to information; event payloads of the `webhooks` section are compared as responses (each event type becomes a synthetic response before the comparison), and a §3 change that oasdiff does not detect is checked by a project comparator over the two bundles. The same tool writes the per-release API changelog `docs/reference/api/changelog/<version>.md`; the release job regenerates it and fails if the committed file differs.

### 5. Deprecation
Deprecated items are marked `deprecated: true` with `x-orcon-sunset` and `x-orcon-deprecation-guide` and respond with `Deprecation` (RFC 9745), `Sunset` (RFC 8594, for removals in the next major) and `Link: <guide>; rel="deprecation"`. Within a major they keep working. A major `vN` is supported for at least 12 months **and** at least two Orcon minor releases (whichever is longer) after `vN+1` is stable (owner decision Q1); announced in release notes and by the public event `api.deprecation_notice`.

### 6. SDKs
- TypeScript: `openapi-typescript` + `openapi-fetch` (MIT), ESM, browsers and Node ≥ 20; the editor depends on it; published as `@orcon/sdk` on npm.
- Go: generated client in `packages/go/sdk`, its own module `github.com/wirvii/orcon/packages/go/sdk` without server dependencies.
- Python: `openapi-python-client` (MIT), Python ≥ 3.10, sync and async; published as `orcon` on PyPI.
- Package names are owner decision Q10; reserving the npm scope/name and the PyPI name is an owner action before the first SDK release.
- Common hand-written layer with identical behaviour: pagination iterators; automatic `Idempotency-Key` on retried POSTs; retries for `429`/`503` honouring `Retry-After`; ETag helpers; SSE subscription with `Last-Event-ID` resume; Standard Webhooks verification; OAuth client-credentials caching and renewal; typed problem errors.
- SemVer per SDK; each release declares supported API majors.
- Released by the same pipeline as the binary with signed provenance (npm provenance, PyPI trusted publishing, Go module tags); contract tests of all three SDKs against a real server on SQLite and PostgreSQL before release.

### 7. Reference site (ADR-0080 §10)
The bundled contract is rendered with Scalar API Reference (MIT; script vendored from the package, no CDN) into a static site with a try-it feature, a generated operation index (`docs/reference/api/README.md`) and the generated problem-code page. Every binary serves the site at `/api/reference/` under the content security policy of ADR-0080 §10, with `/api/reference` redirecting there (ADR-0052 §1); try-it sends only a bearer credential typed by the user and kept in memory, never cookies; the page loads no external resource; the compressed site adds at most 3 MiB to the binary. `make build` builds the site before compiling; it is also attached to every release.

## Consequences
One contract drives server, editor, SDKs, CLI, MCP and the reference; compatibility is enforced mechanically; documentation cannot claim behaviour the server does not have.

## Alternatives considered
Code-first OpenAPI (contract follows implementation accidents); header/date versioning (harder for scripts and proxies); hand-written SDKs (drift across three languages); choosing the Go generator by measurement between `oapi-codegen` and `ogen` (superseded by ADR-0076: `oapi-codegen` strict with `net/http` was fixed by the owner, SPEC-008 Q12); Spectral as linter (needs a separate bundler; ADR-0080); examples checked only against schemas (a valid but wrong example would pass); running examples on both databases for every pull request (doubles the required job time).

## Amendments
Changes applied after acceptance; each entry names its source.

- **SPEC-012 (Amendment 2, 2026-10-13, owner decision).** §1: the built-in rule `operation-4xx-problem-details-rfc7807` is the only lint rule turned off for the real contract (protocol-tagged operations follow their standard's error format; `orcon/problem-responses` covers every other operation). §4: oasdiff runs at `--fail-on ERR` with a committed severity file, event payloads are compared as responses, and a project comparator covers §3 changes oasdiff does not detect; this replaces the event checker planned for the events slice. §7: the reference site is served under the policy of ADR-0080 §10 at `/api/reference/`.
