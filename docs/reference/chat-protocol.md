<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Chat protocol

Status: **Accepted** | Source: ADR-0070 §1, §3 and §6, ADR-0063 §2–§3 (accepted 2026-10-12 via SPEC-006)

This page publishes the chat protocol shared by the hosted chat page, the embeddable widget and clients compatible with the n8n Chat Trigger. The ADRs are normative; if this page and an ADR disagree, the ADR wins. The chat endpoints are trigger ingress addresses, not part of `/api/v1`, and they are not Orcon documents; they keep the values shown here.

## Trigger
The trigger type is `orcon.chat@1`, a hook of ADR-0028 §2.

| Address family | Address | Request shape |
|---|---|---|
| `N8N` | The path of the imported Chat Trigger | The documented n8n shape: `action` (`sendMessage` or `loadPreviousSession`), `sessionId`, `chatInput`. It is kept as n8n defines it because it is an ingress protocol, not an Orcon document. |
| `NATIVE` | `/chat/<project>/<path>` | The same protocol. |

Modes are `HOSTED` and `EMBEDDED`.

## Authentication
| Value | Meaning |
|---|---|
| `NONE` | The input is public. |
| `BASIC` | The input is public. |
| `ORCON_USER` | A signed-in Orcon user. Only this value makes the user's input trusted. |
| `SIGNED_SESSION_TOKEN` | An HMAC JWT minted by the host website with the trigger secret, binding the conversation to a `userId` of the host site; the input is public. |

Public input is resolved before the organization is known, through `orcon_engine.resolve_public_entry` (ADR-0047 §2).

## Sessions
- The server issues a 128-bit session id and a signed session token on the first message.
- An id invented by a client never opens another conversation. Client ids of n8n-compatible clients are accepted but bound to the first authentication fingerprint seen.
- Messages are deduplicated by message id (ADR-0021 §3) and ordered per conversation (ADR-0064 §4).
- The widget keeps the session in `localStorage`.

## Streaming
- Response modes are last node, respond node and streaming (ADR-0063).
- Token deltas are not durable state. If a worker dies mid-stream, the api process sends `stream.reset {reason}`; the step is issued again and clients replace the partial text.
- Only final-answer text is streamed to end users. Tool-call deltas and reasoning are never streamed to them.
- Every delta passes through the masking scan and the reveal rule; a placeholder is never split across deltas.

## Limits
- Per-IP and per-session limits apply before anything is persisted.
- The message size cap is 16 KiB of text by default.
- An optional bot filter exists, together with the ingress backpressure of ADR-0021 and the AI budgets of ADR-0062.

## Hosted page and widget
- The hosted page is server-rendered HTML with minimal JavaScript for streaming and a no-JavaScript fallback (post and reload). It meets WCAG 2.2 AA: an `aria-live="polite"` region announcing streamed answers by sentence, full keyboard operation, managed focus, status not shown by color alone and verified contrast.
- The widget is the Web Component `<orcon-chat>` of `packages/js/chat-widget`, loaded with one script tag. It uses Shadow DOM, works under a strict content security policy, supports token-based theming and the languages Spanish and English, speaks the same protocol as the hosted page, and is at most 40 KiB gzipped. It is an own implementation and does not reuse the n8n chat widget.
