<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Paths and operations

Status: Proposed — pending owner approval

This document defines how public API paths, action verbs, path parameters, operation ids and custom headers are named in the OpenAPI contract `packages/openapi/v1/`. It amends ADR-0052 §2 and §3 through [ADR 0080](../../../../docs/adr/0080-public-api-design-rules-and-contract-toolchain.md) §2. MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119. Rule IDs (`PATH-…`) are stable.

Precedence: accepted ADRs, then accepted `orcon-go` rules, then the API ADRs (ADR-0052 to ADR-0059) as amended by ADR-0080 (while ADR-0080 is Proposed, as design input), then this rulebook, then reviewer preference. The check that enforces each rule is named in brackets: an `orcon/<rule>` lint rule of `@orcon/openapi-tools`, a make target, or `review` when only a reviewer can check it.

## Prefix and grammar

- **PATH-1** Every operation path starts at the server URL `/api/v1`; paths in the document omit the prefix. Protocol-tagged operations (ADR-0052 §1) are outside this file. [`path-shape`]
- **PATH-2** Grammar (each path item declares `x-orcon-resource`, see [extensions.md](extensions.md)):
  - `collection`: `/{c}` or `/{parent}/{parentId}/{c}` — one parent level, only when the child cannot exist without the parent (ownership or tenancy scope: `/projects/{projectId}/flows`, `/flows/{flowId}/versions`).
  - `item`: `/{c}/{cId}` (globally unique id, flat) or `/{parent}/{parentId}/{c}/{cId}` for dependent sub-resources (`/flows/{flowId}/versions/{version}`, `/executions/{executionId}/awaiting-human/{attentionId}`).
  - `singleton`: `<item>/{s}` (`/flows/{flowId}/draft`, `/projects/{projectId}/git-binding`) or `/users/me`, `/instance/{s}`.
  - `view`: read-only cross-resource collection (`/orgs/{orgId}/awaiting-human`); `GET` only.
  - `action`: `<item or singleton>/{verb}` — `POST` only.
  - `stream`: `<collection>/stream`, `GET`, `text/event-stream` (`/events/stream`, `/executions/{executionId}/events/stream`).
  - Namespaces: `instance`, `catalog`, `users/me`. At most two path parameters. [`path-shape`]
- **PATH-3** Segments are kebab-case; collections are plural nouns (`flows`, `credential-types`); an invariant noun needs `x-orcon-plural-exception: <reason>`. [`path-shape`]
- **PATH-4** Relations that are not ownership use typed filters, not nesting: `GET /projects/{projectId}/executions?flowId=…`, never `GET /flows/{flowId}/executions`. [`path-shape`, review]

## Action verbs

- **PATH-5** Action verbs (closed list of 31; adding one amends ADR-0080 and `plugin/verbs.js` in the same pull request):

  | Verb | Meaning |
  |---|---|
  | `activate` | make a resource take effect (flow triggers listen) |
  | `deactivate` | stop it taking effect, keep it |
  | `enable` | allow an account, source or policy to be used |
  | `disable` | forbid its use without deleting it |
  | `pause` | temporarily stop processing or delivery |
  | `resume` | restart what was paused |
  | `publish` | create an immutable version from a draft |
  | `rollback` | make an earlier version current |
  | `move` | change the parent (flow to another project) |
  | `cancel` | stop a pending or running process |
  | `retry` | run a failed process again as a new attempt |
  | `continue` | let a waiting execution proceed |
  | `repeat` | run the waiting step again |
  | `acknowledge` | record that a person is handling an item; stops escalation |
  | `resolve` | close an uncertain or conflicting state with a stated outcome |
  | `complete` | submit the result of a human task |
  | `signal` | deliver an external signal to a wait |
  | `approve` | accept a pending confirmation |
  | `reject` | refuse a pending confirmation |
  | `rotate` | replace a secret with an overlap period |
  | `revoke` | permanently invalidate a credential, key or token |
  | `test` | check a connection or credential without other effects |
  | `validate` | check a document without changing state |
  | `evaluate` | compute a preview without changing state |
  | `export` | produce a downloadable representation |
  | `sync` | reconcile with an external system |
  | `redeliver` | send one past delivery again |
  | `replay` | send a range of past events again |
  | `upgrade` | move an installed component to a newer version |
  | `authorize` | start an OAuth consent for a credential |
  | `reindex` | rebuild a derived index (for example the vectors of a knowledge store) as a long-running operation |

  [`action-verb`]

  Renames fixed by ADR-0080 §2: `flows/{flowId}/draft/expression-preview` becomes `flows/{flowId}/draft/evaluate`; `credentials/{credentialId}/oauth/authorize` becomes `credentials/{credentialId}/authorize`.

## Parameters, operation ids and headers

- **PATH-6** Path parameters are `<resource>Id` in camelCase with `format: uuid` (`{flowId}`, never `{id}`); `{version}` for version numbers. [`path-param-name`]
- **PATH-7** `operationId` is lowerCamel `<verb><Resource>`: `list` (plural resource), `get`, `create`, `update` (PATCH), `replace` (PUT), `delete`, `stream`, or an action verb (`publishFlow`). Unique and never renamed once stable. [`operation-id`]
- **PATH-8** Custom headers use the `Orcon-` prefix (`Orcon-Request-Id`, `Orcon-Org`, `Orcon-Experimental`); never `X-`. [`header-names`]
- **PATH-9** Outside `/api/v1` the API role serves only the routes listed in ADR-0052 §1 as amended by ADR-0080 §10 (including `/api/reference`); a new route outside `/api/v1` needs an ADR. [review]
