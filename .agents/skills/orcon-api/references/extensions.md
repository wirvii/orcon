<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Extensions

Status: Proposed — pending owner approval

This document is the canonical list of every `x-orcon-*` extension of the OpenAPI contract: where it goes, its shape, when it is required and which rule checks it. It amends ADR-0054 §1 and ADR-0059 §2 through [ADR 0080](../../../../docs/adr/0080-public-api-design-rules-and-contract-toolchain.md) §4. An extension that is not in this table is not allowed. MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119.

Precedence: accepted ADRs, then accepted `orcon-go` rules, then the API ADRs (ADR-0052 to ADR-0059) as amended by ADR-0080 (while ADR-0080 is Proposed, as design input), then this rulebook, then reviewer preference.

## Extension table

| Extension | Where | Shape | Required | Rule |
|---|---|---|---|---|
| `x-orcon-resource` | path item | `collection \| item \| singleton \| view \| action \| stream` | always | `path-shape` |
| `x-orcon-plural-exception` | path item | string reason | invariant noun | `path-shape` |
| `x-orcon-permission` | operation | permission \| `{allOf}` \| `{anyOf}` \| `authenticated` \| `anonymous` | always | `permission` |
| `x-orcon-rate-class` | operation | `read \| write \| run \| stream` | always | `rate-class` |
| `x-orcon-stability` | operation, webhook | `stable \| beta \| experimental` | always | `stability` |
| `x-orcon-idempotency` | operation (POST) | `required \| optional` | every POST | `idempotency` |
| `x-orcon-audit` / `x-orcon-audit-reason` | operation | action name \| `none` / string | always / when mutation is `none` | `audit` |
| `x-orcon-problems` | operation | list of catalog codes | always (non-protocol) | `problem-codes-declared` |
| `x-orcon-cli` | operation | `{command}` \| `{none:{category,reason}}` | always | `cli-declared` |
| `x-orcon-mcp` | operation | `{level, tool?, description?}` | always | `mcp-declared` |
| `x-orcon-examples` | operation | list of example cases | stable/beta | `doc-examples` |
| `x-orcon-long-running` | operation | `{kind}` | LRO | `long-running` |
| `x-orcon-sort-default` | `sort` parameter | field expression | when `sort` exists | `sort-allowlist` |
| `x-orcon-include-total` | list operation | boolean | to allow `includeTotal` | `list-pagination` |
| `x-orcon-sunset` / `x-orcon-deprecation-guide` | operation, parameter, field | RFC 3339 date / URL | with `deprecated: true` | `deprecation` |
| `x-orcon-enum-kind` | schema with enum | `identifier` | identifier catalogs | `enum-upper-snake` |
| `x-orcon-external-id` | id field | boolean | provider ids | `id-format` |
| `x-orcon-volatile` | response field | boolean | compared by presence only | `examplestest` |
| `x-orcon-data` | field | `protected` | execution data | `events` |
| `x-orcon-event` | webhooks entry | `{type, cloudEventsType, permission, stability, since}` | every event | `events` |
| `x-orcon-problem-catalog` | `ProblemCode` schema | map code → `{status, title, when, retryable}` | once | `problem-codes-declared` |

New with ADR-0080: `x-orcon-resource`, `x-orcon-problems`, `x-orcon-examples`, `x-orcon-long-running`, `x-orcon-event`, `x-orcon-sort-default`, `x-orcon-enum-kind`, `x-orcon-volatile`, `x-orcon-deprecation-guide`. `x-orcon-mcp` changes to `{ level, tool?, description? }` and the key `confirm` no longer exists ([CHAN-5](channels.md)).

Extension values are contract metadata, not API data: they keep the forms in this table and are outside [FIELD-3](fields.md).
