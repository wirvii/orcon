<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Connector kinds

Status: Proposed — pending owner approval

This file decides which kind a connector is and where it lives. Rule IDs (`KD-…`) are stable and are cited by the linter, by reviews and by catalog rejections. `[official]` marks rules that apply only to `orcon.*` connectors; every other rule also applies to catalog submissions.

## Precedence

When two sources disagree, the higher one wins:

1. Accepted ADRs.
2. Accepted `orcon-go` rules (`../../orcon-go/references/architecture-layers.md` and the rest of that rulebook).
3. The connector ADRs 0029 to 0037 and ADR-0079 (design input while Proposed).
4. This rulebook (`references/*.md`).
5. Reviewer preference.

`orcon-ui` governs everything visual. A conflict between two sources of the same level is not resolved silently: the change stops and the owner decides.

## Rules

- **KD-1** Three kinds (ADR-0029 §1): `DECLARATIVE` for HTTP APIs, `NATIVE` for official connectors only, `WASM` for third-party code. Third parties choose declarative whenever possible.
- **KD-2** [official] Official connectors are declarative by default. Choose native only when the declarative format cannot express the connector.
- **KD-3** [official] A native connector records in `authoring.json` `kind { chosen: "NATIVE", gap: "DG-<n>", explanation }`. The gap id exists in `docs/reference/declarative-gaps.md`; the explanation names the concrete operation that needs it.
- **KD-4** Initial gaps: DG-1 non-HTTP protocol (databases, queues, mail, file transfer); DG-2 request signing not covered by `AWS_SIG_V4` or the webhook presets; DG-3 streaming responses or persistent connections (`listen`); DG-4 complex binary formats (Protocol Buffers, multipart/mixed, archives); DG-5 multi-step stateful protocols beyond `SESSION_TOKEN`; DG-6 client-side payload computation (encryption, compression, hashing of content). New gaps are added by pull request to the registry with an example.
- **KD-5** [official] When three official native connectors cite the same gap, a fourth one needs `kind.extensionBacklog` naming an open backlog item to extend the declarative format for that gap. Extending the format is preferred to adding native code.
- **KD-6** A connector is one service. Several products of one vendor with separate APIs and credentials are separate connectors.
- **KD-7** [official] Location and registration: `internal/adapter/connectors/<name>/` (ADR-0079 §1); registered in `cmd/orcon`; native code follows `../../orcon-go/references/architecture-layers.md`. Connectors for AI model providers live under the approved AI paths (`internal/adapter/llm/**`).
