<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Fields and schemas

Status: Accepted

This document defines how properties, ids, enum values, times, booleans, nulls, secrets and schemas are written in the OpenAPI contract `packages/openapi/v1/` and in the documents the API returns verbatim. The enum rule amends ADR-0052, ADR-0053, ADR-0055, ADR-0056, ADR-0059 and ADR-0073 through [ADR 0080](../../../../docs/adr/0080-public-api-design-rules-and-contract-toolchain.md) §3. MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119. Rule IDs (`FIELD-…`) are stable.

Precedence: accepted ADRs, then accepted `orcon-go` rules, then the API ADRs (ADR-0052 to ADR-0059) as amended by ADR-0080, then this rulebook, then reviewer preference. The check that enforces each rule is named in brackets (an `orcon/<rule>` lint rule, a make target, or `review`).

## Names and ids

- **FIELD-1** Property, query and body names are camelCase; acronyms are words (`apiKeyId`, `callbackUrl`). [`field-camel-case`]
- **FIELD-2** `id` is a read-only UUID string; references are `<resource>Id`/`<resource>Ids`; never numeric ids; provider ids need `x-orcon-external-id: true`. [`id-format`]

## Enum values

- **FIELD-3** Enum and state values are UPPER_SNAKE matching the engine (`RUNNING`, `UNCERTAIN_OPERATION`, `GROUPED`). The rule covers every value that crosses a public interface, including documents the API returns verbatim and whose schemas the contract references (`orcon/flow@1` flow documents, connector manifests): their enum values are UPPER_SNAKE too. Identifier catalogs (event types, permission names, problem codes, operation ids, tool names, media types, external standard values) declare `x-orcon-enum-kind: identifier`. Values of `x-orcon-*` extensions are contract metadata and keep the form in [extensions.md](extensions.md). Clients must accept unknown response values ([VER-3](versioning.md)). [`enum-upper-snake`]

## Times, sizes, booleans and nulls

- **FIELD-4** Instants are `*At` with `format: date-time` in UTC with `Z` (also `since`, `until`); dates `*Date` with `format: date`; durations ISO 8601 with `format: duration`; sizes `*Bytes` integers. [`time-format`]
- **FIELD-5** Booleans are positive adjectives without `is` (`protected`, `enabled`); never a negative flag. [review]
- **FIELD-6** Prefer absent over `null`; `null` only where it means "cleared" (merge patch); written `type: [T, "null"]`. [review]

## Secrets and protected data

- **FIELD-7** Secrets and secret-bearing fields are `writeOnly: true` (and `format: password` for strings), never returned; examples use `example-secret-<n>`. [`secret-fields`]
- **FIELD-8** Execution data fields carry `x-orcon-data: protected` and are present only with `execution:read-data` (otherwise `dataHidden: true` on the resource). [`events`, review]

## Schemas and bounds

- **FIELD-9** Every request body and response body is a named schema in `components/schemas` (PascalCase; requests `<OperationId>Request`); request schemas set `additionalProperties: false`; response schemas stay open (clients ignore unknown fields). [`named-schemas`]
- **FIELD-10** Inputs are bounded: strings have `maxLength`, `format`, `enum` or `pattern`; request arrays have `maxItems`; numbers have `minimum`/`maximum` where meaningful. [`named-schemas`]
