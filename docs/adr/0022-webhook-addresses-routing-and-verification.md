<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# ADR-0022 — Webhook addresses, routing, authentication, signature verification and n8n path compatibility

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12

Accepted by the owner on 2026-10-12 via SPEC-002.

## Context
T7: imported n8n flows keep their exact paths and response behaviour so migration means repointing a domain; new flows use Orcon-native addresses with optional unguessable suffix and signature verification; a trigger may have several active addresses (aliases); collisions are flagged at import and publish. n8n paths are shared by the whole instance, so two organizations can claim the same path; organization isolation (SPEC-004) forbids telling one organization anything about another.

## Decision

### 1. Address families
- `N8N` (compatibility): production `/webhook/<path>`, test `/webhook-test/<path>`; paths with route parameters use the webhook-id-prefixed form n8n displays (the `webhookId` is preserved from the export); forms `/form/<path>`, `/form-test/<path>`; MCP `/mcp/<path>`, `/mcp-test/<path>`; chat uses its webhook path. Response profile `N8N` (per address; owner decision SPEC-002 Q1).
- `NATIVE`: `/hooks/<project>/<path>` and `/hooks-test/<project>/<path>`; forms `/forms/<project>/<path>`; MCP `/mcp-servers/<project>/<path>`; optional random suffix (128-bit, base62) appended as a last segment. Response profile `NATIVE`.
- Scope: `PRODUCTION` or `TEST`.
- Base URLs: `ORCON_PUBLIC_URL` (editor and links) and optional `ORCON_INGRESS_URL`.

### 2. Aliases and lifecycle
Addresses live in the trigger block of the flow document (versioned, reviewed, collision-checked at publication; owner decision SPEC-002 Q7). Adding or removing an alias requires publishing. A trigger may have several active addresses, each with its own family and response profile. Removing an address is a published change; a removed address answers `410 Gone` for a grace period (default 30 days) and `404` afterwards.

### 3. Routing and collisions
- A route index keyed by `(scope, method, pattern)` and carrying the organization of each address (SPEC-004 R-ISO-08) is rebuilt from active published versions and open test sessions; api processes cache it and invalidate on `pg_notify('orcon_routes')`, with a 5 s poll fallback carrying a version counter.
- An incoming request is resolved before its organization is known through the engine-plane function `orcon_engine.resolve_trigger_address` (ADR-0077 §6), which returns the address, the trigger and its `org_id`; the acceptance transaction then runs in that organization's context and re-verifies the route against the database (ADR-0021 §1), so a stale cache can never start an execution of an inactive trigger.
- Identical `(method, pattern)` among active addresses, addresses being imported, or concurrent test sessions → blocking error. When the owner of the path belongs to the same organization, the error names the owning flow. When it belongs to another organization, the error is a generic problem ("path unavailable") with no detail at all — no flow, name, id or organization; the check runs in an engine-plane function that returns only "free" or "taken", so no other organization's row is ever read in the requester's context. Overlapping patterns (static vs parameter) → warning, with the same rule for details; precedence: more static segments first, then the leftmost static segment.

### 4. Authentication and request policy
Per address: none, basic, header, JWT (HS/RS/ES, issuer/audience checks); IP allowlist (CIDR); CORS allowed origins; bot filter; raw body; binary property; maximum size. Credentials come from the credential store.

### 5. Signature verification
Schemes: HMAC-SHA-256/512 over the raw body with configurable header, prefix (e.g. `sha256=`) and encoding (hex/base64); timestamped schemes with tolerance (default 5 min) and replay rejection within tolerance via the deduplication index; presets `GITHUB`, `STRIPE`, `SLACK`, `SHOPIFY`, `META`, `STANDARD_WEBHOOKS`. Constant-time comparison. Failure → 401, nothing persisted, `ingress.rejected{reason: SIGNATURE}`. A connector may declare verification mandatory.

### 6. Call-chain header
`Orcon-Origin` (ADR-0010 §7) is verified on every address; a valid header increments depth; beyond the limit → 508.

### 7. Inactive and unknown
Unknown route, inactive trigger or deleted flow: `N8N` profile → n8n's "not registered" 404 response (captured); `NATIVE` → 404 problem document.

## Consequences
Senders can be migrated gradually; native addresses are hard to guess and verifiable; all address changes are auditable; route resolution never reads organization data without an organization context; a path collision never discloses another organization's flows.

## Alternatives considered
Address table outside versions (faster changes, less auditable; rejected by the owner, SPEC-002 Q7). Only n8n paths (rejected: perpetuates n8n naming and offers no project namespace). Resolving routes with an ordinary query (rejected by ADR-0077: with row-level security and no organization context it returns no rows). Naming the owning flow in every collision error (rejected: it reveals another organization's data).

## Amendments
Changes applied on 2026-10-12 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-004.** §1: the `<project>` segment of native addresses is the instance-unique project identifier (the project slug; if it is already taken, `<org>-<project>`; SPEC-004 Q11). §3: a collision with an address of another organization returns a generic error without the flow name. §4: address authentication secrets come from the organization's credential store; an ingress host per organization is optional; resolving an incoming address before the organization is known is `orcon_engine.resolve_trigger_address` (ADR-0047 §2).
- **SPEC-005.** The "Standard Webhooks" preset is the signature format of Orcon's outbound webhooks (ADR-0055 §5).
