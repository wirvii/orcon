---
name: orcon-api
description: Applies the Orcon public API design standards - resource paths with a closed action-verb list, camelCase fields, UPPER_SNAKE enums, UUID ids, responses without envelopes, cursor pagination, problem+json codes, idempotency and ETags, rate classes, long-running operations, events, webhooks and SSE, versioning and deprecation, and CLI and MCP parity. Use when adding, changing or reviewing an endpoint, event, webhook payload, CLI command or MCP tool in the Orcon OpenAPI contract.
license: Apache-2.0
compatibility: Rules in references/ are self-contained; scripts/ require an Orcon repository checkout with Go, Node 22 with pnpm 9, GNU make and git; PostgreSQL only to reproduce the nightly example run.
metadata:
  version: 1.2.1
  pinned: true
  owner: wirvii
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Orcon API

The complete rules are in `references/`. This file is the procedure and the checklist; open the reference named in each step when the step applies.

Code generation and Go handlers follow the `orcon-go` skill (`../orcon-go/references/api-codegen.md`); this skill governs the shape of the contract only.

Precedence: accepted ADRs > accepted `orcon-go` rules > ADR-0052 to ADR-0059 as amended by ADR-0080 > `references/` > reviewer preference.

| Reference | Rules |
|---|---|
| `references/paths.md` | PATH - path grammar, one nesting level, the 31 action verbs, operation ids, headers |
| `references/fields.md` | FIELD - camelCase, ids, UPPER_SNAKE enums, times, secrets, named schemas, bounds |
| `references/responses.md` | RESP - no envelopes: object, items list, problem+json, headers |
| `references/pagination.md` | PAGE - cursor, limit, filters, sort allowlist |
| `references/errors.md` | ERR - problem+json, problem-code catalog, minimum codes per operation |
| `references/idempotency-and-concurrency.md` | IDEM - Idempotency-Key, ETag, If-Match, patch formats |
| `references/rate-limits.md` | RATE - rate classes and headers |
| `references/long-running.md` | LRO - 202, Operation, wait |
| `references/events.md` | EVT - event types, webhooks section, SSE, signing, payloads |
| `references/versioning.md` | VER - stability, breaking changes, deprecation, changelog |
| `references/channels.md` | CHAN - SDK, CLI command or exemption, MCP level, permission, audit |
| `references/documentation.md` | DOC - summaries, descriptions, field docs, executable examples, reference site |
| `references/extensions.md` | every x-orcon extension, its shape and its lint rule |
| `references/review.md` | review checklist, every check, planted cases, inherited checks, mneme |

## When To Use

- You are about to add or change an operation, schema, parameter, problem code or event type under `packages/openapi/`.
- You add or change a CLI command, an MCP tool or a webhook payload.
- You review an API pull request or a change to the contract lint.
- A design asks how a new resource, action or event should be exposed.

## Critical Rules

1. Read the accepted spec that authorizes the change; stop if it is only proposed.
2. Paths are plural kebab-case resources nested at most one level; other relations are filters (PATH-2 to PATH-4).
3. Non-CRUD actions are `POST .../{verb}` with one of the 31 verbs; a new verb amends ADR-0080 (PATH-5).
4. Fields are camelCase, ids are UUID strings named `<resource>Id`, instants are `*At` in UTC, enum values are UPPER_SNAKE, also inside documents the API returns verbatim such as flow documents and connector manifests (FIELD).
5. No envelopes: return the object, a list as `{ items, nextPageToken? }`, errors as problem+json; request id, ETag and limits go in headers (RESP).
6. Lists use `limit` and `pageToken` with a sort allowlist; never offsets (PAGE).
7. List every problem code the operation can return, from the catalog; add a code only for a new condition (ERR).
8. POSTs declare idempotency; edits and guarded actions require `If-Match` (IDEM).
9. Declare the rate class; long work returns 202 with an Operation (RATE, LRO).
10. Events go in the `webhooks` section with `x-orcon-event`; payloads are thin and never carry secrets (EVT).
11. Never break a stable or beta operation; deprecate with sunset, guide and headers (VER).
12. Declare the CLI command or an exemption with a reason, and the MCP level; destructive tools always need human confirmation (CHAN).
13. Declare permission and audit action on every operation (CHAN-7, CHAN-8).
14. Write summary, description and field docs; add an example tied to a scenario that runs against the real binary on SQLite and PostgreSQL (DOC).
15. Regenerate code and commit it as `orcon-go` says; never hand-edit generated files.
16. Comments, docs, commit messages and pull request text never name build assistants or carry AI authorship trailers; mneme may be named.
17. Edit this skill only under `.agents/skills/orcon-api/` and re-run `scripts/project.sh`; never run `mneme skills pin` on it.

## Automated Checks

| Check | What it verifies | How to fix |
|---|---|---|
| `make api-lint` paths | grammar, kebab plural, verbs, parameter names, operation ids, headers | Rename the path, verb or parameter |
| `make api-lint` fields | camelCase, UUID ids, UPPER_SNAKE enums, time formats, secrets, named schemas, bounds | Fix the schema |
| `make api-lint` responses | no envelope; object, items list or problem+json only | Return the resource itself; move auxiliary data to headers |
| `make api-lint` lists | limit, pageToken, sort allowlist and default | Add the parameters |
| `make api-lint` errors | problem+json responses and complete problem codes | Add the missing codes and responses |
| `make api-lint` idempotency and concurrency | Idempotency-Key, If-Match, ETag, 412 and 428 | Declare the headers and responses |
| `make api-lint` rate class and long-running | class matches the method; 202 with Location and Operation | Fix the class or the response |
| `make api-lint` channels | permission exists, audit, CLI command or exemption, MCP level | Add the declarations |
| `make api-lint` documentation | summary, description, field docs, examples with scenarios | Write the missing text or example |
| `make api-lint` events | webhooks entries with x-orcon-event, payload docs, example | Complete the event entry |
| `make api-compat` | no breaking change against the latest release | Make the change additive or plan a new major |
| `make api-examples` | every example runs against the real binary on SQLite (every pull request) | Fix the example, the scenario or the server |
| `make api-examples DB=postgres` (nightly) | the same examples on PostgreSQL | Fix the dialect difference before other API work |
| `make api-reference` | the reference served at /api/reference builds offline with try-it, within 3 MiB | Fix the document or the build |
| `make api-changelog` | the release changelog matches the contract diff | Regenerate and commit |
| `make generate-check` | generated server, SDKs, CLI and MCP catalogs are fresh | Run `make generate` and commit |
| `make spdx`, `make provenance` | SPDX headers; no build-assistant names or AI trailers | Add the header; reword the text |
| `make skill-check` | this skill conforms and its generated copy is identical | Fix `SKILL.md`, then run `make skill-sync` |

## Verification

1. `make api-check` and `make api-examples` exit 0 on your branch; CI runs the same in the `api-contract` job (`sh scripts/verify.sh` runs both).
2. While iterating: `sh scripts/api-change-check.sh`; for storage-sensitive examples also `make api-examples DB=postgres`.
3. Generated code is fresh: `make generate-check`.
4. This skill: `sh scripts/project.sh --home && mneme skills lint orcon-api && mneme skills validate orcon-api` pass.
5. Report the commands you ran, their results and known limitations when you hand back.

## Workflow

1. Read the accepted spec that authorizes the change (specs live in mneme); stop if it is only proposed.
2. Choose the path shape and, for actions, the verb (`references/paths.md`).
3. Copy the matching template from `assets/templates/` into the contract and replace every marker.
4. Define fields and response shape (`references/fields.md`, `references/responses.md`); for lists add pagination and sort (`references/pagination.md`).
5. List problem codes (`references/errors.md`); declare idempotency, If-Match, rate class and long-running behavior (`references/idempotency-and-concurrency.md`, `references/rate-limits.md`, `references/long-running.md`).
6. For events, add the `webhooks` entry (`references/events.md`).
7. Declare permission, audit, CLI command or exemption, and MCP level (`references/channels.md`).
8. Write summary, description and field docs; add an example and its scenario (`references/documentation.md`).
9. Check compatibility and deprecation (`references/versioning.md`); run `make generate` (see `orcon-go`).
10. Run `sh scripts/verify.sh`; paste `assets/pr-api-checklist.md` into the pull request with the `make api-compat` output.
11. Write the commit message in Conventional Commits, describing the change only; citing mneme identifiers (`SPEC-…`, `BL-…`) is fine.
