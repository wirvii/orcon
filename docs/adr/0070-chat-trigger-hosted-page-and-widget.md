<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0070 — Chat trigger: hosted page, embeddable widget, sessions

Status: **Accepted** | Date: 2026-10-11 (amended 2026-10-12: UPPER_SNAKE values per ADR-0080 §3)

Accepted by the owner on 2026-10-12 via SPEC-006.

## Context
BL-006 givens: chat trigger with hosted chat page and embeddable website widget, streaming, session continuity, n8n Chat Trigger import (ADR-0028 §2 hook).

## Decision
1. **Trigger** `orcon.chat@1`: address family `N8N` (imported path; documented n8n request shape `action: sendMessage|loadPreviousSession`, `sessionId`, `chatInput`, kept as n8n defines it because it is an ingress protocol, not an Orcon document) and `NATIVE` (`/chat/<project>/<path>`); modes `HOSTED` and `EMBEDDED`; authentication `NONE`, `BASIC`, `ORCON_USER`, or `SIGNED_SESSION_TOKEN` (HMAC JWT minted by the host site with the trigger secret, binding the conversation to a `userId`).
2. **Options** (parity with the public n8n Chat Trigger options): initial messages, title, subtitle, input placeholder, "new conversation" button, allowed origins (CORS), file uploads (to blobs, size/type caps), load previous session, response mode (last node, respond node, streaming), custom CSS under CSP, `showInChat` (listed in the editor's Chat page).
3. **Sessions**: the server issues a 128-bit session id and a signed session token on the first message; client-invented ids never open other conversations (n8n-compatible client ids are bound to the first authentication fingerprint seen). Messages deduplicated by message id (ADR-0021 §3), ordered per conversation (ADR-0064 §4).
4. **Hosted page**: server-rendered HTML plus minimal JavaScript for streaming; a no-JavaScript fallback (post and reload); WCAG 2.2 AA: `aria-live="polite"` announcing streamed answers by sentence, full keyboard operation, managed focus, status not by colour alone, verified contrast.
5. **Widget** `packages/js/chat-widget` (Apache-2.0, own implementation; `@n8n/chat` is not reused): Web Component `<orcon-chat>` loaded by one script tag, Shadow DOM, strict-CSP compatible, token-based theming, i18n (es, en), same protocol as the hosted page, ≤ 40 KiB gzip.
6. **Abuse controls**: per-IP and per-session limits before persistence, message size cap (default 16 KiB text), optional bot filter, ADR-0021 backpressure, AI budgets.
7. **Channels**: messaging channels are connector triggers/operations with `sessionKey` = channel chat id; `orcon.chat.send@1` sends an intermediate message and may wait durably for the user's reply.

## Consequences
One chat protocol for hosted, embedded and n8n-compatible clients; conversations cannot be hijacked by guessing ids.

## Alternatives considered
Reusing `@n8n/chat` (rejected: licence and independence); client-generated session ids only (rejected: guessable/hijackable); WebSocket transport (rejected for V1: SSE suffices, ADR-0055).
