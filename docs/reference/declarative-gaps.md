<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# Declarative-format gaps

Status: **Accepted** | Source: ADR-0079 §9, rulebook `.agents/skills/orcon-connector/references/kinds.md` (KD-3 to KD-5)

This registry lists what the declarative HTTP connector format (ADR-0030) cannot express. An official connector is declarative by default; a native Go connector is accepted only when its `authoring.json` cites one of these gaps (`kind.gap: "DG-<n>"`) and names the concrete operation that needs it.

| Gap | What the declarative format cannot express | Typical connectors |
|---|---|---|
| DG-1 | A protocol other than HTTP | Databases, message queues, mail (SMTP, IMAP), file transfer (FTP, SFTP) |
| DG-2 | Request signing not covered by the `AWS_SIG_V4` scheme or the webhook verification presets | Services with a custom signature over headers, body or timestamps |
| DG-3 | Streaming responses or persistent connections (`listen` triggers) | Server-sent events, WebSocket feeds, long-lived subscriptions |
| DG-4 | Complex binary formats | Protocol Buffers, multipart/mixed responses, archives |
| DG-5 | Multi-step stateful protocols beyond the `SESSION_TOKEN` scheme | Login handshakes that keep state across several calls |
| DG-6 | Payload computation on the client | Content encryption, compression, hashing of content before sending |

## Threshold

When three official native connectors cite the same gap, a fourth one is accepted only with `kind.extensionBacklog` naming an open backlog item that extends the declarative format for that gap (KD-5). Extending the format is preferred to adding native code.

## Adding a gap

A new gap is proposed by pull request to this file. The pull request adds one row with the next free id, a description of what the declarative format cannot express and one real connector that needs it. Ids are never reused or renumbered.
