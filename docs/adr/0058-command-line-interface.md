<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0058 — Command-line interface

Status: **Accepted** | Date: 2026-10-11 (amended 2026-10-12: owner answers Q1–Q14, human-attention surface; ADR-0080 §5 CLI coverage; amended 2026-10-13: `orcon serve` is the only server command)

Accepted by the owner on 2026-10-12 via SPEC-005.

## Context
Given (BL-005): the CLI covers the full public API and is built on it. P3 needs pull/push/diff for CI/CD. ADR-0002 (Accepted) defines one binary with runtime roles and leaves the detailed CLI contract to be specified. SPEC-004 adds local administrative commands (`orcon key …`, `orcon audit verify`, `orcon db setup-roles`, …) that operate on the instance directly.

## Decision

### 1. Same binary, client and server
- Server: `orcon serve --role <standalone|api|scheduler|worker>`. The root command starts no server: `orcon` with no subcommand or an unknown one prints usage and exits 2.
- Local administrative commands of SPEC-001/003/004 (store migration, key ceremony and rotation, audit verification, database roles) keep operating on the local instance and are documented as such.
- Client commands only call the public API (ADR-0052) through the Go SDK (ADR-0054); they never access the database. Offline exceptions: `flows validate`, `flows fmt`, `flows export --bpmn` of local files (same Go packages as the server).
- A client-only build of the same source (build tag; no embedded UI, no store adapters, no server roles) is published next to the full binary for lightweight CI (owner decision Q6). It offers the same client commands and offline validation; `orcon serve` and local administrative commands exit with a clear error. A CI check verifies the client-only binary links no store adapter or UI package.

### 2. Contexts and authentication
- `orcon context add|use|list|remove`: named contexts (URL, organization, default project, credential reference) in the user config directory.
- Credentials in the OS keychain (`zalando/go-keyring`, MIT) with a `0600` file fallback; `ORCON_URL`, `ORCON_TOKEN`, `ORCON_CONTEXT`, `ORCON_CLIENT_ID`/`ORCON_CLIENT_SECRET` override.
- `orcon login`: OAuth device authorization grant (RFC 8628); the user confirms the code in an interactive session (ADR-0038 §5, subject to the organization's authentication policy) and receives a personal key (`pak`, default and maximum per the personal-key policy of ADR-0056 §5 — 30 days by default; scopes chosen at confirmation; requires `apikey:create-personal`; unavailable when the organization forbids personal keys, in which case CI-style service-account credentials are used).
- In CI, `orcon` detects `ORCON_FEDERATED_TOKEN_FILE` or the GitHub Actions/GitLab OIDC environment and performs the token exchange of ADR-0056 §9 when the context names a trust.

### 3. Command tree and coverage (amended by ADR-0080 §5)
- Generated: `orcon <resource-plural-kebab> <verb>` (at most three words, unique) for every stable/beta operation, from `operationId` and `x-orcon-cli`; experimental under `orcon x ...`.
- Ergonomic: `orcon flows pull|push|diff|validate|fmt|export|import`, `orcon executions watch|logs|retry|cancel`, `orcon attention list|continue|repeat|cancel` (human-attention operations, ADR-0052 §6), `orcon events tail`, `orcon git status|sync|resolve`, `orcon keys create|rotate|revoke`, `orcon sa create|list|disable`, `orcon confirmations list|reject` (approval is web-only, ADR-0059 §5), `orcon mcp serve --stdio`, `orcon api <METHOD> <path> [--data]`, `orcon completion`, `orcon version`. An ergonomic command is declared as the command of the operation it calls (e.g. the SSE stream declares `events tail`).
- Every operation declares `x-orcon-cli`: `{ command }`, or `{ none: { category, reason } }` with `category` ∈ `protocol` (OAuth and provider endpoints), `ingress` (provider callbacks), `interactive-session-only` (e.g. `confirmations/{confirmationId}/approve`, which only a person in an interactive session may call).
- CI test: every stable/beta `operationId` maps to a command or carries an exemption with an allowed category and a reason.

Generated commands `flows test-runs create|list|get`, `executions effect-previews list|get`, `flows pins list|create|get|replace|delete`, `executions flow get`, `n8n convert-fragment`, `node-types list`. Exemptions with category `interactive-session-only`: `continueEffectPreview` (sending a gated external effect needs a person) and the five editing-session operations (presence of a person in the editor).

### 4. Output and exit codes
`-o table|json|yaml` (table on a terminal, json otherwise); no colour off-terminal or with `NO_COLOR`. Exit codes: 0 success, 1 generic error, 2 usage, 3 not found, 4 conflict or precondition failed, 5 validation failed, 6 permission denied or unauthenticated, 7 rate limited, 8 timeout. Errors print problem `code`, `detail` and `requestId`. Mutating commands send `Idempotency-Key` automatically.

### 5. Flows pull/push/diff
- `pull`: downloads published versions (or drafts with `--drafts`) into the `orcon/repo@1` layout (ADR-0057 §2) and records `.orcon/state.json` (flow id, version, draft revision, content hash).
- `push`: uploads changed files as drafts with `If-Match` from the state file (`--expect-revision`), optionally `--publish`; runs the secret scanner first (ADR-0057 §8); refuses Git-leads protected projects.
- `diff`: semantic diff (nodes, edges, parameters, triggers) between local files and an instance or between two contexts; `--exit-code` returns 1 when different.

## Consequences
Everything scriptable through one tool; one download for CI; the CLI can do nothing the API does not allow; the operations without a command are named with a reason.

## Alternatives considered
Separate CLI binary/repository (duplicates validation and versioning); CLI talking to the database (bypasses authorization and audit); requiring a command for every operation (forces meaningless commands for protocol endpoints and web-only approvals).

## Amendments
Changes applied on 2026-10-12 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-014 (Amendment 5, carried by SPEC-005).** §3 gains generated commands and `interactive-session-only` exemptions. Additive only (ADR-0054 §3): no existing operation, field, permission, MCP level or tool changes; semantics of test runs, pinned data and effect confirmation: ADR-0082 (SPEC-014).
- **SPEC-015 (Amendment 2, 2026-10-13).** §1: the server runs only as `orcon serve`, with `--role`, `--http-address` and `--http-shutdown-timeout` generated from the configuration schema. The root invocation `orcon --role ...` and the `--addr` and `--shutdown-timeout` flags are removed without aliases, because nothing has been released. A fixed command dispatcher exits 2 for a missing or unknown command. The default listen address is `127.0.0.1:8080`.
