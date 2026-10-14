<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Channels: SDKs, CLI, MCP, permissions and audit

Status: Accepted

This document defines how every operation declares its exposure on the SDKs, the CLI and the MCP server, the permission it requires and the audit action it records. It amends ADR-0058 §3 and ADR-0059 §2 through [ADR 0080](../../../../docs/adr/0080-public-api-design-rules-and-contract-toolchain.md) §4 and §5. MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119. Rule IDs (`CHAN-…`) are stable.

Precedence: accepted ADRs, then accepted `orcon-go` rules, then the API ADRs (ADR-0052 to ADR-0059) as amended by ADR-0080, then this rulebook, then reviewer preference. The check that enforces each rule is named in brackets (an `orcon/<rule>` lint rule, a make target, or `review`).

## SDKs

- **CHAN-1** Every operation is in the three SDKs; no SDK-only or editor-only behavior (R-API-01/02). [review]

## CLI

- **CHAN-2** CLI command form: `<resource-plural-kebab> <verb>` (`flows publish`, `executions list`), at most three words, unique. [`cli-declared`]
- **CHAN-3** `x-orcon-cli: { command }` or `{ none: { category: protocol | ingress | interactive-session-only, reason } }`. [`cli-declared`]
- **CHAN-4** Ergonomic commands (`flows pull`, `events tail`, `attention continue`) are declared as the command of the operation they call. [`cli-declared`]

## MCP

- **CHAN-5** `x-orcon-mcp: { level, tool?, description? }`:
  - `none` — not exposed;
  - `read` — no state change; `GET`, or `validate`/`evaluate`;
  - `write` — changes Orcon state, no external effect and recoverable;
  - `destructive` — deletes, publishes, activates, deactivates, runs, retries or resolves human attention: anything with external effects or irreversible loss.

  `destructive` always goes through human confirmation per ADR-0059 §5; the key `confirm` no longer exists. Every `DELETE` exposed is `destructive`. Tool names are snake_case and unique. [`mcp-declared`]
- **CHAN-6** V1 classification. Exposing a new tool needs an accepted spec. [`mcp-declared`, review]

  | Level | Tools |
  |---|---|
  | `read` | list_projects, search_flows, get_flow, get_draft, validate_draft, list_node_types, get_node_type, list_credentials, list_executions, get_execution, get_execution_events, explain_execution_error, list_awaiting_human |
  | `write` | create_flow, update_draft, test_run_draft, cancel_execution |
  | `destructive` | run_flow, retry_execution, publish_flow, activate_flow, deactivate_flow, continue_awaiting_human, repeat_awaiting_human, cancel_awaiting_human |

## Permissions and audit

- **CHAN-7** `x-orcon-permission`: a permission from `internal/authz/permissions.yaml`, `{ allOf: [...] }`, `{ anyOf: [...] }`, `authenticated`, or `anonymous` (protocol operations, and the closed list `createSession`, `getSetupState`, `createSetupSession` of ADR-0080 §4); conditional extra permissions are explained in the description. [`permission`]
- **CHAN-8** `x-orcon-audit`: the audit action (`flow.published`) or `none` (reads); a mutation with `none` needs `x-orcon-audit-reason`. [`audit`]
