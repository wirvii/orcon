<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# ADR-0067 — MCP client and MCP server trigger

Status: **Accepted** | Date: 2026-10-11 (amended 2026-10-12: table names per ADR-0077; pre-organization lookup per ADR-0047 §2; UPPER_SNAKE values per ADR-0080 §3) (amended 2026-10-13: stored enumerated values are UPPER_SNAKE, ADR-0015 §2)

Accepted by the owner on 2026-10-12 via SPEC-006.

## Context
BL-006 givens: MCP client nodes and MCP server trigger per SPEC-002 (ADR-0028 §3 hook). ADR-0059 defines a different object: Orcon's administration MCP server.

## Decision

### 1. MCP client
- Configuration `mcp_server(org_id, id, project_id, name, transport: STREAMABLE_HTTP|SSE|STDIO, url?, stdio_command?, authentication_kind: NONE|BEARER|HEADER|OAUTH2, secret_reference_document, allowed_tool_document?, created_by_principal_id?)` (documents with their `_format` columns, read whole); secrets write-only (ADR-0043); OAuth 2.1 with single-holder refresh (ADR-0045).
- Network through the egress client with approved domains. `STDIO` is disabled by default; an Instance Admin may enable it with allowed commands and a host tag; the process runs as an isolated subprocess without inherited environment or credentials (same treatment as host-touching nodes in SPEC-003).
- Protocol revision negotiated and recorded; tool/resource/prompt lists pinned at publication (ADR-0066 §6).
- Tools without a trusted read-only override are `UNSAFE` (annotations such as `readOnlyHint`, `idempotentHint` are hints only); an editor may declare a tool `READ` (recorded as a declared contract, proof level `DECLARED`).
- Results are `EXTERNAL` content, size-capped.
- `orcon.mcp.client@1` calls MCP tools directly from a flow with the same rules.

### 2. MCP server trigger `orcon.mcpServer@1`
- Exposes the tools connected to the trigger in the published version; each `tools/call` is an ingress event (ADR-0021) deduplicated by `(sessionId, JSON-RPC id)`; sessions persisted without sticky routing (ADR-0028 §3); results via the response channel (ADR-0023); `notifications/tools/list_changed` on publication.
- Authentication: none, bearer, header (n8n parity) or Orcon API keys (ADR-0056) requiring `flow:execute` on the project.
- Addresses: n8n family `/mcp/<path>` and native `/mcp-servers/<project>/<path>` (ADR-0022), resolved before the organization is known by `orcon_engine.resolve_trigger_address` (ADR-0047 §2).

### 3. Library
The official MCP Go SDK if its licence is permissive (confirmed in the slice); otherwise an in-repo implementation of the subset used (shared with ADR-0059).

### 4. Tests
Conformance against reference MCP servers/clients; drift; auth; SSRF; duplicate `tools/call` delivery; stdio isolation.

### 5. Logical schema additions
`mcp_server`, `mcp_tool_snapshot(org_id, id, flow_version_id, mcp_server_id, content_hash, document, document_format)`, `mcp_trigger_session(org_id, id, …)` (columns completed by the implementing slice under the database rulebook).

## Consequences
Orcon consumes and exposes MCP with the same safety and durability as other tools and triggers.

## Alternatives considered
Treating MCP results as trusted (rejected); sticky MCP sessions (rejected, ADR-0023).

## Amendments
Changes applied after acceptance; each entry names its source.

- **SPEC-009 K19 (2026-10-13, owner decision).** §1 (`mcp_server.transport`, `mcp_server.authentication_kind`): enumerated columns store the same UPPER_SNAKE values the API exposes (ADR-0015 §2 applies to every table); no adapter translates an enumerated value between the database and the API.
