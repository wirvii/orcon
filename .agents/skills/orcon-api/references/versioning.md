<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Versioning, compatibility and deprecation

Status: Accepted

This document defines stability levels, what counts as a breaking change, how compatibility is checked, how operations are deprecated and how each release documents its API changes. It refines ADR-0054. Code generation from the contract is defined in [`../../orcon-go/references/api-codegen.md`](../../orcon-go/references/api-codegen.md) and is not repeated here. MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119. Rule IDs (`VER-…`) are stable.

Precedence: accepted ADRs, then accepted `orcon-go` rules, then the API ADRs (ADR-0052 to ADR-0059) as amended by [ADR 0080](../../../../docs/adr/0080-public-api-design-rules-and-contract-toolchain.md), then this rulebook, then reviewer preference. The check that enforces each rule is named in brackets (an `orcon/<rule>` lint rule, a make target, or `review`).

## Stability

- **VER-1** Every operation and event declares `x-orcon-stability: stable | beta | experimental`; experimental requires `x-orcon-experimental-feature` (the feature name) and `Orcon-Experimental: <feature>` (a comma-separated list) and is outside the guarantee. [`stability`]

## Breaking and non-breaking changes

- **VER-2** Breaking for stable/beta (ADR-0054 §3): removing or renaming an operation, path, parameter, field or response enum value; adding a required request parameter or field; making an optional request field required; narrowing accepted values; changing a type or format; changing an observable default; changing an error code for the same condition; stricter pagination, sort default or idempotency; stricter permission (security fixes excepted and announced). [`make api-compat`, review]
- **VER-3** Non-breaking: new optional request fields, new response fields, new operations, new request enum values, new error codes for new conditions. Clients ignore unknown response fields and enum values.
- **VER-4** `make api-compat` compares with the latest released tag using `oasdiff breaking --fail-on ERR` and the committed severity file `packages/js/openapi-tools/oasdiff-levels.txt`, which raises to error every oasdiff check that detects a VER-2 change and lowers to information the additions VER-3 allows. Event payloads in `webhooks` are compared as responses (`scripts/events-as-responses.mjs` turns each event type into a synthetic response first); a VER-2 change oasdiff does not detect is checked by `scripts/compat-extra.mjs`. A breaking change on stable/beta fails CI; `--fail-on WARN` is never used. [`make api-compat`] [`make api-compat`]
- **VER-5** Breaking changes go to a new major (`packages/openapi/v2/`), served by the same binary. [review]

## Deprecation and changelog

- **VER-6** Deprecation: `deprecated: true` plus `x-orcon-sunset` and `x-orcon-deprecation-guide`; responses send `Deprecation`, `Sunset` and `Link: <guide>; rel="deprecation"`; `api.deprecation_notice` event; a major lives ≥ 12 months and ≥ two Orcon minor releases after the next major is stable. [`deprecation`]
- **VER-7** Each release has `docs/reference/api/changelog/<version>.md` produced by `make api-changelog`. [`make api-changelog-check`]

## Generated artifacts and the generator subset

- **VER-8** Generated artifacts (bundle, server interfaces, SDKs, CLI metadata, MCP catalog) are regenerated and committed; see [`../../orcon-go/references/api-codegen.md`](../../orcon-go/references/api-codegen.md). [`make generate-check`]
- **VER-9** Constructs the Go generator does not support are forbidden. Measured on 2026-10-13 against `oapi-codegen` v2.8.0, the version pinned in `tools/go.mod` (ADR-0076): multi-type `type` arrays other than `[T, "null"]` make generation fail; `prefixItems`, `$dynamicRef`, `$dynamicAnchor`, `patternProperties`, `not`, `if`/`then`/`else`, `dependentSchemas` and `unevaluatedProperties` are dropped silently, so the generated code would accept what the contract forbids. `oneOf` without `discriminator` is forbidden too. The list is measured again whenever the pinned generator changes; `api-codegen.md` governs generation. [`codegen-subset`]
