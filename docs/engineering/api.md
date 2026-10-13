<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Public API design

Status: Accepted

Orcon has one public API described by OpenAPI 3.1 in `packages/openapi/v1/`. The editor, the CLI, the SDKs and the MCP server are its clients; none of them has behavior the API does not offer. Every Orcon server shows its own reference for exactly the API it serves at `/api/reference`.

This page is a short orientation for contributors. It contains no rules of its own.

## The API rulebook

The complete API design rulebook is in `.agents/skills/orcon-api/references/`:

- [Paths and operations](../../.agents/skills/orcon-api/references/paths.md) — path grammar, the closed list of action verbs, path parameters, operation ids, headers.
- [Fields and schemas](../../.agents/skills/orcon-api/references/fields.md) — camelCase, UUID ids, UPPER_SNAKE enum values, times, secrets, named schemas.
- [Responses](../../.agents/skills/orcon-api/references/responses.md) — responses without envelopes, status codes.
- [Pagination, filters and sort](../../.agents/skills/orcon-api/references/pagination.md) — cursor pages, typed filters, sort allowlists.
- [Errors and problem codes](../../.agents/skills/orcon-api/references/errors.md) — problem+json and the problem-code catalog.
- [Idempotency and concurrency](../../.agents/skills/orcon-api/references/idempotency-and-concurrency.md) — `Idempotency-Key`, ETags and `If-Match`.
- [Rate limits](../../.agents/skills/orcon-api/references/rate-limits.md) — rate classes and headers.
- [Long-running operations](../../.agents/skills/orcon-api/references/long-running.md) — `202` with an `Operation` resource.
- [Events, webhooks and streams](../../.agents/skills/orcon-api/references/events.md) — event types in the contract, payloads, SSE, signed webhooks.
- [Versioning, compatibility and deprecation](../../.agents/skills/orcon-api/references/versioning.md) — stability, breaking changes, deprecation, changelog.
- [Channels](../../.agents/skills/orcon-api/references/channels.md) — SDKs, CLI commands, MCP levels, permissions, audit.
- [Documentation and executable examples](../../.agents/skills/orcon-api/references/documentation.md) — summaries, descriptions, examples run against the real binary, the reference site.
- [Extensions](../../.agents/skills/orcon-api/references/extensions.md) — every `x-orcon-*` extension.
- [API review](../../.agents/skills/orcon-api/references/review.md) — review checklist, checks, planted cases.

The decisions behind it are [ADR 0052](../adr/0052-public-api-architecture-and-resource-model.md), [ADR 0053](../adr/0053-api-conventions.md), [ADR 0054](../adr/0054-api-contract-versioning-and-sdks.md), [ADR 0055](../adr/0055-event-streaming-and-outbound-webhooks.md), [ADR 0056](../adr/0056-programmatic-identity.md), [ADR 0057](../adr/0057-flows-as-code-git-integration.md), [ADR 0058](../adr/0058-command-line-interface.md), [ADR 0059](../adr/0059-built-in-mcp-server.md) and [ADR 0080](../adr/0080-public-api-design-rules-and-contract-toolchain.md). How Go code is generated from the contract and how handlers are written is in `.agents/skills/orcon-go/references/api-codegen.md` ([link](../../.agents/skills/orcon-go/references/api-codegen.md)).

## The shape in one table

| Topic | In short |
|---|---|
| Paths | `/api/v1/<plural-kebab-collection>/{resourceId}`, at most one parent level; relations that are not ownership use filters |
| Actions | `POST <item>/<verb>` with one of 31 listed verbs (`publish`, `cancel`, `retry`, …) |
| Fields | camelCase names, UUID string ids, `<resource>Id` references, UPPER_SNAKE enum values, `*At` instants in UTC |
| Responses | no envelope: the resource itself, a list as `{ items, nextPageToken? }`, extra data in headers |
| Errors | `application/problem+json` with `type: urn:orcon:problem:<code>` from one catalog |
| Pagination | `limit` and `pageToken`, never offsets |

## Starting a change

Start from the templates in `.agents/skills/orcon-api/assets/templates/` (an action operation, a list operation, an example block, an event and a problem code).

## Before a pull request

Run `make api-check` (lint, compatibility, changelog self-test and reference build), `make api-examples` (the examples against the real binary on SQLite) and `make generate`. The examples also run on PostgreSQL every night; `make api-examples DB=postgres` reproduces that run locally.

## Releases and the reference site

For a release, `make api-changelog VERSION=…` writes the API changelog of that version. `make api-reference` builds the reference site that every binary serves at `/api/reference`.

## Project tooling — mneme

API work in this repository starts from a specification tracked in mneme, the project's tool for the backlog, specifications, acceptance criteria and QA reports. The rulebook is packaged as the `orcon-api` mneme skill; after changing it, run `make skill-sync` and validate it with `mneme skills lint orcon-api` and `mneme skills validate orcon-api`.

## Changing a rule

To change a rule or add an action verb, amend ADR 0080 and the lint plugin (`packages/js/openapi-tools/plugin/`) in the same pull request as the rulebook change.
