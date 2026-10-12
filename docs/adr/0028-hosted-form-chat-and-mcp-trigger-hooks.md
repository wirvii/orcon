<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0028 — Hosted form trigger and chat/MCP trigger hook points

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12

Accepted by the owner on 2026-10-12 via SPEC-002.

## Context
T5 includes a hosted form trigger and a chat trigger (designed under BL-006). n8n also offers an MCP server trigger. These are `WEBHOOK`-family triggers with richer HTTP interaction.

## Decision

### 1. Hosted form
- Served by the api role as server-rendered HTML that works without JavaScript; field types: text, email, password, number, date, dropdown, checkbox, radio, textarea, file, hidden, custom HTML (sanitized with an allowlist; scripts and styles removed); initial values from query parameters; custom CSS under a Content-Security-Policy; CSRF token; bot filter; authentication none, basic, or Orcon user (authorization hook until the IAM ADR). The public entry is resolved before its organization is known through `orcon_engine.resolve_public_entry` (ADR-0077 §6).
- Response modes: on submit (completion page), when the flow finishes, or through a respond node (ADR-0023).
- Multi-page forms: subsequent pages are durable waits (ADR-0010 §8) whose secret per-execution URL renders the next page and is resolved through `orcon_engine.resolve_wait_secret`; their mandatory timeout path applies. These waits are for the form respondent; any human-attention notification about them is governed by ADR-0073 (`execution.awaiting_human`, `execution.awaiting_human.escalated`, `execution.human_resolved`), where they are the `END_USER_WAIT` kind: off by default, enabled per node with `attention.notify: ON` (owner decision SPEC-002 N1). This ADR defines no notification of its own.
- Files are streamed to the blob store before the acceptance commit.
- Accessibility: WCAG 2.2 AA; labels and errors programmatically associated; keyboard operable; status not conveyed by colour alone; verified with automated checks and a manual keyboard/screen-reader pass.

### 2. Chat hook
A chat trigger is a `WEBHOOK`-family address with a required session id; each message is an ingress event deduplicated by message id, subject to the backpressure cap, answered through the response channel including `STREAM` mode. Public chat entries are resolved through `orcon_engine.resolve_public_entry`. Agent behaviour, memory, widget and history loading are specified by the AI design (BL-006).

### 3. MCP hook
MCP server triggers accept SSE and streamable HTTP. Session metadata (`session_id`, holding api process, expiry) is persisted in an organization-scoped table under the ADR-0077 rules; any api process may accept a client message as an ingress event; results reach the process holding the stream through the response channel, so no sticky routing is required. Tool exposure, protocol versions and authentication details belong to BL-006.

## Consequences
Forms, chat and MCP reuse ingress, deduplication, backpressure and response delivery; no separate subsystem.

## Alternatives considered
Client-side rendered form app (rejected: requires JavaScript, harder accessibility). Sticky routing for MCP (rejected, ADR-0023).

## Amendments
Changes applied on 2026-10-12 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-004.** A form restricted to "Orcon user" means a member of the organization holding `flow:submit-form` in the project; public forms and chats are resolved with `orcon_engine.resolve_public_entry`.
- **SPEC-005.** The administration MCP server (ADR-0059, `/api/v1/mcp`) is distinct from the MCP trigger (`/mcp*`, `/mcp-servers/*`).
